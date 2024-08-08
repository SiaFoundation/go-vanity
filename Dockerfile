FROM docker.io/library/golang:1.22 AS builder

WORKDIR /vanity

# copy source
COPY . .

# build
RUN go build -o bin/ -tags='netgo timetzdata' -trimpath -a -ldflags '-s -w'  ./cmd/vanity

FROM scratch

LABEL maintainer="The Sia Foundation <info@sia.tech>" \
      org.opencontainers.image.description.vendor="The Sia Foundation" \
      org.opencontainers.image.description="A Go package vanity URL server" \
      org.opencontainers.image.source="https://github.com/SiaFoundation/go-vanity" \
      org.opencontainers.image.licenses=MIT

ENV PUID=0
ENV PGID=0

ENV VANITY_DOMAIN=
ENV VANITY_VCS=

# copy binary and prepare data dir.
COPY --from=builder /vanity/bin/* /usr/bin/
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
VOLUME [ "/data" ]

EXPOSE 8080/tcp

USER ${PUID}:${PGID}

ENTRYPOINT [ "vanity" ]