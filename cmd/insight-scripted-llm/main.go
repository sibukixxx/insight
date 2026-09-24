// Command insight-scripted-llm serves a deterministic, grounded,
// OpenAI-compatible /chat/completions endpoint for conformance testing.
// Start insight-lab with -base-url pointing here to run the model-backed
// Public Engine Contract fixtures (e.g. from the standalone SDK
// repositories) without any real LLM call. Never use it as a model.
package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"insight-lab/internal/llm/scripted"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8788", "listen address")
	flag.Parse()
	log.Printf("insight-scripted-llm (TEST TOOL, not a model) listening on %s", *addr)
	srv := &http.Server{Addr: *addr, Handler: scripted.Handler(), ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
}
