package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type dashboardTone int

const (
	toneNeutral dashboardTone = iota
	toneInfo
	toneOK
	toneWarn
	toneError
)

type dashboardMetric struct {
	Label string
	Value string
	Tone  dashboardTone
}

var (
	badgeBaseStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1)
	badgeNeutralStyle = badgeBaseStyle.
				Foreground(colorMuted).
				Background(lipgloss.AdaptiveColor{Light: "254", Dark: "236"})
	badgeInfoStyle = badgeBaseStyle.
			Foreground(lipgloss.Color("16")).
			Background(colorBrand)
	badgeOKStyle = badgeBaseStyle.
			Foreground(lipgloss.Color("16")).
			Background(colorAccent)
	badgeWarnStyle = badgeBaseStyle.
			Foreground(lipgloss.Color("16")).
			Background(colorWarning)
	badgeErrorStyle = badgeBaseStyle.
			Foreground(lipgloss.Color("15")).
			Background(colorError)
	dashboardCardStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorPanelBorder).
				Padding(0, 1)
)

func dashboardContentWidth(width int) int {
	if width <= 0 {
		width = defaultViewWidth
	}
	contentWidth := width - 6
	if contentWidth < 64 {
		contentWidth = 64
	}
	if contentWidth > 104 {
		contentWidth = 104
	}
	return contentWidth
}

func renderDashboardShell(body string, screen dashboardScreen, width int) string {
	contentWidth := dashboardContentWidth(width)
	header := lipgloss.JoinHorizontal(
		lipgloss.Top,
		markStyle.Render("[mcp]"),
		mutedStyle.Render(" "),
		brandStyle.Render("Doctor Mode"),
		mutedStyle.Render("  /  scan, choose, apply"),
	)

	return shellStyle.Width(contentWidth).Render(
		header + "\n" +
			renderDashboardProgress(screen) + "\n\n" +
			body,
	)
}

func renderDashboardProgress(screen dashboardScreen) string {
	labels := []string{"Start", "Scan", "Provider", "Targets", "Apply"}
	current := dashboardProgressIndex(screen)
	parts := make([]string, len(labels))
	for i, label := range labels {
		switch {
		case i == current:
			parts[i] = activeStepStyle.Render(fmt.Sprintf("%d %s", i+1, label))
		case i < current:
			parts[i] = doneStepStyle.Render(fmt.Sprintf("✓ %s", label))
		default:
			parts[i] = stepStyle.Render(fmt.Sprintf("%d %s", i+1, label))
		}
	}
	return strings.Join(parts, mutedStyle.Render(" -> "))
}

func dashboardProgressIndex(screen dashboardScreen) int {
	switch screen {
	case screenWelcome:
		return 0
	case screenDoctor:
		return 1
	case screenProviderReady:
		return 2
	case screenTargetSelect, screenConflictResolve, screenCredentialEntry:
		return 3
	case screenPlanPreview, screenApplyResult:
		return 4
	default:
		return 1
	}
}

func dashboardHeading(title, subtitle string) string {
	var b strings.Builder
	b.WriteString(sectionTitleStyle.Render(title))
	if subtitle != "" {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render(subtitle))
	}
	b.WriteString("\n\n")
	return b.String()
}

func dashboardMetrics(metrics ...dashboardMetric) string {
	if len(metrics) == 0 {
		return ""
	}
	parts := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		label := mutedStyle.Render(metric.Label)
		parts = append(parts, label+" "+dashboardBadge(metric.Value, metric.Tone))
	}
	return strings.Join(parts, mutedStyle.Render("   ")) + "\n\n"
}

func dashboardBadge(label string, tone dashboardTone) string {
	switch tone {
	case toneInfo:
		return badgeInfoStyle.Render(label)
	case toneOK:
		return badgeOKStyle.Render(label)
	case toneWarn:
		return badgeWarnStyle.Render(label)
	case toneError:
		return badgeErrorStyle.Render(label)
	default:
		return badgeNeutralStyle.Render(label)
	}
}

func dashboardCallout(tone dashboardTone, title string, lines ...string) string {
	var b strings.Builder
	b.WriteString(dashboardBadge(title, tone))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		b.WriteString("\n  ")
		b.WriteString(line)
	}
	b.WriteString("\n\n")
	return b.String()
}

func dashboardCard(title, body string, active bool) string {
	prefix := "  "
	if active {
		prefix = "> "
	}
	content := title
	if body != "" {
		content += "\n" + mutedStyle.Render(body)
	}
	style := dashboardCardStyle
	if active {
		style = style.BorderForeground(colorAccent)
	}
	lines := strings.Split(style.Render(content), "\n")
	for i, line := range lines {
		if active && i == 0 {
			lines[i] = accentStyle.Render(prefix) + line
			continue
		}
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
}
