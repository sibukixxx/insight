//go:build !demo

package sampledata

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestScenariosIsEmptyWhenDeliveryBuild(t *testing.T) {
	scenarios, err := Scenarios()
	if err != nil || len(scenarios) != 0 {
		t.Fatalf("Scenarios() = %v, %v; want none", scenarios, err)
	}
	if _, err := ScenarioInput("ja-shop-records"); !errors.Is(err, ErrUnknownScenario) {
		t.Errorf("ScenarioInput err = %v, want ErrUnknownScenario", err)
	}
}

// The build tag is the only thing keeping sample data out of a customer
// binary, so check the linked result, not just the Go constant.
func TestDeliveryBinaryContainsNoSampleDataWhenBuiltWithoutDemoTag(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the delivery binary")
	}
	bin := filepath.Join(t.TempDir(), "insight-lab")
	cmd := exec.Command("go", "build", "-o", bin, "insight-lab/cmd/insight-lab")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	data, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"policy-data-treated-2024", "架空店舗A・2024年12月の月次記録", "pop-08212-2015", "memo-interview-01"} {
		if bytes.Contains(data, []byte(marker)) {
			t.Errorf("delivery binary contains sample marker %q", marker)
		}
	}
}
