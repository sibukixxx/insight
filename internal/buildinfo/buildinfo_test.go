package buildinfo

import (
	"runtime/debug"
	"testing"
)

func TestResolveUsesInjectedVersionAndVCSSettings(t *testing.T) {
	info := &debug.BuildInfo{Settings: []debug.BuildSetting{
		{Key: "vcs.revision", Value: "3656fa12b9d8"}, {Key: "vcs.modified", Value: "true"},
	}}
	got := resolve("v0.9.0", info, true)
	want := Info{Version: "v0.9.0", Commit: "3656fa12b9d8", Dirty: "true"}
	if got != want {
		t.Fatalf("resolve = %+v, want %+v", got, want)
	}
}

func TestResolveStopsAtUnknownWithoutGuessing(t *testing.T) {
	// A binary built outside a git checkout (e.g. go install from a proxy)
	// carries a module version but no vcs settings; neither is a substitute
	// for an injected release version or a commit.
	info := &debug.BuildInfo{Main: debug.Module{Version: "v0.0.0-20260924050106-3656fa12b9d8"}}
	got := resolve("", info, true)
	want := Info{Version: Unknown, Commit: Unknown, Dirty: Unknown}
	if got != want {
		t.Fatalf("resolve = %+v, want %+v", got, want)
	}
	if got := resolve("", nil, false); got != want {
		t.Fatalf("resolve without build info = %+v, want %+v", got, want)
	}
}
