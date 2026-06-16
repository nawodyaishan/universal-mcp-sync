package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/app"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/provider"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/redact"
)

type previewModel struct {
	ctx      *wizardContext
	spinner  spinner.Model
	applying bool
}

type applyResultMsg struct {
	result app.ApplyResult
	err    error
}

func newPreviewModel(ctx *wizardContext) previewModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(colorAccent)
	return previewModel{ctx: ctx, spinner: s}
}

func runApplyCmd(ctx *wizardContext) tea.Cmd {
	return func() tea.Msg {
		result, err := ctx.manager.Apply(ctx.plan)
		return applyResultMsg{result: result, err: err}
	}
}

func (m previewModel) Init() tea.Cmd {
	return nil
}

func (m previewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.applying {
			return m, nil
		}
		switch msg.String() {
		case "b", "esc":
			return m, signalBack
		case "enter":
			m.applying = true
			return m, tea.Batch(runApplyCmd(m.ctx), m.spinner.Tick)
		}
	case applyResultMsg:
		m.applying = false
		if msg.err != nil {
			m.ctx.err = msg.err
			return m, nil
		}
		m.ctx.err = nil
		m.ctx.result = msg.result
		return m, signalNext
	case spinner.TickMsg:
		if m.applying {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m previewModel) View() string {
	if m.applying {
		var builder strings.Builder
		builder.WriteString(dashboardHeading("Applying", "Writing local config changes and verifying the result."))
		builder.WriteString(dashboardCallout(toneInfo, "In progress", m.spinner.View()+" Applying configuration…"))
		builder.WriteString(renderKeyHelp("ctrl+c quit"))
		return builder.String()
	}
	return renderPreviewPlan(m.ctx.plan, m.ctx.manager.HomeDir) + "\n\n" +
		renderKeyHelp("enter apply", "esc back", "? help", "ctrl+c quit")
}

func signalNext() tea.Msg { return nextMsg{} }
func signalBack() tea.Msg { return backMsg{} }

type nextMsg struct{}
type backMsg struct{}

func renderPreviewPlan(plan app.ExecutionPlan, homeDir string) string {
	var builder strings.Builder
	builder.WriteString(dashboardHeading("Preview Plan", "Review the exact target operations before anything is written."))

	if len(plan.Operations) == 0 {
		builder.WriteString(dashboardCallout(toneWarn, "No targets selected", "Go back to Setup and choose at least one target app."))
		return strings.TrimSpace(builder.String())
	}

	targetLabel := "targets"
	if len(plan.Operations) == 1 {
		targetLabel = "target"
	}

	first := plan.Operations[0]
	builder.WriteString(dashboardCallout(toneOK, "Ready to apply MCP configuration", "Backups are created before file writes.", "Credentials stay redacted in the preview."))
	builder.WriteString(dashboardMetrics(
		dashboardMetric{Label: "Targets", Value: fmt.Sprintf("%d %s", len(plan.Operations), targetLabel), Tone: toneInfo},
		dashboardMetric{Label: "Provider", Value: first.ProviderID, Tone: toneOK},
		dashboardMetric{Label: "Mode", Value: string(first.Config.Type), Tone: toneNeutral},
	))

	if len(plan.Warnings) > 0 {
		builder.WriteString(sectionTitleStyle.Render("Warnings"))
		builder.WriteString("\n")
		for _, warning := range plan.Warnings {
			fmt.Fprintf(&builder, "  %s %s\n", dashboardBadge("WARN", toneWarn), redact.Text(warning))
		}
		builder.WriteString("\n")
	}

	builder.WriteString(sectionTitleStyle.Render("Targets"))
	builder.WriteString("\n")
	for index, op := range plan.Operations {
		builder.WriteString(renderPreviewOperation(index, op, homeDir))
		builder.WriteString("\n")
	}

	return strings.TrimSpace(builder.String())
}

func renderPreviewOperation(index int, op app.Operation, homeDir string) string {
	status := dashboardBadge("UPDATE", toneInfo)
	if op.SkipReason != "" {
		status = dashboardBadge("SKIP", toneNeutral)
	} else if op.WillCreate {
		status = dashboardBadge("CREATE", toneOK)
	} else if op.Path == "" {
		status = dashboardBadge("COMMAND", toneInfo)
	}

	var body strings.Builder
	fmt.Fprintf(&body, "Config     %s\n", op.FileLabel)
	fmt.Fprintf(&body, "Transport  %s\n", previewTransportLabel(op.Config))
	fmt.Fprintf(&body, "Key        %s\n", op.CredentialLabel)

	if op.SkipReason != "" {
		fmt.Fprintf(&body, "Reason     %s", redact.Text(op.SkipReason))
		return dashboardCard(fmt.Sprintf("%s %d. %s", status, index+1, op.AppName), strings.TrimRight(body.String(), "\n"), false)
	}

	if op.Path == "" {
		fmt.Fprintf(&body, "Action     update through %s command", op.AppName)
		return dashboardCard(fmt.Sprintf("%s %d. %s", status, index+1, op.AppName), strings.TrimRight(body.String(), "\n"), false)
	}

	action := "update existing file"
	if op.WillCreate {
		action = "create new file"
	}
	fmt.Fprintf(&body, "Action     %s\n", action)
	fmt.Fprintf(&body, "Path       %s\n", shortenHomePath(op.Path, homeDir))
	if op.WillCreate {
		body.WriteString("Backup     not needed for new file")
	} else if op.BackupPath != "" {
		fmt.Fprintf(&body, "Backup     %s", shortenHomePath(op.BackupPath, homeDir))
	}
	if op.GitWarning {
		body.WriteString("\n")
		body.WriteString("Warning    target path may be shared through source control")
	}
	return dashboardCard(fmt.Sprintf("%s %d. %s", status, index+1, op.AppName), strings.TrimRight(body.String(), "\n"), false)
}

func previewTransportLabel(cfg provider.MCPConfig) string {
	if cfg.Type == provider.TransportStdio {
		if cfg.Command == "" {
			return "stdio command"
		}
		return fmt.Sprintf("stdio command %s", cfg.Command)
	}
	return string(cfg.Type)
}

func shortenHomePath(path, homeDir string) string {
	if path == "" || homeDir == "" {
		return path
	}
	cleanPath := filepath.Clean(path)
	cleanHome := filepath.Clean(homeDir)
	if cleanPath == cleanHome {
		return "~"
	}
	prefix := cleanHome + string(filepath.Separator)
	if strings.HasPrefix(cleanPath, prefix) {
		return "~" + strings.TrimPrefix(cleanPath, cleanHome)
	}
	return path
}
