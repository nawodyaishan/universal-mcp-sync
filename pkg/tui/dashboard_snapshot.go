package tui

// DashboardSnapshot is a read-only projection of dashboard state for
// diagnostics and UX exploration. It intentionally exposes strings and simple
// counters instead of mutable dashboard internals.
type DashboardSnapshot struct {
	Screen                 string
	BlockReason            string
	HasScanError           bool
	HasPlanError           bool
	HasApplyError          bool
	HasValidationError     bool
	InFlight               string
	MissingCredentials     bool
	ConflictUnresolved     bool
	NoTargetsSelected      bool
	RuntimeMissing         bool
	ProviderCursor         int
	ClientCursor           int
	RenderedProviderCursor bool
	RenderedTargetCursor   bool
}

func (m DashboardModel) Snapshot() DashboardSnapshot {
	s := DashboardSnapshot{
		Screen:             dashboardScreenName(m.screen),
		HasScanError:       m.err != nil,
		HasPlanError:       m.planErr != nil,
		HasApplyError:      m.applyErr != nil,
		HasValidationError: m.validErr != nil,
		ProviderCursor:     m.providerCursor,
		ClientCursor:       m.clientCursor,
	}
	if m.screen != screenWelcome {
		s.MissingCredentials = m.selectedProviderNeedsCredentials()
		s.ConflictUnresolved = hasConflictClient(m.report)
		s.NoTargetsSelected = m.screen == screenTargetSelect && m.selectedTargetCount() == 0
	}
	switch {
	case m.scanning:
		s.InFlight = "scanning"
	case m.validating:
		s.InFlight = "validating"
	case m.planning || m.preflighting:
		s.InFlight = "planning"
	case m.applying:
		s.InFlight = "applying"
	}

	if len(m.readiness) > 0 {
		rendered := RenderedProviderIndices(m.readiness, hasConflictClient(m.report))
		for _, idx := range rendered {
			if idx == m.providerCursor {
				s.RenderedProviderCursor = true
				break
			}
		}
		if m.providerCursor >= 0 && m.providerCursor < len(m.readiness) {
			s.RuntimeMissing = m.readiness[m.providerCursor].State == ProviderStateRuntimeMissing
		}
	}
	entries := allTargetEntries(m.report, m.resolvedConflicts, m.includeWorkspace)
	s.RenderedTargetCursor = len(entries) == 0 || (m.clientCursor >= 0 && m.clientCursor < len(entries))
	s.BlockReason = dashboardBlockReason(s)
	return s
}

func dashboardBlockReason(s DashboardSnapshot) string {
	switch {
	case s.HasScanError:
		return "scan error"
	case s.HasApplyError:
		return "apply error"
	case s.HasPlanError:
		return "plan error"
	case s.HasValidationError:
		return "validation error"
	case s.RuntimeMissing:
		return "runtime missing"
	case s.ConflictUnresolved:
		return "conflict unresolved"
	case s.MissingCredentials:
		return "missing credentials"
	case s.NoTargetsSelected:
		return "no targets selected"
	default:
		return ""
	}
}

func dashboardScreenName(screen dashboardScreen) string {
	switch screen {
	case screenWelcome:
		return "Welcome"
	case screenDoctor:
		return "Doctor"
	case screenProviderReady:
		return "ProviderReady"
	case screenTargetSelect:
		return "TargetSelect"
	case screenPlanPreview:
		return "PlanPreview"
	case screenApplyResult:
		return "ApplyResult"
	case screenConflictResolve:
		return "ConflictResolve"
	case screenCredentialEntry:
		return "CredentialEntry"
	default:
		return "Unknown"
	}
}
