package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderWizardShell(body string, current stage, width int) string {
	contentWidth := dashboardContentWidth(width)
	header := lipgloss.JoinHorizontal(
		lipgloss.Top,
		markStyle.Render("[mcp]"),
		mutedStyle.Render(" "),
		brandStyle.Render("Wizard Mode"),
		mutedStyle.Render("  /  guided setup"),
	)

	return shellStyle.Width(contentWidth).Render(
		header + "\n" +
			renderWizardProgress(current) + "\n\n" +
			body,
	)
}

func renderWizardProgress(current stage) string {
	labels := []string{"Setup", "Assign", "Preview", "Results"}
	parts := make([]string, len(labels))
	for i, label := range labels {
		switch {
		case stage(i) == current:
			parts[i] = activeStepStyle.Render(fmt.Sprintf("%d %s", i+1, label))
		case stage(i) < current:
			parts[i] = doneStepStyle.Render(fmt.Sprintf("✓ %s", label))
		default:
			parts[i] = stepStyle.Render(fmt.Sprintf("%d %s", i+1, label))
		}
	}
	return strings.Join(parts, mutedStyle.Render(" -> "))
}

func renderWizardSetup(ctx *wizardContext, formView string) string {
	var b strings.Builder
	b.WriteString(dashboardHeading("Setup", "Choose provider, target apps, and credentials for the guided apply flow."))

	providerCount := len(ctx.registry.All())
	targetCount := 0
	if ctx.manager != nil {
		targetCount = len(ctx.manager.Apps)
	}
	credentialMode := "manual"
	credentialTone := toneInfo
	if ctx.isPreloaded {
		credentialMode = "preloaded"
		credentialTone = toneOK
	}
	b.WriteString(dashboardMetrics(
		dashboardMetric{Label: "providers", Value: fmt.Sprintf("%d", providerCount), Tone: toneInfo},
		dashboardMetric{Label: "target apps", Value: fmt.Sprintf("%d", targetCount), Tone: toneInfo},
		dashboardMetric{Label: "credentials", Value: credentialMode, Tone: credentialTone},
	))

	if ctx.isPreloaded {
		profileLabel := "profiles"
		if len(ctx.profiles) == 1 {
			profileLabel = "profile"
		}
		b.WriteString(dashboardCallout(toneOK, "Credentials loaded", fmt.Sprintf("%d credential %s available from CLI input.", len(ctx.profiles), profileLabel)))
	}

	b.WriteString(strings.TrimRight(formView, "\n"))
	return b.String()
}

func wizardProviderName(ctx *wizardContext) string {
	if ctx == nil {
		return "unknown"
	}
	if ctx.provider != nil {
		return ctx.provider.Name()
	}
	if prov, ok := ctx.registry.Get(ctx.providerID); ok {
		return prov.Name()
	}
	if ctx.providerID != "" {
		return ctx.providerID
	}
	return "unknown"
}

func wizardPositiveTone(count int) dashboardTone {
	if count > 0 {
		return toneOK
	}
	return toneWarn
}
