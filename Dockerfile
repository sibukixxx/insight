# Insight Lab standalone image (#138): one Go binary with the embedded
# Reference Web, SQLite and a data volume. No database server, broker or
# cloud account is needed (docs/adr/0001-standalone-first-portable-deployment.md).
# The committed internal/web/dist is embedded, so the build needs no Node.

FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# VERSION is recorded in every analysis run; without it the engine reports
# UNKNOWN. The image is built without .git, so the commit is UNKNOWN too.
ARG VERSION=""
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false \
      -ldflags "-s -w ${VERSION:+-X insight-lab/internal/buildinfo.Version=${VERSION}}" \
      -o /out/insight-lab ./cmd/insight-lab

FROM alpine:3.22
RUN addgroup -S -g 65532 insight && adduser -S -D -H -u 65532 -G insight insight \
    && mkdir -p /data && chown insight:insight /data
COPY --from=build /out/insight-lab /usr/local/bin/insight-lab
USER insight:insight
VOLUME ["/data"]
EXPOSE 8787
# The listener binds all interfaces inside the container; publish it to the
# host's loopback only (compose.yaml does) unless an authenticating proxy
# fronts it: Insight has no built-in authentication.
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=3 \
  CMD ["insight-lab", "health", "-url", "http://127.0.0.1:8787/api/health"]
ENTRYPOINT ["insight-lab"]
CMD ["serve", "-host", "0.0.0.0", "-port", "8787", "-db", "/data/insight.db", "-no-browser"]
