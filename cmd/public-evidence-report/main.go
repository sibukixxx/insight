package main

import (
	"flag"
	"fmt"
	"os"

	"insight-lab/internal/publicreport"
)

func main() {
	input := flag.String("input", "data/public-evidence/company-count/source_extract.csv", "source extract CSV")
	metadata := flag.String("metadata", "data/public-evidence/company-count/source_metadata.json", "source metadata JSON")
	outputDir := flag.String("output-dir", "data/public-evidence/company-count/generated", "output directory")
	flag.Parse()

	if err := publicreport.Generate(*input, *metadata, *outputDir); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
	fmt.Printf("generated public evidence report artifacts in %s\n", *outputDir)
}
