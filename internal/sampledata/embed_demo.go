//go:build demo

package sampledata

import (
	"embed"
	"io/fs"
)

const Embedded = true

//go:embed testdata/research_policy.json
var demoJSON []byte

//go:embed scenarios
var scenarioFS embed.FS

func payload() []byte { return demoJSON }

func scenarioFiles() fs.FS { return scenarioFS }
