package uxexplore

import (
	"context"
	"encoding/json"
	"testing"
)

func TestDriver_ReachesInitialStatePerFixture(t *testing.T) {
	for _, fixture := range EnumerateFixtures() {
		t.Run(fixture.Name, func(t *testing.T) {
			driver, err := NewDriver(fixture)
			if err != nil {
				t.Fatalf("NewDriver: %v", err)
			}
			trace, err := driver.Run(context.Background())
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if len(trace.Visited) != 1 {
				t.Fatalf("expected one initial visit, got %d", len(trace.Visited))
			}
			if trace.Visited[0].Fingerprint.Screen == "" {
				t.Fatalf("empty screen fingerprint: %#v", trace.Visited[0].Fingerprint)
			}
		})
	}
}

func TestDriver_Determinism_TwoRunsByteIdentical(t *testing.T) {
	fixture := EnumerateFixtures()[0]
	first, err := runTraceJSON(fixture)
	if err != nil {
		t.Fatal(err)
	}
	second, err := runTraceJSON(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("driver is not deterministic\nfirst:  %s\nsecond: %s", first, second)
	}
}

func TestFingerprint_StableAcrossEquivalentStates(t *testing.T) {
	driver, err := NewDriver(FixtureSpec{Name: "stable", Credentials: CredentialsValid, Provider: ProviderRequiresCreds, Targets: TargetsOne})
	if err != nil {
		t.Fatalf("NewDriver: %v", err)
	}
	first, err := driver.Run(context.Background())
	if err != nil {
		t.Fatalf("Run first: %v", err)
	}
	second, err := driver.Run(context.Background())
	if err != nil {
		t.Fatalf("Run second: %v", err)
	}
	if first.Visited[0].Fingerprint != second.Visited[0].Fingerprint {
		t.Fatalf("fingerprint mismatch\nfirst:  %#v\nsecond: %#v", first.Visited[0].Fingerprint, second.Visited[0].Fingerprint)
	}
}

func runTraceJSON(fixture FixtureSpec) ([]byte, error) {
	driver, err := NewDriver(fixture)
	if err != nil {
		return nil, err
	}
	trace, err := driver.Run(context.Background())
	if err != nil {
		return nil, err
	}
	return json.Marshal(trace)
}
