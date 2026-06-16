package tui

import (
	"github.com/nawodyaishan/universal-mcp-sync/pkg/doctor"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/manifest"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/provider"
)

// ProviderState is the readiness classification for a single provider.
type ProviderState string

const (
	ProviderStateNoKeyNeeded       ProviderState = "no-key-needed"
	ProviderStateReady             ProviderState = "ready"
	ProviderStateMissingCredential ProviderState = "missing-credentials"
	ProviderStateRuntimeMissing    ProviderState = "runtime-missing"
	ProviderStateConflictBlocked   ProviderState = "conflict-blocked"
)

// ProviderReadinessItem holds readiness state for one provider.
type ProviderReadinessItem struct {
	Meta    manifest.ProviderMeta
	State   ProviderState
	Reasons []string // human-readable blockers
}

// ComputeReadiness classifies all known providers given a doctor report and supplied profiles.
// It is a pure function with no I/O.
//
// Severity order (highest wins when multiple apply):
//
//	conflict-blocked > runtime-missing > missing-credentials > ready > no-key-needed
func ComputeReadiness(
	providers []manifest.ProviderMeta,
	report doctor.Report,
	profiles []provider.CredentialProfile,
) []ProviderReadinessItem {
	runtimeAvailable := runtimeAvailabilityMap(report)
	profiledProviders := profiledProviderIDs(profiles)

	items := make([]ProviderReadinessItem, 0, len(providers))
	for _, meta := range providers {
		item := ProviderReadinessItem{Meta: meta}

		switch {
		case len(meta.RuntimeIDs) > 0 && !allRuntimesAvailable(meta.RuntimeIDs, runtimeAvailable):
			item.State = ProviderStateRuntimeMissing
			for _, id := range meta.RuntimeIDs {
				if !runtimeAvailable[id] {
					item.Reasons = append(item.Reasons, "runtime missing: "+id)
				}
			}

		case len(meta.Credentials) == 0:
			item.State = ProviderStateNoKeyNeeded

		case profiledProviders[string(meta.ID)]:
			item.State = ProviderStateReady

		default:
			item.State = ProviderStateMissingCredential
			for _, cred := range meta.Credentials {
				if cred.GetURL != "" {
					item.Reasons = append(item.Reasons, "get key: "+cred.GetURL)
				}
			}
			item.Reasons = append(item.Reasons, "press [k] or [Enter] to add your API key")
		}

		items = append(items, item)
	}
	return items
}

// RenderedProviderIndices returns indices of readiness items whose rows will be
// rendered on the Provider Readiness screen. When conflicts are present every
// conflict-blocked provider is hidden behind the banner, so the cursor and the
// Enter/v key handlers must skip those indices to avoid silent validation against
// invisible rows (Phase 13 anchor — DM-P40).
func RenderedProviderIndices(items []ProviderReadinessItem, hasConflicts bool) []int {
	out := make([]int, 0, len(items))
	for i, it := range items {
		if hasConflicts && it.State == ProviderStateConflictBlocked {
			continue
		}
		out = append(out, i)
	}
	return out
}

func hasConflictClient(report doctor.Report) bool {
	for _, c := range report.Clients {
		if c.Confidence == doctor.ConfidenceConflict {
			return true
		}
	}
	return false
}

func runtimeAvailabilityMap(report doctor.Report) map[string]bool {
	m := make(map[string]bool, len(report.Runtimes))
	for _, rt := range report.Runtimes {
		m[rt.ID] = rt.Available
	}
	return m
}

// allRuntimesAvailable returns false only when a runtime is explicitly reported
// as unavailable. Runtimes absent from the report are treated as unknown, not missing.
func allRuntimesAvailable(ids []string, available map[string]bool) bool {
	for _, id := range ids {
		if avail, found := available[id]; found && !avail {
			return false
		}
	}
	return true
}

func profiledProviderIDs(profiles []provider.CredentialProfile) map[string]bool {
	m := make(map[string]bool, len(profiles))
	for _, p := range profiles {
		if p.ProviderID != "" {
			m[p.ProviderID] = true
		}
	}
	return m
}
