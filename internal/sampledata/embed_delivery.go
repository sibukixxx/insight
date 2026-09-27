//go:build !demo

package sampledata

import "io/fs"

const Embedded = false

func payload() []byte { return nil }

func scenarioFiles() fs.FS { return nil }
