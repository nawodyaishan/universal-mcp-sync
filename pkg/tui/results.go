package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/app"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/redact"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/verify"
)

type resultsModel struct {
	ctx *wizardContext
}

func (m resultsModel) Init() tea.Cmd {
	return nil
}

func (m resultsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "enter":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m resultsModel) View() string {
	return renderApplyResults(m.ctx.result) + "\n\n" +
		renderKeyHelp("enter finish", "q quit", "ctrl+c quit")
}

func renderApplyResults(result app.ApplyResult) string {
	var builder strings.Builder
	builder.WriteString(dashboardHeading("Results", "Apply summary, backups, and verification feedback."))

	// Summary header
	if len(result.UpdatedTargets) > 0 {
		builder.WriteString(dashboardCallout(toneOK, fmt.Sprintf("✓ Successfully updated %d targets", len(result.UpdatedTargets)), "Restart affected AI agents after this session."))
	} else {
		builder.WriteString(dashboardCallout(toneWarn, "! No changes were applied", "No target config files needed an update."))
	}

	builder.WriteString(dashboardMetrics(
		dashboardMetric{Label: "updated", Value: fmt.Sprintf("%d", len(result.UpdatedTargets)), Tone: wizardPositiveTone(len(result.UpdatedTargets))},
		dashboardMetric{Label: "skipped", Value: fmt.Sprintf("%d", len(result.SkippedTargets)), Tone: countTone(len(result.SkippedTargets))},
		dashboardMetric{Label: "backups", Value: fmt.Sprintf("%d", len(result.BackupPaths)), Tone: toneInfo},
	))

	if len(result.UpdatedTargets) > 0 {
		builder.WriteString(sectionTitleStyle.Render("Updated Targets"))
		builder.WriteString("\n")
		for _, target := range result.UpdatedTargets {
			fmt.Fprintf(&builder, "  %s %s\n", dashboardBadge("UPDATED", toneOK), target)
		}
		builder.WriteString("\n")
	}

	if len(result.SkippedTargets) > 0 {
		builder.WriteString(sectionTitleStyle.Render("Skipped Targets"))
		builder.WriteString("\n")
		for _, target := range result.SkippedTargets {
			fmt.Fprintf(&builder, "  %s %s\n", dashboardBadge("SKIP", toneNeutral), target)
		}
		builder.WriteString("\n")
	}

	if len(result.Warnings) > 0 {
		builder.WriteString(sectionTitleStyle.Render("Warnings"))
		builder.WriteString("\n")
		for _, warning := range result.Warnings {
			fmt.Fprintf(&builder, "  %s %s\n", dashboardBadge("WARN", toneWarn), redact.Text(warning))
		}
		builder.WriteString("\n")
	}

	// Backups section
	if len(result.BackupPaths) > 0 {
		builder.WriteString(sectionTitleStyle.Render("Backups Created"))
		builder.WriteString("\n")
		for _, path := range result.BackupPaths {
			fmt.Fprintf(&builder, "  %s %s\n", dashboardBadge("BACKUP", toneInfo), dimStyle.Render(path))
		}
		builder.WriteString("\n")
	}

	// Verification section
	if len(result.Verification) > 0 {
		builder.WriteString(sectionTitleStyle.Render("Verification"))
		builder.WriteString("\n")
		for _, item := range result.Verification {
			status := dashboardBadge("?", toneNeutral)
			switch item.Status {
			case verify.StatusOK:
				status = dashboardBadge("OK", toneOK)
			case verify.StatusWarning:
				status = dashboardBadge("WARN", toneWarn)
			case verify.StatusFailed:
				status = dashboardBadge("FAIL", toneError)
			case verify.StatusSkipped:
				status = dashboardBadge("SKIP", toneNeutral)
			}

			fmt.Fprintf(&builder, "  %s %s\n", status, item.Target)
			for _, detail := range item.Details {
				fmt.Fprintf(&builder, "      %s\n", dimStyle.Render(detail))
			}
		}
		builder.WriteString("\n")
	}

	if len(result.RolledBack) > 0 {
		builder.WriteString(sectionTitleStyle.Render("Rolled Back"))
		builder.WriteString("\n")
		for _, path := range result.RolledBack {
			fmt.Fprintf(&builder, "  %s %s\n", dashboardBadge("ROLLBACK", toneWarn), path)
		}
		builder.WriteString("\n")
	}

	if len(result.RollbackFailed) > 0 {
		builder.WriteString(sectionTitleStyle.Render("Rollback Failed"))
		builder.WriteString("\n")
		for _, path := range result.RollbackFailed {
			fmt.Fprintf(&builder, "  %s %s\n", dashboardBadge("FAILED", toneError), path)
		}
		builder.WriteString("\n")
	}

	// Final call to action
	builder.WriteString(dashboardCallout(toneInfo, "Next Steps", "Restart the affected AI agents to reload the new configuration."))

	return builder.String()
}
