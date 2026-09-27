package app

import (
	"path/filepath"
	"testing"

	"insight-lab/internal/execution"
)

func TestParseConfigRuntimeIsLocalByDefaultAndProcessNeedsHeavyDirAndInputRoot(t *testing.T) {
	db := filepath.Join(t.TempDir(), "i.db")
	cfg, err := ParseConfig([]string{"-db", db})
	if err != nil || cfg.Runtime != execution.RuntimeLocal {
		t.Fatalf("default runtime = %q, %v", cfg.Runtime, err)
	}
	if _, err := ParseConfig([]string{"-db", db, "-runtime", "process", "-heavy-dir", t.TempDir()}); err == nil {
		t.Fatal("process runtime without -input-root was accepted")
	}
	if _, err := ParseConfig([]string{"-db", db, "-runtime", "remote"}); err == nil {
		t.Fatal("an unknown runtime was accepted")
	}
	cfg, err = ParseConfig([]string{"-db", db, "-runtime", "process", "-heavy-dir", t.TempDir(), "-input-root", t.TempDir()})
	if err != nil || cfg.Runtime != execution.RuntimeProcess {
		t.Fatalf("process runtime = %q, %v", cfg.Runtime, err)
	}
}

func TestParseConfigReadsModelSettingsFromEnvironmentUnlessFlagsOverride(t *testing.T) {
	t.Setenv("INSIGHT_LAB_MODEL", "env-model")
	t.Setenv("INSIGHT_LAB_BASE_URL", "http://env.invalid/v1")
	db := filepath.Join(t.TempDir(), "i.db")
	cfg, err := ParseConfig([]string{"-db", db})
	if err != nil || cfg.Model != "env-model" || cfg.BaseURL != "http://env.invalid/v1" {
		t.Fatalf("from env = %q %q, %v", cfg.Model, cfg.BaseURL, err)
	}
	cfg, err = ParseConfig([]string{"-db", db, "-model", "flag-model"})
	if err != nil || cfg.Model != "flag-model" {
		t.Fatalf("flag must win: %q, %v", cfg.Model, err)
	}
}
