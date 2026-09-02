BINARY  := insight-lab
PKG     := ./cmd/insight-lab
BINDIR  := bin

.PHONY: build build-demo build-delivery test vet clean cross-compile cross-compile-demo cross-compile-delivery eval-demo

build: build-delivery

# 顧客への納品用ビルド。デモデータは一切コンパイルされない（internal/sampledata/embed_delivery.go）。
build-delivery:
	go build -o $(BINDIR)/$(BINARY) $(PKG)

# 商談デモ用ビルド。サンプルインタビューデータを埋め込む（internal/sampledata/embed_demo.go）。
build-demo:
	go build -tags demo -o $(BINDIR)/$(BINARY)-demo $(PKG)

test:
	go test ./...
	go test -tags demo ./...

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
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -o $(BINDIR)/$(BINARY)-darwin-arm64        $(PKG)
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -o $(BINDIR)/$(BINARY)-darwin-amd64        $(PKG)
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -o $(BINDIR)/$(BINARY)-linux-amd64         $(PKG)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o $(BINDIR)/$(BINARY)-windows-amd64.exe   $(PKG)

cross-compile-demo:
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -tags demo -o $(BINDIR)/$(BINARY)-demo-darwin-arm64      $(PKG)
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -tags demo -o $(BINDIR)/$(BINARY)-demo-darwin-amd64      $(PKG)
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -tags demo -o $(BINDIR)/$(BINARY)-demo-linux-amd64       $(PKG)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -tags demo -o $(BINDIR)/$(BINARY)-demo-windows-amd64.exe $(PKG)
