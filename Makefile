BINARY  := insight-lab
PKG     := ./cmd/insight-lab
BINDIR  := bin

# Release version recorded in every analysis run's execution snapshot. It
# comes from a git tag (override with `make build VERSION=v0.9.0`). Without a
# tag it stays empty and the engine reports UNKNOWN; the commit and dirty
# state are still read from the Go build's VCS metadata.
VERSION ?= $(shell git describe --tags --dirty 2>/dev/null)
LDFLAGS := $(if $(VERSION),-X insight-lab/internal/buildinfo.Version=$(VERSION))

.PHONY: build build-demo build-delivery build-all test test-golden vet clean cross-compile cross-compile-demo cross-compile-delivery eval-demo web-install web-build web-check web-test web-e2e docker-build docker-smoke

build: build-delivery

# 顧客への納品用ビルド。デモデータは一切コンパイルされない（internal/sampledata/embed_delivery.go）。
build-delivery:
	go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/$(BINARY) $(PKG)

# 商談デモ用ビルド。サンプルインタビューデータを埋め込む（internal/sampledata/embed_demo.go）。
build-demo:
	go build -ldflags "$(LDFLAGS)" -tags demo -o $(BINDIR)/$(BINARY)-demo $(PKG)

# Frontend (web/ → internal/web/dist). The committed dist/ keeps `make build`
# Node-free; these targets need Node 22+ and pnpm.
WEB_DIST := internal/web/dist

web-install:
	pnpm --dir web install --frozen-lockfile

web-build: web-install
	pnpm --dir web build

# Frontend + Go single binary.
build-all: web-build build

web-test: web-install
	pnpm --dir web typecheck
	pnpm --dir web lint
	pnpm --dir web test

# Fails when the committed dist/ is not the build output of web/ (stale or
# hand-edited assets, or files the build no longer produces).
web-check: web-test web-build
	@git diff --exit-code -- $(WEB_DIST) || (echo "internal/web/dist is stale: run make web-build and commit it" >&2; exit 1)
	@test -z "$$(git ls-files --others --exclude-standard -- $(WEB_DIST))" || (echo "untracked files in internal/web/dist:" >&2; git ls-files --others --exclude-standard -- $(WEB_DIST) >&2; exit 1)

# Browser E2E against real Go binaries (delivery and demo builds) with the
# scripted local model; no paid LLM. Needs `pnpm --dir web exec playwright install chromium` once.
web-e2e: web-build
	pnpm --dir web test:e2e

# Container packaging (#138): Docker is optional; the binary needs none of this.
docker-build:
	docker build --build-arg VERSION=$(VERSION) -t insight-lab:local .

# Default Compose smoke: healthy, Insight only, data survives recreation.
docker-smoke:
	sh scripts/docker-smoke.sh

test:
	go test ./...
	go test -tags demo ./...

test-golden:
	python3 testdata/golden/harness/test_golden_eval.py
	python3 testdata/golden/harness/test_discovery_benchmark.py
	python3 testdata/golden/harness/discovery_benchmark.py > /dev/null
	go test -tags=golden ./...

vet:
	go vet ./...
	go vet -tags demo ./...

clean:
	rm -rf $(BINDIR)

# 実LLMでデモデータを解析し、評価指標・Insight・痕跡を docs/evaluation/ に保存する。
# INSIGHT_LAB_API_KEY と INSIGHT_LAB_MODEL（任意で INSIGHT_LAB_BASE_URL）が必要。
eval-demo:
	./scripts/eval-demo.sh

cross-compile: cross-compile-demo cross-compile-delivery

cross-compile-delivery:
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/$(BINARY)-darwin-arm64        $(PKG)
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/$(BINARY)-darwin-amd64        $(PKG)
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/$(BINARY)-linux-amd64         $(PKG)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/$(BINARY)-windows-amd64.exe   $(PKG)

cross-compile-demo:
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -tags demo -o $(BINDIR)/$(BINARY)-demo-darwin-arm64      $(PKG)
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -tags demo -o $(BINDIR)/$(BINARY)-demo-darwin-amd64      $(PKG)
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -tags demo -o $(BINDIR)/$(BINARY)-demo-linux-amd64       $(PKG)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -tags demo -o $(BINDIR)/$(BINARY)-demo-windows-amd64.exe $(PKG)
