package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/nawodyaishan/universal-mcp-sync/pkg/app"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/doctor"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/redact"
)

// View renders the current state of the dashboard.
func (m DashboardModel) View() string {
	shell := func(body string) string {
		return renderDashboardShell(m.applyRecordingHeader(body), m.screen, m.width)
	}
	if m.showHelp {
		return shell(renderDashboardHelpOverlay(m.screen))
	}

	switch m.screen {
	case screenWelcome:
		return shell(m.renderWelcome())
	case screenProviderReady:
		return shell(m.renderProviderReady())
	case screenTargetSelect:
		return shell(m.renderTargetSelect())
	case screenConflictResolve:
		return shell(m.renderConflictResolve())
	case screenCredentialEntry:
		return shell(m.renderCredentialEntry())
	case screenPlanPreview:
		return shell(m.renderPlanPreview())
	case screenApplyResult:
		return shell(m.renderApplyResult())
	}

	// screenDoctor — Phase 7 base
	var content string
	if m.scanning {
		content = "Scanning for AI clients and runtimes...\n"
	} else if m.err != nil {
		content = fmt.Sprintf("Error scanning clients: %v\n", m.err)
	} else {
		content = m.renderReport()
	}

	if m.placeholderMsg != "" {
		content += "\n[Status] " + m.placeholderMsg + "\n"
	}

	content += "\n" + actionBarDoctor(m.err != nil, m.manager != nil)
	return renderDashboardShell(m.applyRecordingHeader(content), m.screen, m.width)
}

// applyRecordingHeader prepends a `● rec` strip when the model has a session
// recorder attached. The marker keeps users aware that keystrokes are being
// captured to disk, even when help/overlay screens are active.
func (m DashboardModel) applyRecordingHeader(body string) string {
	if m.recorder == nil {
		return body
	}
	return "● rec  recording session to " + m.recorder.Path() + "\n\n" + body
}

// --- Phase 7 report rendering (unchanged) ---

func (m DashboardModel) renderWelcome() string {
	var b strings.Builder
	b.WriteString(dashboardHeading("Welcome", "Choose how you want to work with MCP configuration."))

	type option struct {
		title string
		body  string
	}
	options := []option{
		{
			title: "Doctor Mode",
			body:  "Scan installed AI clients, detect MCP config paths, surface conflicts, then guide provider selection and apply.",
		},
		{
			title: "Wizard Mode",
			body:  "Use the legacy step-by-step setup flow when you already know the provider, target apps, and credentials.",
		},
	}
	for i, opt := range options {
		b.WriteString(dashboardCard(opt.title, opt.body, i == m.welcomeCursor))
		b.WriteString("\n")
	}

	return b.String() + "\n" + actionBarWelcome()
}

func actionBarWelcome() string {
	return "[↑↓] choose  [Enter] start  [d] doctor  [w] wizard  [?] help  [q] quit"
}

func (m DashboardModel) renderReport() string {
	var b strings.Builder

	clients := m.report.Clients
	conflicts := 0
	for _, client := range clients {
		if client.Confidence == doctor.ConfidenceConflict {
			conflicts++
		}
	}
	warnings := len(m.report.Warnings)
	b.WriteString(dashboardHeading("System Status", "Doctor scan summary for installed AI clients and MCP config surfaces."))
	b.WriteString(dashboardMetrics(
		dashboardMetric{Label: "clients", Value: fmt.Sprintf("%d", len(clients)), Tone: toneInfo},
		dashboardMetric{Label: "runtimes", Value: fmt.Sprintf("%d", len(m.report.Runtimes)), Tone: runtimeMetricTone(m.report)},
		dashboardMetric{Label: "warnings", Value: fmt.Sprintf("%d", warnings), Tone: countTone(warnings)},
		dashboardMetric{Label: "conflicts", Value: fmt.Sprintf("%d", conflicts), Tone: countTone(conflicts)},
	))

	if len(m.report.Runtimes) > 0 {
		b.WriteString(sectionTitleStyle.Render("Runtimes") + "\n")
		for _, rt := range m.report.Runtimes {
			status := dashboardBadge("OK", toneOK)
			if !rt.Available {
				status = dashboardBadge("MISSING", toneError)
			} else if rt.Error != "" {
				status = dashboardBadge("WARNING", toneWarn)
			}
			fmt.Fprintf(&b, "  %s %s\n", status, rt.Name)
			if rt.Error != "" {
				fmt.Fprintf(&b, "    %s\n", redact.Key(rt.Error))
			}
		}
		b.WriteString("\n")
	}

	if len(m.report.Warnings) > 0 {
		b.WriteString(sectionTitleStyle.Render("Global Warnings") + "\n")
		for _, w := range m.report.Warnings {
			fmt.Fprintf(&b, "  %s %s\n", dashboardBadge("WARN", toneWarn), redact.Key(w))
		}
		b.WriteString("\n")
	}

	if len(clients) == 0 {
		b.WriteString(dashboardCallout(toneWarn, "No AI clients detected", "Install or launch a supported client, then rescan."))
	} else {
		fmt.Fprintf(&b, "%s %s\n", sectionTitleStyle.Render("AI Clients Detected"), dashboardBadge(fmt.Sprintf("%d", len(clients)), toneInfo))
		for _, client := range clients {
			if client.Confidence == doctor.ConfidenceConflict {
				m.renderClient(&b, client)
			}
		}
		for _, client := range clients {
			if client.Confidence != doctor.ConfidenceConflict {
				m.renderClient(&b, client)
			}
		}
	}

	return b.String()
}

func (m DashboardModel) renderClient(b *strings.Builder, client doctor.ClientFinding) {
	status := dashboardBadge(string(client.Confidence), clientTone(client))
	if !client.Installed {
		status = dashboardBadge("not installed", toneNeutral)
	}
	fmt.Fprintf(b, "  %s %s\n", status, client.Name)
	if client.EffectivePath != "" {
		fmt.Fprintf(b, "    %s %s\n", mutedStyle.Render("Config"), redact.Key(client.EffectivePath))
	}
	if len(client.ConfiguredProviders) > 0 {
		fmt.Fprintf(b, "    %s %s\n", mutedStyle.Render("Providers"), strings.Join(client.ConfiguredProviders, ", "))
	}
	for _, issue := range client.Issues {
		fmt.Fprintf(b, "    %s Issue: %s\n", dashboardBadge("ISSUE", toneError), redact.Key(issue))
	}
	for _, warning := range client.Warnings {
		fmt.Fprintf(b, "    %s Warning: %s\n", dashboardBadge("WARN", toneWarn), redact.Key(warning))
	}
}

// --- Phase 8 screen renders ---

func (m DashboardModel) renderProviderReady() string {
	var b strings.Builder
	b.WriteString(dashboardHeading("Provider Readiness", "Choose the MCP server profile to install."))

	if m.computingReady {
		b.WriteString(dashboardCallout(toneInfo, "Computing readiness", "Checking credentials, runtime dependencies, and detected conflicts."))
		return b.String() + actionBarProviderReady(false, false, false, false)
	}
	if m.readinessErr != nil {
		b.WriteString(dashboardCallout(toneError, "Readiness error", redact.Text(m.readinessErr.Error())))
		return b.String() + actionBarProviderReady(false, false, false, false)
	}

	// Conflict banner — shown when conflict clients are blocking planning.
	conflictClients := conflictClientsInReport(m.report)
	hasConflicts := len(conflictClients) > 0
	if hasConflicts {
		lines := append([]string{"Resolve these before planning:"}, conflictClients...)
		lines = append(lines, "Press [r] to go to conflict resolution.")
		b.WriteString(dashboardCallout(toneWarn, "Conflicts detected", lines...))
	}

	if m.validErr != nil {
		b.WriteString(dashboardCallout(toneError, "Validation error", redact.Text(m.validErr.Error())))
	}
	if m.validating {
		b.WriteString(dashboardCallout(toneInfo, "Validating...", "Running offline checks for the selected provider."))
	}

	rendered := RenderedProviderIndices(m.readiness, hasConflicts)
	hasSelectable := len(rendered) > 0
	b.WriteString(dashboardMetrics(
		dashboardMetric{Label: "providers", Value: fmt.Sprintf("%d", len(m.readiness)), Tone: toneInfo},
		dashboardMetric{Label: "ready", Value: fmt.Sprintf("%d", readinessCount(m.readiness, ProviderStateReady)+readinessCount(m.readiness, ProviderStateNoKeyNeeded)), Tone: toneOK},
		dashboardMetric{Label: "attention", Value: fmt.Sprintf("%d", readinessAttentionCount(m.readiness)), Tone: countTone(readinessAttentionCount(m.readiness))},
	))

	// Pre-render provider items in display order (ready first, then blocked/missing).
	type providerRow struct {
		readinessIdx int
		text         string // pre-rendered, may span multiple lines
		lineCount    int
	}
	var rows []providerRow
	for i, item := range m.readiness {
		if item.State != ProviderStateReady && item.State != ProviderStateNoKeyNeeded {
			continue
		}
		cursor := "  "
		if i == m.providerCursor {
			cursor = accentStyle.Render("> ")
		}
		rows = append(rows, providerRow{i, fmt.Sprintf("%s%s %s\n", cursor, providerStateBadge(item.State), item.Meta.Name), 1})
	}
	for i, item := range m.readiness {
		if item.State == ProviderStateReady || item.State == ProviderStateNoKeyNeeded {
			continue
		}
		if hasConflicts && item.State == ProviderStateConflictBlocked {
			continue
		}
		cursor := "  "
		if i == m.providerCursor {
			cursor = accentStyle.Render("> ")
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "%s%s %s\n", cursor, providerStateBadge(item.State), item.Meta.Name)
		for _, reason := range item.Reasons {
			fmt.Fprintf(&sb, "      %s\n", mutedStyle.Render(reason))
		}
		rows = append(rows, providerRow{i, sb.String(), 1 + len(item.Reasons)})
	}

	// Apply scroll window when height is known and total exceeds available lines.
	firstRow, lastRow := 0, len(rows)
	if m.height > 0 && len(rows) > 0 {
		bodyPrefixLines := strings.Count(b.String(), "\n")
		const shellFrame = 7
		const footerLines = 3
		const indicatorLines = 2
		recordingLines := 0
		if m.recorder != nil {
			recordingLines = 2
		}
		avail := m.height - shellFrame - recordingLines - bodyPrefixLines - footerLines - indicatorLines

		// Find cursor position in rows.
		cursorPos := 0
		for j, r := range rows {
			if r.readinessIdx == m.providerCursor {
				cursorPos = j
				break
			}
		}

		// Expand window from cursor until we exhaust available lines.
		firstRow, lastRow = cursorPos, cursorPos+1
		used := rows[cursorPos].lineCount
		for used < avail {
			expanded := false
			if firstRow > 0 && used+rows[firstRow-1].lineCount <= avail {
				firstRow--
				used += rows[firstRow].lineCount
				expanded = true
			}
			if lastRow < len(rows) && used+rows[lastRow].lineCount <= avail {
				used += rows[lastRow].lineCount
				lastRow++
				expanded = true
			}
			if !expanded {
				break
			}
		}
	}

	if firstRow > 0 {
		fmt.Fprintf(&b, "  ↑ %d more\n", firstRow)
	}
	for _, row := range rows[firstRow:lastRow] {
		b.WriteString(row.text)
	}
	if lastRow < len(rows) {
		fmt.Fprintf(&b, "  ↓ %d more\n", len(rows)-lastRow)
	}

	if !hasSelectable && hasConflicts {
		b.WriteString(dashboardCallout(toneWarn, "Provider selection blocked", "No providers can be selected until the conflicts above are resolved."))
	}

	b.WriteString("\n")
	return b.String() + actionBarProviderReady(hasConflicts, hasSelectable, m.validating, m.cursorProviderNeedsCredentials())
}

// conflictClientsInReport returns names of clients with ConfidenceConflict.
func conflictClientsInReport(report doctor.Report) []string {
	var names []string
	for _, c := range report.Clients {
		if c.Confidence == doctor.ConfidenceConflict {
			names = append(names, c.Name)
		}
	}
	return names
}

func actionBarProviderReady(hasConflicts, hasSelectable, validating bool, needsCredentials bool) string {
	if hasConflicts && !hasSelectable {
		// Phase 13 DM-P40: no provider is renderable. [Enter] is a synonym for
		// [r] in this state (see handleKeyProviderReady), so advertise both.
		return "[r/Enter] resolve conflicts  [Esc] back  [?] help  [q] quit"
	}
	if validating {
		if hasConflicts {
			return "[r] resolve conflicts  [↑↓] navigate  [Esc] back  [q] quit"
		}
		return "[↑↓] navigate  [Esc] back  [?] help  [q] quit"
	}
	if hasConflicts {
		return "[r] resolve conflicts  [↑↓] navigate  [Enter] select provider  [Esc] back  [q] quit"
	}
	if needsCredentials {
		return "[↑↓] navigate  [k] add credentials  [Esc] back  [?] help  [q] quit\n" + guidanceFor(screenProviderReady, footerMissingCredentials)
	}
	return "[↑↓] navigate  [v] live validate  [Enter] select  [Esc] back  [?] help  [q] quit"
}

func (m DashboardModel) renderTargetSelect() string {
	var b strings.Builder
	b.WriteString(dashboardHeading("Select Targets", "Pick the client config files that should receive this MCP server."))

	// Show which provider is being configured and which credential profile is active.
	if m.selectedProv >= 0 && m.selectedProv < len(m.readiness) {
		item := m.readiness[m.selectedProv]
		line := "MCP: " + item.Meta.Name
		for _, p := range m.profiles {
			if p.ProviderID == string(item.Meta.ID) {
				line += "  (" + p.Label + ")"
				break
			}
		}
		b.WriteString(mutedStyle.Render(line) + "\n\n")
	}

	if m.planning || m.preflighting {
		b.WriteString(dashboardCallout(toneInfo, "Building plan", "Preparing a saved plan and running preflight checks."))
		return b.String() + actionBarTargetSelect(m.includeWorkspace, false, false, false, false, false, false)
	}
	if m.planErr != nil {
		b.WriteString(dashboardCallout(toneError, "Plan error", redact.Text(m.planErr.Error())))
	}

	entries := allTargetEntries(m.report, m.resolvedConflicts, m.includeWorkspace)
	selectedRows := m.effectiveSelectedTargets(entries)
	onConflict := m.clientCursor < len(entries) && entries[m.clientCursor].isConflict
	credentialBlocked := m.selectedProviderNeedsCredentials()
	selectedTargets := m.selectedTargetCount()
	b.WriteString(dashboardMetrics(
		dashboardMetric{Label: "selected", Value: fmt.Sprintf("%d", selectedTargets), Tone: selectedCountTone(selectedTargets)},
		dashboardMetric{Label: "available", Value: fmt.Sprintf("%d", selectableTargetCount(entries)), Tone: toneInfo},
		dashboardMetric{Label: "workspace", Value: workspaceStateLabel(m.includeWorkspace), Tone: workspaceTone(m.includeWorkspace)},
		dashboardMetric{Label: "conflicts", Value: fmt.Sprintf("%d", conflictTargetCount(entries)), Tone: countTone(conflictTargetCount(entries))},
	))

	if credentialBlocked {
		prov, _ := m.selectedProvider()
		name := "selected provider"
		if prov != nil {
			name = prov.Name()
		}
		b.WriteString(dashboardCallout(toneWarn, "Credential profile required", fmt.Sprintf("Add credentials for %s before planning.", name), "Press [k] to add credentials or [Esc] to return to Provider Readiness."))
	} else if selectedTargets == 0 {
		b.WriteString(dashboardCallout(toneWarn, "No targets selected", "Select at least one target before planning."))
	}

	// Compute scroll window: each entry renders as 2 lines (name + path).
	// Dynamically account for everything already written and the shell frame.
	const entryLines = 2
	first, last := 0, len(entries)
	if m.height > 0 {
		// Lines already written in this body (title, separator, messages).
		bodyPrefixLines := strings.Count(b.String(), "\n")
		// Shell frame: top-pad(1) + header(1) + blank(1) + stage(1) + 2-blanks(2) + bot-pad(1) = 7
		const shellFrame = 7
		// Recording header added by applyRecordingHeader: "● rec...\n" + "\n" = 2 lines.
		recordingLines := 0
		if m.recorder != nil {
			recordingLines = 2
		}
		// Footer: "\n" + action-bar(1-2) = 3 lines.
		const footerLines = 3
		// The two scroll indicators themselves take 1 line each when shown.
		const indicatorLines = 2
		avail := m.height - shellFrame - recordingLines - bodyPrefixLines - footerLines - indicatorLines
		if avail > 0 && len(entries)*entryLines > avail {
			vis := avail / entryLines
			if vis < 2 {
				vis = 2
			}
			first = m.clientCursor - vis/2
			if first < 0 {
				first = 0
			}
			last = first + vis
			if last > len(entries) {
				last = len(entries)
				first = last - vis
				if first < 0 {
					first = 0
				}
			}
		}
	}
	if first > 0 {
		fmt.Fprintf(&b, "  ↑ %d more\n", first)
	}

	// Eligible entries (non-conflict).
	for i, entry := range entries {
		if i < first || i >= last {
			continue
		}
		if entry.isConflict {
			continue
		}
		check := "[ ]"
		if selectedRows[entry.id] {
			check = "[x]"
		}
		cursor := "  "
		if i == m.clientCursor {
			cursor = accentStyle.Render("> ")
		}
		fmt.Fprintf(&b, "%s%s %s %s\n", cursor, check, targetScopeBadge(string(entry.scope)), targetEntryLabel(entry))
		if entry.path != "" {
			scope := string(entry.scope)
			if scope == "" {
				scope = "user"
			}
			fmt.Fprintf(&b, "     %s  %s\n", redact.Key(entry.path), mutedStyle.Render(scope))
		}
	}
	if last < len(entries) {
		fmt.Fprintf(&b, "  ↓ %d more\n", len(entries)-last)
	}

	// Conflict entries section.
	hasConflicts := false
	for _, entry := range entries {
		if entry.isConflict {
			hasConflicts = true
			break
		}
	}
	if hasConflicts {
		b.WriteString("\n")
		b.WriteString(sectionTitleStyle.Render("Conflict clients") + " " + mutedStyle.Render("(press r to resolve)") + "\n")
		for i, entry := range entries {
			if !entry.isConflict {
				continue
			}
			cursor := "  "
			if i == m.clientCursor {
				cursor = accentStyle.Render("> ")
			}
			fmt.Fprintf(&b, "%s%s %s - conflict\n", cursor, dashboardBadge("CONFLICT", toneWarn), entry.name)
		}
	}

	b.WriteString("\n")
	return b.String() + actionBarTargetSelect(m.includeWorkspace, onConflict, hasConflicts, !credentialBlocked && selectedTargets > 0, m.planErr != nil, credentialBlocked, selectedTargets == 0)
}

func actionBarTargetSelect(includeWs bool, onConflict bool, hasConflicts bool, canPlan bool, hasPlanErr bool, needsCredentials bool, noTargets bool) string {
	ws := "off"
	if includeWs {
		ws = "on"
	}
	if onConflict {
		return "[↑↓] navigate  [r] resolve conflict  [Esc] back  [q] quit"
	}
	if needsCredentials {
		return fmt.Sprintf("[↑↓] navigate  [Space] toggle  [i] workspace(%s)  [k] add credentials  [Esc] back  [q] quit\n%s", ws, guidanceFor(screenTargetSelect, footerMissingCredentials))
	}
	if hasConflicts {
		return fmt.Sprintf("[↑↓] navigate  [r] resolve conflicts  [Space] toggle  [i] workspace(%s)  [Esc] back  [q] quit", ws)
	}
	// DM-P69: don't advertise [Enter] plan when there's a plan error (recovery mode)
	if !canPlan || hasPlanErr {
		bar := fmt.Sprintf("[↑↓] navigate  [Space] toggle  [i] workspace(%s)  [Esc] back  [q] quit", ws)
		switch {
		case noTargets:
			return bar + "\n" + guidanceFor(screenTargetSelect, footerNoTargets)
		case hasPlanErr:
			return bar + "\n" + guidanceFor(screenTargetSelect, footerPlanError)
		default:
			return bar
		}
	}
	return fmt.Sprintf("[↑↓] navigate  [Space] toggle  [i] workspace(%s)  [Enter] plan  [Esc] back  [q] quit", ws)
}

func (m DashboardModel) renderConflictResolve() string {
	var b strings.Builder
	if m.resolveTarget == nil {
		b.WriteString(dashboardCallout(toneWarn, "No conflict selected", "Return to target selection and choose a conflict row."))
		return renderDashboardShell(b.String(), m.screen, m.width)
	}
	c := *m.resolveTarget
	b.WriteString(dashboardHeading("Resolve Conflict: "+c.Name, "Choose the config surface that should be managed for this client."))

	candidates := conflictCandidatesForDisplay(c)
	for i, cand := range candidates {
		title := fmt.Sprintf("[%d] %s", i+1, cand.Label)
		if cand.Deprecated {
			title += "  (deprecated)"
		}
		var card strings.Builder
		fmt.Fprintf(&card, "%s %s\n", mutedStyle.Render("Path     "), cand.Path)
		if cand.IsSymlink && cand.Resolved != "" {
			fmt.Fprintf(&card, "%s %s\n", mutedStyle.Render("Symlink  "), cand.Resolved)
		}
		if cand.ParseOK {
			fmt.Fprintf(&card, "%s %s\n", mutedStyle.Render("Parse    "), dashboardBadge("ok", toneOK))
		} else if cand.ParseError != "" {
			fmt.Fprintf(&card, "%s %s %s\n", mutedStyle.Render("Parse    "), dashboardBadge("error", toneError), redact.Key(cand.ParseError))
		}
		if len(cand.Providers) > 0 {
			fmt.Fprintf(&card, "%s %s\n", mutedStyle.Render("Providers"), strings.Join(cand.Providers, ", "))
		} else {
			fmt.Fprintf(&card, "%s %s\n", mutedStyle.Render("Providers"), "(none)")
		}
		b.WriteString(dashboardCard(title, strings.TrimRight(card.String(), "\n"), false))
		b.WriteString("\n")
	}
	if len(candidates) == 0 {
		b.WriteString(dashboardCallout(toneWarn, "No accessible candidates found", "Skip this client or inspect the config paths manually."))
	}

	bar := "[s] skip client  [Esc] cancel"
	if len(candidates) >= 1 {
		bar += "  [1] use this"
	}
	if len(candidates) >= 2 {
		bar += "  [2] use this"
	}
	b.WriteString(bar)
	return b.String()
}

func (m DashboardModel) renderPlanPreview() string {
	var b strings.Builder
	b.WriteString(dashboardHeading("Plan Preview", "Review the saved plan before any local config files are changed."))

	if m.applying {
		b.WriteString(dashboardCallout(toneInfo, guidanceFor(screenPlanPreview, footerApplying), "Writing config updates with backup and rollback safeguards."))
		return b.String() + "\n[q] quit"
	}

	if m.currentPlan != nil {
		b.WriteString(app.FormatSavedPlan(*m.currentPlan, time.Now().UTC()))
		b.WriteString("\n")
	}

	if m.planPreflight != nil && len(m.planPreflight.ApprovalPrompts) > 0 {
		b.WriteString(sectionTitleStyle.Render("Approvals required") + "\n")
		for _, prompt := range m.planPreflight.ApprovalPrompts {
			b.WriteString("  " + dashboardBadge("REVIEW", toneWarn) + " " + prompt.Message + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(mutedStyle.Render("Press [y/Enter] to apply or [n/Esc] to cancel") + "\n")
	return b.String()
}

func (m DashboardModel) renderApplyResult() string {
	var b strings.Builder
	b.WriteString(dashboardHeading("Apply Result", "Final write status and follow-up scan."))

	if m.applyErr != nil {
		b.WriteString(dashboardCallout(toneError, "Error", redact.Text(m.applyErr.Error())))
	} else if m.applyResult != nil {
		b.WriteString(app.FormatApplyResult(*m.applyResult))
	}

	if m.scanning {
		b.WriteString("\n" + dashboardCallout(toneInfo, "Rescanning...", "Refreshing detected clients after apply."))
	}

	if m.scanning {
		b.WriteString("\n[q] quit")
		return b.String()
	}

	b.WriteString("\n[r] rescan  [q] quit")
	return b.String()
}

func actionBarDoctor(hasErr bool, hasManager bool) string {
	if !hasManager {
		return "[r] rescan  [w] wizard  [?] help  [q] quit\n" + guidanceFor(screenDoctor, footerManagerMissing)
	}
	if hasErr {
		return "[r] rescan  [w] wizard  [?] help  [q] quit\n" + guidanceFor(screenDoctor, footerScanError)
	}
	return "[p/Enter] providers  [r] rescan  [w] wizard  [?] help  [q] quit"
}

func runtimeMetricTone(report doctor.Report) dashboardTone {
	for _, rt := range report.Runtimes {
		if !rt.Available {
			return toneError
		}
		if rt.Error != "" {
			return toneWarn
		}
	}
	return toneOK
}

func countTone(count int) dashboardTone {
	if count > 0 {
		return toneWarn
	}
	return toneOK
}

func selectedCountTone(count int) dashboardTone {
	if count > 0 {
		return toneOK
	}
	return toneWarn
}

func clientTone(client doctor.ClientFinding) dashboardTone {
	if !client.Installed {
		return toneNeutral
	}
	switch client.Confidence {
	case doctor.ConfidenceHigh, doctor.ConfidenceMedium:
		return toneOK
	case doctor.ConfidenceConflict:
		return toneWarn
	default:
		return toneNeutral
	}
}

func providerStateBadge(state ProviderState) string {
	switch state {
	case ProviderStateReady, ProviderStateNoKeyNeeded:
		return dashboardBadge(string(state), toneOK)
	case ProviderStateMissingCredential:
		return dashboardBadge(string(state), toneWarn)
	case ProviderStateRuntimeMissing, ProviderStateConflictBlocked:
		return dashboardBadge(string(state), toneError)
	default:
		return dashboardBadge(string(state), toneNeutral)
	}
}

func readinessCount(items []ProviderReadinessItem, state ProviderState) int {
	count := 0
	for _, item := range items {
		if item.State == state {
			count++
		}
	}
	return count
}

func readinessAttentionCount(items []ProviderReadinessItem) int {
	count := 0
	for _, item := range items {
		switch item.State {
		case ProviderStateMissingCredential, ProviderStateRuntimeMissing, ProviderStateConflictBlocked:
			count++
		}
	}
	return count
}

func selectableTargetCount(entries []targetEntry) int {
	count := 0
	for _, entry := range entries {
		if !entry.isConflict {
			count++
		}
	}
	return count
}

func conflictTargetCount(entries []targetEntry) int {
	count := 0
	for _, entry := range entries {
		if entry.isConflict {
			count++
		}
	}
	return count
}

func workspaceStateLabel(includeWorkspace bool) string {
	if includeWorkspace {
		return "on"
	}
	return "off"
}

func workspaceTone(includeWorkspace bool) dashboardTone {
	if includeWorkspace {
		return toneInfo
	}
	return toneNeutral
}

func targetScopeBadge(scope string) string {
	if scope == "" {
		scope = "user"
	}
	switch scope {
	case "global":
		return dashboardBadge("global", toneInfo)
	case "project", "workspace":
		return dashboardBadge(scope, toneWarn)
	default:
		return dashboardBadge(scope, toneNeutral)
	}
}
