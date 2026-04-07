package main

import (
	"context"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

var tmpl = template.Must(template.New("main").Parse(`<!DOCTYPE html>
<html>
	<head>
		<meta http-equiv="Content-Type" content="text/html; charset=utf-8"/>
		<meta name="go-import" content="{{.Domain}}/{{.PkgRoot}} git https://{{.VCS}}/{{.RepoRoot}}">
		<meta http-equiv="refresh" content="0; url=https://godoc.org/{{.Domain}}{{.ImportPath}}">
	</head>
	<body>
		Redirecting to docs at <a href="https://godoc.org/{{.Domain}}{{.ImportPath}}">godoc.org/{{.Domain}}{{.ImportPath}}</a>...
	</body>
</html>
`))

var (
	domain    string
	vcs       string
	httpAddr  string
	overrides string
)

// parseOverrides parses a comma-separated list of repo=vanity mappings.
// e.g. "sia-storage-go=sia-storage,old-name=new-name"
// The key is the repo name in the VCS, the value is the vanity package name.
func parseOverrides(s string) (repoToVanity map[string]string, vanityToRepo map[string]string) {
	repoToVanity = make(map[string]string)
	vanityToRepo = make(map[string]string)
	if s == "" {
		return
	}
	for _, entry := range strings.Split(s, ",") {
		parts := strings.SplitN(strings.TrimSpace(entry), "=", 2)
		if len(parts) != 2 {
			continue
		}
		repo, vanity := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		repoToVanity[repo] = vanity
		vanityToRepo[vanity] = repo
	}
	return
}

func run(ctx context.Context, domain, vcs, httpAddr string, repoToVanity, vanityToRepo map[string]string) error {
	l, err := net.Listen("tcp", httpAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", httpAddr, err)
	}
	defer l.Close()

	s := &http.Server{
		BaseContext: func(_ net.Listener) context.Context { return ctx },
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Method != http.MethodGet {
				http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
				return
			}
			pkgRoot := strings.Split(req.URL.Path, "/")[1]
			repoRoot := pkgRoot
			// if the request came in with a repo name that has a vanity override,
			// reject it — the vanity name is canonical
			if _, ok := repoToVanity[pkgRoot]; ok {
				http.NotFound(w, req)
				return
			}
			// check if the request path matches a vanity name that maps to a different repo
			if repo, ok := vanityToRepo[pkgRoot]; ok {
				repoRoot = repo
			}
			w.Header().Set("Cache-Control", "public, max-age=300")
			tmpl.Execute(w, struct {
				Domain     string
				VCS        string
				PkgRoot    string
				RepoRoot   string
				ImportPath string
			}{
				Domain:     domain,
				VCS:        vcs,
				PkgRoot:    pkgRoot,
				RepoRoot:   repoRoot,
				ImportPath: req.URL.Path,
			})
		}),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	defer func() {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	}()
	go s.Serve(l)

	log.Println("server started", l.Addr())
	<-ctx.Done()
	log.Println("shutting down...")
	return nil
}

func main() {
	flag.StringVar(&domain, "domain", os.Getenv("VANITY_DOMAIN"), "vanity domain, e.g. foo.com")
	flag.StringVar(&vcs, "vcs", os.Getenv("VANITY_VCS"), "vcs URL, e.g. github.com/foo")
	flag.StringVar(&httpAddr, "addr", ":8080", "host:port to listen on")
	flag.StringVar(&overrides, "overrides", os.Getenv("VANITY_OVERRIDES"), "comma-separated repo=vanity mappings, e.g. sia-storage-go=sia-storage")
	flag.Parse()

	switch {
	case domain == "":
		log.Fatal("Missing required flag: -domain")
	case vcs == "":
		log.Fatal("Missing required flag: -vcs")
	case httpAddr == "":
		log.Fatal("Missing required flag: -addr")
	}

	repoToVanity, vanityToRepo := parseOverrides(overrides)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := run(ctx, domain, vcs, httpAddr, repoToVanity, vanityToRepo); err != nil {
		log.Fatal(err)
	}
}
