package uxexplore

import (
	"fmt"
	"strings"

	"github.com/nawodyaishan/universal-mcp-sync/pkg/tui"
)

// Invariant is a per-visit check applied to every reachable state during
// exploration. Implementations must be cheap (microseconds) — heavy structural
// analysis lives in the analyzer (PR 14d).
type Invariant interface {
	ID() string
	Check(m tui.DashboardModel) error
}

// Invariants returns the full per-visit invariant set used by the probe. The
// order is stable so findings ordering stays deterministic.
func Invariants() []Invariant {
	return []Invariant{
		I01FooterActionsValid{},
		I02HiddenKeysNoAdvance{},
		I03RepeatedPrimaryStable{},
		I04RequiresCredsBlocksPlan{},
		I05NoKeyPlansWithoutCreds{},
		I06CheckboxMatchesPlan{},
		I07NoTargetsNoPlan{},
		I08ConflictBlocksPlan{},
		I09ResolvedConflictNotBlocked{},
		I10ChosenCandidateReachesPlan{},
		I11SkippedConflictNotPlanned{},
		I12WorkspaceToggleHasEffect{},
		I13ErrorOffersRecovery{},
		I14EscRestoresPrevious{},
		I15NoRawCredentialInView{},
		I16CursorOnRenderedRow{},
		I17ProgressEdgeExists{},
	}
}

// --- I-01 -----------------------------------------------------------------
type I01FooterActionsValid struct{}

func (I01FooterActionsValid) ID() string { return "I-01" }
func (I01FooterActionsValid) Check(m tui.DashboardModel) error {
	view := m.View()
	keys := ParseActionBar(view)
	if len(keys) == 0 {
		return fmt.Errorf("footer action bar empty for screen %s", m.Snapshot().Screen)
	}
	return nil
}

// --- I-02 -----------------------------------------------------------------
// I-02 is fully verified by the probe's unmapped-key pass (it requires
// firing keys, which is a probe responsibility). The per-visit check is a
// no-op so the invariant ID stays referenceable from findings.
type I02HiddenKeysNoAdvance struct{}

func (I02HiddenKeysNoAdvance) ID() string                       { return "I-02" }
func (I02HiddenKeysNoAdvance) Check(_ tui.DashboardModel) error { return nil }

// --- I-03 -----------------------------------------------------------------
// I-03 is fully verified by the probe's double-press pass. Per-visit no-op.
type I03RepeatedPrimaryStable struct{}

func (I03RepeatedPrimaryStable) ID() string                       { return "I-03" }
func (I03RepeatedPrimaryStable) Check(_ tui.DashboardModel) error { return nil }

// --- I-04 -----------------------------------------------------------------
type I04RequiresCredsBlocksPlan struct{}

func (I04RequiresCredsBlocksPlan) ID() string { return "I-04" }
func (I04RequiresCredsBlocksPlan) Check(m tui.DashboardModel) error {
	s := m.Snapshot()
	if s.MissingCredentials && (s.Screen == "PlanPreview" || s.InFlight == "planning") {
		return fmt.Errorf("plan reached with missing credentials")
	}
	return nil
}

// --- I-05 -----------------------------------------------------------------
// I-05 (no-key providers remain plannable) is asserted by fixture-driven
// coverage rather than a per-visit check.
type I05NoKeyPlansWithoutCreds struct{}

func (I05NoKeyPlansWithoutCreds) ID() string                       { return "I-05" }
func (I05NoKeyPlansWithoutCreds) Check(_ tui.DashboardModel) error { return nil }

// --- I-06 -----------------------------------------------------------------
// I-06 (visible checkbox == planned) needs comparison against the rendered
// plan output; lives in the analyzer once findings include plan content.
type I06CheckboxMatchesPlan struct{}

func (I06CheckboxMatchesPlan) ID() string                       { return "I-06" }
func (I06CheckboxMatchesPlan) Check(_ tui.DashboardModel) error { return nil }

// --- I-07 -----------------------------------------------------------------
type I07NoTargetsNoPlan struct{}

func (I07NoTargetsNoPlan) ID() string { return "I-07" }
func (I07NoTargetsNoPlan) Check(m tui.DashboardModel) error {
	s := m.Snapshot()
	if s.NoTargetsSelected && s.InFlight == "planning" {
		return fmt.Errorf("plan started with zero selected targets")
	}
	return nil
}

// --- I-08 -----------------------------------------------------------------
type I08ConflictBlocksPlan struct{}

func (I08ConflictBlocksPlan) ID() string { return "I-08" }
func (I08ConflictBlocksPlan) Check(m tui.DashboardModel) error {
	s := m.Snapshot()
	if s.ConflictUnresolved && (s.Screen == "PlanPreview" || s.InFlight == "planning") {
		return fmt.Errorf("plan reached with unresolved conflicts")
	}
	return nil
}

// --- I-09 -----------------------------------------------------------------
// I-09 is best verified by the explorer's edge graph (conflict resolved →
// readiness cleared). Per-visit no-op.
type I09ResolvedConflictNotBlocked struct{}

func (I09ResolvedConflictNotBlocked) ID() string                       { return "I-09" }
func (I09ResolvedConflictNotBlocked) Check(_ tui.DashboardModel) error { return nil }

// --- I-10 -----------------------------------------------------------------
type I10ChosenCandidateReachesPlan struct{}

func (I10ChosenCandidateReachesPlan) ID() string                       { return "I-10" }
func (I10ChosenCandidateReachesPlan) Check(_ tui.DashboardModel) error { return nil }

// --- I-11 -----------------------------------------------------------------
type I11SkippedConflictNotPlanned struct{}

func (I11SkippedConflictNotPlanned) ID() string                       { return "I-11" }
func (I11SkippedConflictNotPlanned) Check(_ tui.DashboardModel) error { return nil }

// --- I-12 -----------------------------------------------------------------
type I12WorkspaceToggleHasEffect struct{}

func (I12WorkspaceToggleHasEffect) ID() string                       { return "I-12" }
func (I12WorkspaceToggleHasEffect) Check(_ tui.DashboardModel) error { return nil }

// --- I-13 -----------------------------------------------------------------
type I13ErrorOffersRecovery struct{}

func (I13ErrorOffersRecovery) ID() string { return "I-13" }
func (I13ErrorOffersRecovery) Check(m tui.DashboardModel) error {
	s := m.Snapshot()
	hasError := s.HasScanError || s.HasPlanError || s.HasApplyError || s.HasValidationError
	if !hasError {
		return nil
	}
	keys := ParseActionBar(m.View())
	for _, k := range keys {
		switch k {
		case "q", "?", "ctrl+c":
			continue
		default:
			return nil
		}
	}
	return fmt.Errorf("error state %s has no recovery key (only quit/help advertised)", s.Screen)
}

// --- I-14 -----------------------------------------------------------------
type I14EscRestoresPrevious struct{}

func (I14EscRestoresPrevious) ID() string                       { return "I-14" }
func (I14EscRestoresPrevious) Check(_ tui.DashboardModel) error { return nil }

// --- I-15 -----------------------------------------------------------------
// I-15 (no raw credential in output) — the per-visit check scans the rendered
// view for known credential profile values supplied via the Driver. The probe
// is responsible for passing the credentials through this check; if a value is
// present, the invariant fails. Static probes can't know the credentials, so
// this implementation is a placeholder that always passes. The real check
// lives in `pkg/uxexplore/redaction_check.go` once driver-level credential
// awareness lands in PR 14g.
type I15NoRawCredentialInView struct{}

func (I15NoRawCredentialInView) ID() string                       { return "I-15" }
func (I15NoRawCredentialInView) Check(_ tui.DashboardModel) error { return nil }

// --- I-16 -----------------------------------------------------------------
type I16CursorOnRenderedRow struct{}

func (I16CursorOnRenderedRow) ID() string { return "I-16" }
func (I16CursorOnRenderedRow) Check(m tui.DashboardModel) error {
	s := m.Snapshot()
	switch s.Screen {
	case "ProviderReady":
		if !s.RenderedProviderCursor {
			return fmt.Errorf("providerCursor=%d does not point at a rendered row", s.ProviderCursor)
		}
	case "TargetSelect":
		if !s.RenderedTargetCursor {
			return fmt.Errorf("clientCursor=%d does not point at a rendered row", s.ClientCursor)
		}
	}
	return nil
}

// --- I-17 -----------------------------------------------------------------
// I-17 per-visit weak form: non-terminal states must have a non-empty action
// bar. The full edge-graph version lives in the analyzer (PR 14d).
type I17ProgressEdgeExists struct{}

func (I17ProgressEdgeExists) ID() string { return "I-17" }
func (I17ProgressEdgeExists) Check(m tui.DashboardModel) error {
	fp := Fingerprint(m)
	if IsTerminal(fp) {
		return nil
	}
	keys := ParseActionBar(m.View())
	progress := false
	for _, k := range keys {
		switch strings.ToLower(k) {
		case "q", "?", "ctrl+c", "esc":
			continue
		default:
			progress = true
		}
	}
	if !progress {
		return fmt.Errorf("non-terminal state %s/%s has no progress edge in action bar", fp.Screen, fp.PreconditionClass)
	}
	return nil
}
