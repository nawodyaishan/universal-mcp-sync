package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/config"
)

type assignmentModel struct {
	ctx    *wizardContext
	cursor int
}

func (m assignmentModel) Init() tea.Cmd {
	return nil
}

func (m assignmentModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	selectedApps := selectedAppIDs(m.ctx.manager.Apps, m.ctx.selected)
	switch km.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(selectedApps)-1 {
			m.cursor++
		}
	case "left", "h":
		m.rotateAssignment(selectedApps, -1)
	case "right", "l":
		m.rotateAssignment(selectedApps, 1)
	case "enter":
		if len(selectedApps) == 0 {
			m.ctx.err = fmt.Errorf("select at least one target app")
			return m, signalBack
		}
		plan, err := m.ctx.manager.PrepareProvider(m.ctx.provider, m.ctx.profiles, m.ctx.selected, m.ctx.assignments)
		if err != nil {
			m.ctx.err = err
			return m, nil
		}
		m.ctx.err = nil
		m.ctx.plan = plan
		return m, signalNext
	case "b", "esc":
		return m, signalBack
	}
	return m, nil
}

func (m assignmentModel) View() string {
	var builder strings.Builder
	selectedApps := selectedAppIDs(m.ctx.manager.Apps, m.ctx.selected)
	builder.WriteString(dashboardHeading("Assign Credentials", "Map credential profiles to each selected target app."))

	if len(selectedApps) == 0 {
		builder.WriteString(dashboardCallout(toneWarn, "No target apps selected", "Go back to Setup and choose at least one target app."))
		builder.WriteString(renderKeyHelp("esc back"))
		return builder.String()
	}

	profileCount := len(m.ctx.profiles)
	builder.WriteString(dashboardMetrics(
		dashboardMetric{Label: "targets", Value: fmt.Sprintf("%d", len(selectedApps)), Tone: toneInfo},
		dashboardMetric{Label: "profiles", Value: fmt.Sprintf("%d", profileCount), Tone: wizardPositiveTone(profileCount)},
		dashboardMetric{Label: "provider", Value: wizardProviderName(m.ctx), Tone: toneOK},
	))

	if profileCount <= 1 {
		builder.WriteString(dashboardCallout(toneInfo, "Single profile assignment", "All selected target apps will use the same credential profile."))
	} else {
		builder.WriteString(dashboardCallout(toneInfo, "Profile routing", "Use left and right to rotate the credential profile for the highlighted target."))
	}

	builder.WriteString(sectionTitleStyle.Render("Distribute Credentials"))
	builder.WriteString("\n")
	for i, appID := range selectedApps {
		label := assignmentLabel(m.ctx.profiles, m.ctx.assignments[appID])
		title := fmt.Sprintf("%s %s", dashboardBadge("TARGET", toneInfo), config.AppName(appID))
		body := fmt.Sprintf("Credential profile  %s", label)
		builder.WriteString(dashboardCard(title, body, m.cursor == i))
		builder.WriteString("\n")
	}
	hints := []string{"up/down move"}
	if len(m.ctx.profiles) > 1 {
		hints = append(hints, "left/right change")
	}
	hints = append(hints, "enter preview", "esc back")
	builder.WriteString("\n")
	builder.WriteString(renderKeyHelp(hints...))
	return builder.String()
}

func (m *assignmentModel) rotateAssignment(selectedApps []config.AppID, delta int) {
	if len(m.ctx.profiles) <= 1 || len(selectedApps) == 0 {
		return
	}
	appID := selectedApps[m.cursor]
	current := m.ctx.assignments[appID]
	next := current + delta
	if next < 0 {
		next = len(m.ctx.profiles) - 1
	}
	if next >= len(m.ctx.profiles) {
		next = 0
	}
	m.ctx.assignments[appID] = next
}
