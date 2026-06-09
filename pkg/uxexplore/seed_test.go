package uxexplore

import (
	"context"
	"testing"
)

// UXE-08: a seeded run extends the recorded path rather than restarting from
// scratch. We assert the trace is tagged "seeded" and that it visits more
// states than the seed itself (the probe ran from the post-seed state).
func TestRunWithSeed_ExtendsRecordedPath(t *testing.T) {
	d, err := NewDriver(FixtureSpec{Name: "happy", Credentials: CredentialsValid, Provider: ProviderRequiresCreds, Conflicts: ConflictsNone, Targets: TargetsOne})
	if err != nil {
		t.Fatalf("NewDriver: %v", err)
	}
	trace, err := d.RunWithSeed(context.Background(), []SeedKey{"p", "enter"})
	if err != nil {
		t.Fatalf("RunWithSeed: %v", err)
	}
	if trace.Origin != "seeded" {
		t.Errorf("expected Origin=seeded, got %q", trace.Origin)
	}
	if len(trace.Visited) < 2 {
		t.Fatalf("expected >= 2 reachable states after seed, got %d", len(trace.Visited))
	}
}
