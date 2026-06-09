package uxexplore

import (
	"context"
	"strings"
)

// SeedKey is a single keystroke to replay against a fresh dashboard model
// before the probe starts exploring. The string follows the same key-label
// convention as the recorder (`enter`, `esc`, `up`, `down`, single runes).
type SeedKey string

// RunWithSeed drives the dashboard through seed keys, then hands off to the
// probe from the post-seed state. The returned trace is tagged with Origin
// "seeded" so downstream tools can distinguish recorded-then-extended runs
// from purely synthetic runs.
func (d *Driver) RunWithSeed(ctx context.Context, seed []SeedKey) (*Trace, error) {
	m, err := d.StartModel(ctx)
	if err != nil {
		return nil, err
	}
	if shouldPrimeSeedDoctorMode(m.Snapshot().Screen, seed) {
		next, ok := drive(m, "d")
		if !ok {
			trace := &Trace{Fixture: d.spec, Origin: "seeded"}
			NewProbe().Visit(m, trace)
			return trace, nil
		}
		m = next
	}
	for _, key := range seed {
		next, ok := drive(m, string(key))
		if !ok {
			break
		}
		m = next
	}
	trace := &Trace{Fixture: d.spec, Origin: "seeded"}
	NewProbe().Visit(m, trace)
	return trace, nil
}

func shouldPrimeSeedDoctorMode(screen string, seed []SeedKey) bool {
	if screen != "Welcome" {
		return false
	}
	for _, key := range seed {
		label := strings.ToLower(string(key))
		switch label {
		case "d", "w", "enter", "up", "down":
			return false
		default:
			return true
		}
	}
	return false
}
