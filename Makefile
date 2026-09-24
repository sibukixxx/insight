BINARY  := insight-lab
PKG     := ./cmd/insight-lab
BINDIR  := bin

# Release version recorded in every analysis run's execution snapshot. It
# comes from a git tag (override with `make build VERSION=v0.9.0`). Without a
# tag it stays empty and the engine reports UNKNOWN; the commit and dirty
# state are still read from the Go build's VCS metadata.
VERSION ?= $(shell git describe --tags --dirty 2>/dev/null)
LDFLAGS := $(if $(VERSION),-X insight-lab/internal/buildinfo.Version=$(VERSION))

.PHONY: build build-demo build-delivery test test-sdk test-golden vet clean cross-compile cross-compile-demo cross-compile-delivery eval-demo

build: build-delivery

# 顧客への納品用ビルド。デモデータは一切コンパイルされない（internal/sampledata/embed_delivery.go）。
build-delivery:
	go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/$(BINARY) $(PKG)

# 商談デモ用ビルド。サンプルインタビューデータを埋め込む（internal/sampledata/embed_demo.go）。
build-demo:
	go build -ldflags "$(LDFLAGS)" -tags demo -o $(BINDIR)/$(BINARY)-demo $(PKG)

test:
	go test ./...
	go test -tags demo ./...

# Go and TypeScript SDKs. The live conformance run against real engines is
# part of `make test` (internal/http/public_conformance_test.go).
test-sdk:
	cd sdk/go && go vet ./... && go test ./...
	cd sdk/node && node scripts/generate-types.mjs --check && node --test "test/*.test.ts"
	@if [ -d sdk/node/node_modules ]; then cd sdk/node && npx tsc --noEmit; else echo "skipping TypeScript typecheck: run npm install in sdk/node"; fi

test-golden:
	python3 testdata/golden/harness/test_golden_eval.py
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
