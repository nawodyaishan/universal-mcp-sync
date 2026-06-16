package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/config"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/provider"
)

const defaultViewWidth = 88

var (
	colorBorder      = lipgloss.AdaptiveColor{Light: "57", Dark: "99"}
	colorPanelBorder = lipgloss.AdaptiveColor{Light: "55", Dark: "62"}
	colorAccent      = lipgloss.AdaptiveColor{Light: "28", Dark: "114"}
	colorBrand       = lipgloss.AdaptiveColor{Light: "136", Dark: "229"}
	colorMuted       = lipgloss.AdaptiveColor{Light: "240", Dark: "244"}
	colorStep        = lipgloss.AdaptiveColor{Light: "241", Dark: "245"}
	colorSection     = lipgloss.AdaptiveColor{Light: "93", Dark: "183"}
	colorError       = lipgloss.AdaptiveColor{Light: "160", Dark: "203"}
	colorWarning     = lipgloss.AdaptiveColor{Light: "166", Dark: "214"}
	colorDim         = lipgloss.AdaptiveColor{Light: "246", Dark: "240"}

	shellStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(colorBorder)
	panelStyle = lipgloss.NewStyle().
			PaddingLeft(1).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(colorPanelBorder)
	markStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)
	brandStyle = lipgloss.NewStyle().
			Foreground(colorBrand).
			Bold(true)
	accentStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)
	mutedStyle = lipgloss.NewStyle().
			Foreground(colorMuted)
	stepStyle = lipgloss.NewStyle().
			Foreground(colorStep).
			Padding(0, 1)
	activeStepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("16")).
			Background(colorAccent).
			Bold(true).
			Padding(0, 1)
	doneStepStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Padding(0, 1)
	sectionTitleStyle = lipgloss.NewStyle().
				Foreground(colorSection).
				Bold(true)
	errorStyle = lipgloss.NewStyle().
			Foreground(colorError).
			Bold(true)
	dimStyle = lipgloss.NewStyle().
			Foreground(colorDim)
)

func selectedAppIDs(apps []config.AppConfig, selected map[config.AppID]bool) []config.AppID {
	ids := make([]config.AppID, 0, len(apps))
	for _, appConfig := range apps {
		if selected[appConfig.ID] {
			ids = append(ids, appConfig.ID)
		}
	}
	return ids
}

func assignmentLabel(profiles []provider.CredentialProfile, index int) string {
	if index < 0 || index >= len(profiles) {
		return "unassigned"
	}
	return profiles[index].Label
}

func renderError(err error) string {
	if err == nil {
		return ""
	}
	return "\n" + errorStyle.Render("Error: "+err.Error()) + "\n"
}

func renderShell(body string, current stage, width int) string {
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

	header := lipgloss.JoinHorizontal(
		lipgloss.Top,
		markStyle.Render("[<>]"),
		mutedStyle.Render(" "),
		brandStyle.Render("MCP Config"),
		mutedStyle.Render("  @"),
		accentStyle.Render("nawodyaishan"),
		mutedStyle.Render("  /  local config sync"),
	)

	return shellStyle.Width(contentWidth).Render(
		header + "\n" +
			renderStageBar(current) + "\n\n" +
			body,
	)
}

func renderStageBar(current stage) string {
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

func renderSection(title, body, help string) string {
	parts := []string{sectionTitleStyle.Render(title), panelStyle.Render(strings.TrimRight(body, "\n"))}
	if help != "" {
		parts = append(parts, mutedStyle.Render(help))
	}
	return strings.Join(parts, "\n\n")
}

func renderKeyHelp(items ...string) string {
	return fmt.Sprintf("[%s]", strings.Join(items, "] ["))
}

func renderHelpOverlay() string {
	rows := [][2]string{
		{"Navigation", ""},
		{"↑ / k", "move up"},
		{"↓ / j", "move down"},
		{"← / h", "previous credential"},
		{"→ / l", "next credential"},
		{"enter", "confirm / advance"},
		{"esc / b", "go back"},
		{"ctrl+c", "quit"},
		{"", ""},
		{"Global", ""},
		{"?", "toggle this help"},
	}

	keyCol := lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Width(16)
	descCol := lipgloss.NewStyle().Foreground(colorMuted)
	headCol := lipgloss.NewStyle().Foreground(colorSection).Bold(true)

	var sb strings.Builder
	for _, row := range rows {
		if row[1] == "" {
			if row[0] == "" {
				sb.WriteString("\n")
			} else {
				sb.WriteString(headCol.Render(row[0]) + "\n")
			}
			continue
		}
		sb.WriteString(keyCol.Render(row[0]) + descCol.Render(row[1]) + "\n")
	}

	return panelStyle.Render(
		sectionTitleStyle.Render("Keyboard Shortcuts") + "\n\n" + strings.TrimRight(sb.String(), "\n"),
	)
}

func renderDashboardHelpOverlay(screen dashboardScreen) string {
	title := "Help - System Status"
	var rows [][2]string

	switch screen {
	case screenWelcome:
		title = "Help - Welcome"
		rows = [][2]string{
			{"Actions", ""},
			{"↑", "choose the previous mode"},
			{"↓", "choose the next mode"},
			{"enter", "start the selected mode"},
			{"d", "start doctor mode"},
			{"w", "start wizard mode"},
			{"", ""},
			{"Global", ""},
			{"?", "close help"},
			{"q / ctrl+c", "quit"},
		}
	case screenProviderReady:
		title = "Help - Provider Readiness"
		rows = [][2]string{
			{"Navigation", ""},
			{"↑ / k", "move up"},
			{"↓ / j", "move down"},
			{"enter", "select and validate provider"},
			{"v", "run live validation"},
			{"r", "resolve conflicts when shown"},
			{"esc", "back to system status"},
			{"", ""},
			{"Global", ""},
			{"?", "close help"},
			{"q / ctrl+c", "quit"},
		}
	case screenTargetSelect:
		title = "Help - Select Targets"
		rows = [][2]string{
			{"Navigation", ""},
			{"↑ / k", "move up"},
			{"↓ / j", "move down"},
			{"space", "toggle selected target"},
			{"i", "toggle workspace targets"},
			{"enter", "build plan"},
			{"r", "open conflict resolution"},
			{"esc", "back to provider readiness"},
			{"", ""},
			{"Global", ""},
			{"?", "close help"},
			{"q / ctrl+c", "quit"},
		}
	case screenCredentialEntry:
		title = "Help - Add Credentials"
		rows = [][2]string{
			{"Actions", ""},
			{"enter", "submit credentials"},
			{"tab", "next field"},
			{"shift+tab", "previous field"},
			{"backspace", "delete last character"},
			{"esc", "cancel and go back"},
			{"", ""},
			{"Global", ""},
			{"?", "close help"},
			{"q / ctrl+c", "quit"},
		}
	case screenConflictResolve:
		title = "Help - Resolve Conflict"
		rows = [][2]string{
			{"Actions", ""},
			{"1", "choose the first candidate"},
			{"2", "choose the second candidate"},
			{"s", "skip this client"},
			{"esc", "cancel and go back"},
			{"", ""},
			{"Global", ""},
			{"?", "close help"},
			{"q / ctrl+c", "quit"},
		}
	case screenPlanPreview:
		title = "Help - Plan Preview"
		rows = [][2]string{
			{"Actions", ""},
			{"y / enter", "apply the plan"},
			{"n / esc", "back to target selection"},
			{"", ""},
			{"Global", ""},
			{"?", "close help"},
			{"q / ctrl+c", "quit"},
		}
	case screenApplyResult:
		title = "Help - Apply Result"
		rows = [][2]string{
			{"Actions", ""},
			{"r", "rescan after the result"},
			{"", ""},
			{"Global", ""},
			{"?", "close help"},
			{"q / ctrl+c", "quit"},
		}
	default:
		rows = [][2]string{
			{"Actions", ""},
			{"p / enter", "open provider readiness"},
			{"r", "rescan clients and runtimes"},
			{"w", "route to the wizard"},
			{"", ""},
			{"Global", ""},
			{"?", "close help"},
			{"q / ctrl+c", "quit"},
		}
	}

	keyCol := lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Width(16)
	descCol := lipgloss.NewStyle().Foreground(colorMuted)
	headCol := lipgloss.NewStyle().Foreground(colorSection).Bold(true)

	var sb strings.Builder
	for _, row := range rows {
		if row[1] == "" {
			if row[0] == "" {
				sb.WriteString("\n")
			} else {
				sb.WriteString(headCol.Render(row[0]) + "\n")
			}
			continue
		}
		sb.WriteString(keyCol.Render(row[0]) + descCol.Render(row[1]) + "\n")
	}

	return panelStyle.Render(
		sectionTitleStyle.Render(title) + "\n\n" + strings.TrimRight(sb.String(), "\n"),
	)
}
