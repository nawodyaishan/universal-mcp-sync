package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"github.com/nawodyaishan/universal-mcp-sync/pkg/app"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/config"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/provider"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/redact"
)

type stage int

const (
	stageSetup stage = iota
	stageAssignments
	stagePreview
	stageResults
)

type Model struct {
	ctx      *wizardContext
	stage    stage
	width    int
	showHelp bool
	recorder *SessionRecorder

	// Sub-models
	setupForm   *setupForm
	assignments assignmentModel
	preview     previewModel
	results     resultsModel
}

func NewModel(manager *app.Manager, initialKeys []string, initialRaw string) Model {
	return NewWizardModel(manager, initialKeys, initialRaw)
}

func NewWizardModel(manager *app.Manager, initialKeys []string, initialRaw string) Model {
	selected := make(map[config.AppID]bool, len(manager.Apps))
	for _, appConfig := range manager.Apps {
		if appConfig.ID == config.AppClaudeCode {
			selected[appConfig.ID] = true
		}
	}

	registry := provider.DefaultRegistry()

	ctx := &wizardContext{
		manager:     manager,
		registry:    registry,
		providerID:  "exa", // Default to Exa, will be updated by setup form
		isPreloaded: len(initialKeys) > 0,
		selected:    selected,
		assignments: app.DefaultAssignments(selected, len(initialKeys)),
	}

	// For backward compatibility, if keys were passed in, we seed them as profiles for Exa
	if ctx.isPreloaded {
		profiles := make([]provider.CredentialProfile, len(initialKeys))
		for i, key := range initialKeys {
			profiles[i] = provider.CredentialProfile{
				ProviderID: "exa",
				Values: map[string]string{
					"EXA_API_KEY": key,
				},
				Label: redact.Key(key),
			}
		}
		ctx.profiles = profiles
	}

	model := Model{
		ctx:         ctx,
		stage:       stageSetup,
		setupForm:   newSetupForm(ctx, initialRaw),
		assignments: assignmentModel{ctx: ctx},
		preview:     newPreviewModel(ctx),
		results:     resultsModel{ctx: ctx},
	}
	return model
}

func (m Model) WithRecorder(r *SessionRecorder) Model {
	m.recorder = r
	return m
}

func (m Model) Init() tea.Cmd {
	return m.setupForm.form.Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		recordWizardKey(m.recorder, msg, m.recordScreen(), m.recordBlockReason())
		switch msg.String() {
		case "ctrl+c":
			m.recordFinalAndClose()
			return m, tea.Quit
		case "q", "Q":
			if m.stage == stageSetup {
				m.recordFinalAndClose()
				return m, tea.Quit
			}
		case "esc":
			if m.stage == stageSetup {
				// Navigate to the previous field in the huh form (shift+tab).
				// When already on the first field, shift+tab is a no-op so esc
				// has no effect there; use ctrl+c to quit from the first field.
				prevKey := tea.KeyMsg{Type: tea.KeyShiftTab}
				_, cmd := m.setupForm.update(prevKey)
				return m, cmd
			}
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case nextMsg:
		m.stage++
		return m, nil
	case backMsg:
		if m.stage > 0 {
			m.stage--
			if m.stage == stageSetup {
				if !m.ctx.isPreloaded {
					m.ctx.profiles = nil
				}
				m.setupForm.form.State = huh.StateNormal
				m.setupForm.rebuildForm() // Ensure fields are fresh
				return m, m.setupForm.form.Init()
			}
		}
		return m, nil
	}

	var cmd tea.Cmd
	switch m.stage {
	case stageSetup:
		_, cmd = m.setupForm.update(msg)
		if m.setupForm.form.State == huh.StateCompleted {
			m.setupForm.syncToContext()
			m.ctx.assignments = app.DefaultAssignments(m.ctx.selected, len(m.ctx.profiles))
			m.stage = stageAssignments
		}
	case stageAssignments:
		next, c := m.assignments.Update(msg)
		if a, ok := next.(assignmentModel); ok {
			m.assignments = a
		}
		cmd = c
	case stagePreview:
		next, c := m.preview.Update(msg)
		if p, ok := next.(previewModel); ok {
			m.preview = p
		}
		cmd = c
	case stageResults:
		next, c := m.results.Update(msg)
		if r, ok := next.(resultsModel); ok {
			m.results = r
		}
		cmd = c
	}

	if cmd != nil && isTeaQuit(cmd) {
		m.recordFinalAndClose()
	}
	return m, cmd
}

func (m Model) View() string {
	if m.showHelp {
		return renderWizardShell(m.applyRecordingHeader(renderHelpOverlay()), m.stage, m.width)
	}

	view := ""
	switch m.stage {
	case stageSetup:
		view = renderWizardSetup(m.ctx, m.setupForm.form.View())
	case stageAssignments:
		view = m.assignments.View()
	case stagePreview:
		view = m.preview.View()
	case stageResults:
		view = m.results.View()
	}

	if m.ctx.err != nil {
		view += renderError(m.ctx.err)
	}
	return renderWizardShell(m.applyRecordingHeader(view), m.stage, m.width)
}

func (m Model) Err() error {
	return m.ctx.err
}

func (m Model) applyRecordingHeader(body string) string {
	if m.recorder == nil {
		return body
	}
	return "● rec  recording session to " + m.recorder.Path() + "\n\n" + body
}

func (m Model) recordScreen() string {
	switch m.stage {
	case stageSetup:
		return "WizardSetup"
	case stageAssignments:
		return "WizardAssignments"
	case stagePreview:
		return "WizardPreview"
	case stageResults:
		return "WizardResults"
	default:
		return "Wizard"
	}
}

func (m Model) recordBlockReason() string {
	if m.ctx != nil && m.ctx.err != nil {
		return "wizard error"
	}
	return ""
}

func (m Model) recordFinalAndClose() {
	if m.recorder == nil {
		return
	}
	m.recorder.Record(RecordEntry{Kind: "final", Screen: m.recordScreen(), PC: "wizard", BlockReason: m.recordBlockReason()})
	_ = m.recorder.Close()
}

func recordWizardKey(r *SessionRecorder, msg tea.KeyMsg, screen, blockReason string) {
	if r == nil {
		return
	}
	label := msg.String()
	if msg.Paste {
		label = "<paste>"
	} else if screen == "WizardSetup" && msg.Type == tea.KeyRunes {
		label = "<input>"
	}
	r.Record(RecordEntry{
		Kind:        "key",
		Key:         label,
		Screen:      screen,
		PC:          "wizard",
		BlockReason: blockReason,
	})
}
