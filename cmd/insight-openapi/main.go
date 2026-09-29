// Command insight-openapi regenerates docs/openapi/public-engine-v1.json
// from contracts/public-engine/v1/schema.json (a developer tool; it is not
// part of the insight-lab binary). With -check it only reports whether the
// committed file is current and exits 1 when it is stale.
//
//	go run ./cmd/insight-openapi          # write
//	go run ./cmd/insight-openapi -check   # verify
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"insight-lab/internal/openapigen"
)

func main() {
	root := flag.String("root", ".", "repository root")
	check := flag.Bool("check", false, "fail when the committed spec differs from the generated one instead of writing it")
	flag.Parse()

	schema, err := os.ReadFile(filepath.Join(*root, openapigen.SchemaPath))
	if err != nil {
		fail(err)
	}
	spec, err := openapigen.GeneratePublic(schema)
	if err != nil {
		fail(err)
	}
	target := filepath.Join(*root, openapigen.PublicSpecPath)
	if *check {
		current, err := os.ReadFile(target)
		if err != nil {
			fail(err)
		}
		if !bytes.Equal(current, spec) {
			fail(fmt.Errorf("%s is stale: run make openapi and commit it", openapigen.PublicSpecPath))
		}
		fmt.Printf("%s is up to date\n", openapigen.PublicSpecPath)
		return
	}
	if err := os.WriteFile(target, spec, 0o644); err != nil {
		fail(err)
	}
	fmt.Printf("wrote %s\n", openapigen.PublicSpecPath)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "insight-openapi:", err)
	os.Exit(1)
}
