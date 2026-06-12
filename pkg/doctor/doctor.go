package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/nawodyaishan/universal-mcp-sync/pkg/manifest"
)

func New(options Options) (*Doctor, error) {
	if options.HomeDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home directory: %w", err)
		}
		options.HomeDir = homeDir
	}
	if options.GOOS == "" {
		options.GOOS = runtime.GOOS
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.CommandTimeout <= 0 {
		options.CommandTimeout = time.Second
	}
	return &Doctor{
		options:  options,
		lookPath: defaultLookPath,
		runCmd:   defaultRunCmd,
	}, nil
}

func (d *Doctor) Scan(ctx context.Context) (Report, error) {
	clients, warnings, err := d.scanClients()
	if err != nil {
		return Report{}, err
	}

	report := Report{
		Platform: d.options.GOOS,
		Clients:  clients,
		Warnings: warnings,
	}
	if d.options.CheckRuntimes {
		report.Runtimes = d.scanRuntimes(ctx)
	}
	return report, nil
}

func (d *Doctor) scanClients() ([]ClientFinding, []string, error) {
	clients := manifest.ForPlatform(manifest.AllClients(), d.options.GOOS)
	results := make([]ClientFinding, 0, len(clients))
	warnings := make([]string, 0)

	for _, client := range clients {
		finding, extraWarnings, err := d.scanClient(client)
		if err != nil {
			return nil, nil, err
		}
		results = append(results, finding)
		warnings = append(warnings, extraWarnings...)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})
	return results, warnings, nil
}

func (d *Doctor) scanClient(client manifest.ClientManifest) (ClientFinding, []string, error) {
	finding := ClientFinding{
		ID:         client.ID,
		Name:       client.Name,
		Confidence: ConfidenceLow,
	}
	if client.CLIName != "" {
		if path, err := d.lookPath(client.CLIName); err == nil && path != "" {
			finding.CLIAvailable = true
			finding.Installed = true
		}
	}

	candidates := make([]CandidateFinding, 0, len(client.Candidates))
	providers := make(map[string]bool)
	existingCurrent := make([]CandidateFinding, 0)
	existingDeprecated := make([]CandidateFinding, 0)

	for _, candidate := range client.Candidates {
		candidateFinding, skip, err := d.scanCandidate(candidate)
		if err != nil {
			return ClientFinding{}, nil, fmt.Errorf("%s %s: %w", client.ID, candidate.Label, err)
		}
		if skip {
			continue
		}

		if candidateFinding.Exists {
			finding.Installed = true
			if candidate.Deprecated {
				existingDeprecated = append(existingDeprecated, candidateFinding)
			} else {
				existingCurrent = append(existingCurrent, candidateFinding)
			}
		}
		for _, providerID := range candidateFinding.Providers {
			providers[providerID] = true
		}
		if candidateFinding.ParseError != "" {
			finding.Issues = append(finding.Issues, fmt.Sprintf("%s: %s", candidateFinding.Path, candidateFinding.ParseError))
		} else if candidateFinding.Exists && !candidateFinding.RootKeyOK {
			finding.Issues = append(finding.Issues, fmt.Sprintf("%s: expected %q object", candidateFinding.Path, candidateFinding.RootKey))
		}
		candidates = append(candidates, candidateFinding)
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Path == candidates[j].Path {
			return candidates[i].Label < candidates[j].Label
		}
		return candidates[i].Path < candidates[j].Path
	})
	finding.Candidates = candidates
	finding.ConfiguredProviders = sortedProviderIDs(providers)

	if len(existingCurrent) > 1 {
		finding.Confidence = ConfidenceConflict
		finding.Issues = append(finding.Issues, "multiple current config candidates exist")
	} else if len(existingCurrent) == 1 {
		finding.Confidence = mappedConfidence(existingCurrent[0].Label, client.Candidates)
		finding.EffectivePath = existingCurrent[0].Path
	} else if len(existingDeprecated) > 0 {
		finding.Confidence = ConfidenceMedium
		finding.EffectivePath = existingDeprecated[0].Path
	} else if finding.CLIAvailable {
		finding.Confidence = ConfidenceMedium
	} else {
		finding.Confidence = ConfidenceLow
	}

	hints, clientWarnings, globalWarnings := d.clientHintsAndWarnings(client, finding)
	finding.MigrationHints = hints
	finding.Warnings = append(finding.Warnings, clientWarnings...)

	// Warn when Claude Code managed-settings are deployed (org policy may override usync).
	if client.ID == manifest.ClientClaudeCode {
		if detected := d.checkManagedSettings(); len(detected) > 0 {
			finding.Warnings = append(finding.Warnings,
				fmt.Sprintf("Claude Code managed configuration detected (%s); usync changes to user/local scope may be overridden by org policy.",
					strings.Join(detected, ", ")))
		}
	}

	return finding, globalWarnings, nil
}

var defaultManagedSettingsPaths = []string{
	"/etc/claude-code/managed-settings.json",
	"/etc/claude-code/managed-mcp.json",
}

// checkManagedSettings returns paths from ManagedSettingsPaths that exist on disk.
// nil ManagedSettingsPaths → use package defaults; empty (non-nil) slice → skip check.
func (d *Doctor) checkManagedSettings() []string {
	paths := d.options.ManagedSettingsPaths
	if paths == nil {
		paths = defaultManagedSettingsPaths
	}
	var found []string
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			found = append(found, p)
		}
	}
	return found
}

func (d *Doctor) scanCandidate(candidate manifest.ConfigCandidate) (CandidateFinding, bool, error) {
	path, err := manifest.ExpandPath(candidate.PathTemplate, manifest.PathVars{
		Home:      d.options.HomeDir,
		Workspace: d.options.WorkspaceDir,
	})
	if err != nil {
		if strings.Contains(err.Error(), "workspace") {
			return CandidateFinding{}, true, nil
		}
		return CandidateFinding{}, false, err
	}

	finding := CandidateFinding{
		Label:      candidate.Label,
		Path:       path,
		Scope:      candidate.Scope,
		Deprecated: candidate.Deprecated,
		RootKey:    candidate.RootKey,
		ParseOK:    true,
		RootKeyOK:  candidate.RootKey == "",
		Writable:   writableForMissing(path),
	}

	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return finding, false, nil
		}
		finding.ParseOK = false
		finding.ParseError = "stat failed"
		return finding, false, nil
	}

	finding.Exists = true
	finding.Writable = writableForExisting(path, info.Mode())
	if info.Mode()&os.ModeSymlink != 0 {
		finding.IsSymlink = true
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			finding.ParseOK = false
			finding.ParseError = "broken symlink"
			return finding, false, nil
		}
		finding.Resolved = resolved
	}

	data, err := os.ReadFile(path)
	if err != nil {
		finding.ParseOK = false
		finding.ParseError = "read failed"
		return finding, false, nil
	}

	providers, rootOK, rootType, parseErr := parseCandidateConfig(data, candidate.Format, candidate.RootKey)
	if parseErr != nil {
		finding.ParseOK = false
		finding.ParseError = parseErr.Error()
		return finding, false, nil
	}

	finding.Providers = providers
	finding.RootKeyOK = rootOK
	finding.RootType = rootType
	return finding, false, nil
}

func (d *Doctor) clientHintsAndWarnings(client manifest.ClientManifest, finding ClientFinding) ([]MigrationHint, []string, []string) {
	hints := make([]MigrationHint, 0)
	clientWarnings := make([]string, 0)
	globalWarnings := make([]string, 0)
	hasConfig := false
	for _, candidate := range finding.Candidates {
		if candidate.Exists {
			hasConfig = true
			break
		}
	}

	for _, warning := range client.Warnings {
		switch warning.Code {
		case "sunset":
			if finding.Installed && beforeOrOn(d.options.Now(), warning.Deadline, "2026-07-15") {
				clientWarnings = append(clientWarnings, warning.Message)
				globalWarnings = append(globalWarnings, fmt.Sprintf("%s: %s", client.Name, warning.Message))
				hints = append(hints, MigrationHint{
					FromID:   client.ID,
					ToID:     manifest.ClientAntigravityCLI,
					Reason:   warning.Message,
					Deadline: warning.Deadline,
				})
			}
		default:
			if finding.Installed || hasConfig {
				clientWarnings = append(clientWarnings, warning.Message)
			}
		}
	}

	if client.ID == manifest.ClientAntigravity && finding.Confidence == ConfidenceConflict {
		hints = append(hints, MigrationHint{
			FromID: client.ID,
			ToID:   manifest.ClientAntigravity,
			Reason: "resolve Antigravity config path conflict before applying changes",
		})
	}

	if client.ID == manifest.ClientWindsurf {
		for _, candidate := range finding.Candidates {
			if candidate.Exists && candidate.Deprecated {
				hints = append(hints, MigrationHint{
					FromID: client.ID,
					ToID:   client.ID,
					Reason: "legacy Windsurf config path detected",
				})
				break
			}
		}
	}

	return hints, clientWarnings, globalWarnings
}

func mappedConfidence(label string, candidates []manifest.ConfigCandidate) Confidence {
	for _, candidate := range candidates {
		if candidate.Label != label {
			continue
		}
		switch candidate.Confidence {
		case manifest.ConfidenceHigh:
			return ConfidenceHigh
		case manifest.ConfidenceMedium:
			return ConfidenceMedium
		default:
			return ConfidenceLow
		}
	}
	return ConfidenceLow
}

func sortedProviderIDs(seen map[string]bool) []string {
	providers := make([]string, 0, len(seen))
	for providerID := range seen {
		providers = append(providers, providerID)
	}
	slices.Sort(providers)
	return providers
}

func beforeOrOn(now time.Time, primaryDeadline, fallbackDeadline string) bool {
	primary, primaryOK := parseDeadline(primaryDeadline)
	fallback, fallbackOK := parseDeadline(fallbackDeadline)

	switch {
	case primaryOK && fallbackOK && fallback.After(primary):
		primary = fallback
	case !primaryOK && fallbackOK:
		primary = fallback
	case !primaryOK:
		return false
	}

	return !now.After(primary.Add(24*time.Hour - time.Nanosecond))
}

func parseDeadline(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}
