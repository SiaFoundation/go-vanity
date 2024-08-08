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
	domain   string
	vcs      string
	httpAddr string
)

func run(ctx context.Context, domain, vcs, httpAddr string) error {
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
			root := strings.Split(req.URL.Path, "/")[1]
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
				PkgRoot:    root,
				RepoRoot:   root,
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
	flag.Parse()

	switch {
	case domain == "":
		log.Fatal("Missing required flag: -domain")
	case vcs == "":
		log.Fatal("Missing required flag: -vcs")
	case httpAddr == "":
		log.Fatal("Missing required flag: -addr")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := run(ctx, domain, vcs, httpAddr); err != nil {
		log.Fatal(err)
	}
}
