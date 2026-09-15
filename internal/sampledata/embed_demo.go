//go:build demo

package sampledata

import _ "embed"

const Embedded = true

//go:embed testdata/research_policy.json
var demoJSON []byte

func payload() []byte { return demoJSON }
