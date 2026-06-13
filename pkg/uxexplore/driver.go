package uxexplore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"

	"github.com/nawodyaishan/universal-mcp-sync/pkg/tui"
)

type Driver struct {
	spec    FixtureSpec
	scanner FakeScanner
	manager FakeDashboardManager
}

func NewDriver(spec FixtureSpec) (*Driver, error) {
	home, err := os.MkdirTemp("", "uxexplore-*")
	if err != nil {
		return nil, err
	}
	return &Driver{
		spec:    spec,
		scanner: BuildScanner(spec),
		manager: BuildManager(spec, home),
	}, nil
}

func (d *Driver) Run(ctx context.Context) (*Trace, error) {
	_ = ctx
	m := tui.NewDashboardModelWithWelcome(d.scanner, d.manager, BuildProfiles(d.spec))
	if cmd := m.Init(); cmd != nil {
		next, _ := m.Update(cmd())
		m = next.(tui.DashboardModel)
	}
	trace := &Trace{Fixture: d.spec}
	trace.Visited = append(trace.Visited, VisitedState{
		Fingerprint: Fingerprint(m),
		ViewDigest:  viewDigest(m.View()),
		ModelSnap:   ModelSnap{Screen: m.Snapshot().Screen, View: m.View()},
	})
	return trace, nil
}

// Explore runs the full probe from the post-init state. The returned trace
// contains every reachable state, edge, and invariant violation discovered.
func (d *Driver) Explore(ctx context.Context) (*Trace, error) {
	m, err := d.StartModel(ctx)
	if err != nil {
		return nil, err
	}
	trace := &Trace{Fixture: d.spec}
	NewProbe().Visit(m, trace)
	return trace, nil
}

// StartModel constructs the driver's DashboardModel and drains any post-Init
// command so callers receive the same starting state the probe uses. The
// current product starts at the welcome mode chooser, so Init is usually nil.
func (d *Driver) StartModel(ctx context.Context) (tui.DashboardModel, error) {
	_ = ctx
	m := tui.NewDashboardModelWithWelcome(d.scanner, d.manager, BuildProfiles(d.spec))
	if cmd := m.Init(); cmd != nil {
		if msg := cmd(); msg != nil {
			next, _ := m.Update(msg)
			m = next.(tui.DashboardModel)
		}
	}
	return m, nil
}

func viewDigest(view string) string {
	sum := sha256.Sum256([]byte(view))
	return hex.EncodeToString(sum[:])
}
