package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/app"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/config"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/doctor"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/manifest"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/provider"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/validate"
)

// DashboardScanner abstracts the doctor scan so the TUI can be tested without filesystem access.
type DashboardScanner interface {
	Scan(ctx context.Context) (doctor.Report, error)
}

// DashboardManager abstracts plan/apply/validate operations for unit testing.
type DashboardManager interface {
	PrepareProvider(
		prov provider.MCPProvider,
		profiles []provider.CredentialProfile,
		selected map[config.AppID]bool,
		assign map[config.AppID]int,
	) (app.ExecutionPlan, error)
	PrepareProviderWithTargetPaths(
		prov provider.MCPProvider,
		profiles []provider.CredentialProfile,
		selected map[config.AppID]bool,
		assign map[config.AppID]int,
		targetPaths app.TargetPathOverrides,
	) (app.ExecutionPlan, error)
	PrepareProviderWithTargetFiles(
		prov provider.MCPProvider,
		profiles []provider.CredentialProfile,
		selected map[config.AppID]bool,
		assign map[config.AppID]int,
		targetFiles app.TargetFileOverrides,
	) (app.ExecutionPlan, error)
	BuildSavedPlan(plan app.ExecutionPlan, opts app.SavedPlanOptions) (app.SavedPlan, error)
	PreflightSavedPlan(plan app.SavedPlan, opts app.SavedPlanApplyOptions) (app.SavedPlanPreflight, error)
	ApplySavedPlan(plan app.SavedPlan, opts app.SavedPlanApplyOptions) (app.ApplyResult, error)
	Validate(ctx context.Context, prov provider.MCPProvider, profiles []provider.CredentialProfile, live bool) (validate.Report, error)
	HomeDir() string
}

// ProductionScanner wraps the real pkg/doctor.
type ProductionScanner struct {
	Options doctor.Options
}

func NewProductionScanner(homeDir, workspaceDir string) *ProductionScanner {
	return &ProductionScanner{
		Options: doctor.Options{
			HomeDir:       homeDir,
			WorkspaceDir:  workspaceDir,
			CheckRuntimes: true,
		},
	}
}

func (s *ProductionScanner) Scan(ctx context.Context) (doctor.Report, error) {
	doc, err := doctor.New(s.Options)
	if err != nil {
		return doctor.Report{}, err
	}
	return doc.Scan(ctx)
}

// dashboardScreen is the active screen in the dashboard state machine.
type dashboardScreen int

const (
	screenWelcome         dashboardScreen = iota // launch choice between doctor and wizard
	screenDoctor                                 // Phase 7 base
	screenProviderReady                          // provider readiness list
	screenTargetSelect                           // client/target selection
	screenPlanPreview                            // plan preview + approval prompts
	screenApplyResult                            // apply result
	screenConflictResolve                        // conflict resolution overlay
	screenCredentialEntry                        // in-flow credential entry overlay
)

// ConflictResolution records a user's session-scoped choice for a conflict client.
type ConflictResolution struct {
	ChosenPath  string // candidate path selected; empty if Skipped
	ChosenLabel string // candidate label selected; empty if Skipped
	Skipped     bool   // true when user chose to exclude this client
}

// targetEntry is a unified list item for screenTargetSelect rendering.
type targetEntry struct {
	id         string
	clientID   manifest.ClientID
	name       string
	label      string
	path       string
	scope      manifest.ScopeKind
	kind       config.FileKind
	exists     bool
	creatable  bool
	gitWarning bool
	isConflict bool // true = unresolved conflict; cursor can reach but not toggle
}

// Typed messages used by Phase 8 commands.
type scanResultMsg struct {
	report doctor.Report
	err    error
}
type providerReadinessMsg struct {
	items []ProviderReadinessItem
	err   error
}
type validationResultMsg struct {
	report validate.Report
	live   bool
	err    error
}
type planCreatedMsg struct {
	plan app.SavedPlan
	path string
	err  error
}
type preflightResultMsg struct {
	preflight app.SavedPlanPreflight
	err       error
}
type dashApplyResultMsg struct {
	result app.ApplyResult
	err    error
}

// DashboardModel is the TUI state for the doctor dashboard (Phase 7 base + Phase 8 extensions).
type DashboardModel struct {
	// Phase 7 fields
	scanner        DashboardScanner
	report         doctor.Report
	err            error
	scanning       bool
	RouteToWizard  bool
	placeholderMsg string
	width          int
	showHelp       bool

	// Phase 8 fields
	manager          DashboardManager
	profiles         []provider.CredentialProfile
	screen           dashboardScreen
	welcomeCursor    int
	providerCursor   int
	readiness        []ProviderReadinessItem
	readinessErr     error
	computingReady   bool
	validating       bool
	validReport      *validate.Report
	validErr         error
	selectedProv     int
	clientCursor     int
	selectedClients  map[manifest.ClientID]bool
	selectedTargets  map[string]bool
	includeWorkspace bool
	planning         bool
	currentPlan      *app.SavedPlan
	planPath         string
	preflighting     bool
	planPreflight    *app.SavedPlanPreflight
	planErr          error
	applying         bool
	applyResult      *app.ApplyResult
	applyErr         error

	// Phase 12 — conflict resolution
	resolveTarget     *doctor.ClientFinding // client being resolved in overlay
	resolvedConflicts map[manifest.ClientID]ConflictResolution

	// Phase 14 — in-memory credential entry
	credEntry    *credentialEntryState
	credReturnTo dashboardScreen

	// Phase 14 PR 14g — optional session recorder. When non-nil, every
	// tea.KeyMsg is appended to the transcript before dispatch.
	recorder *SessionRecorder

	height int // terminal height, set from tea.WindowSizeMsg
}

// WithRecorder attaches a session recorder to the model. The recorder is a
// pointer so subsequent screen transitions in copied models still write to
// the same file.
func (m DashboardModel) WithRecorder(r *SessionRecorder) DashboardModel {
	m.recorder = r
	return m
}

// NewDashboardModel creates a new dashboard.
// manager and profiles may be nil; when manager is nil Phase 8 screens are unreachable.
func NewDashboardModel(scanner DashboardScanner, manager DashboardManager, profiles []provider.CredentialProfile) DashboardModel {
	return DashboardModel{
		scanner:  scanner,
		scanning: true,
		manager:  manager,
		profiles: profiles,
		screen:   screenDoctor,
	}
}

// NewDashboardModelWithWelcome creates a dashboard that first asks the user to
// choose Doctor Mode or Wizard Mode. The scan starts only after Doctor Mode is
// selected so the first visible screen is the mode choice.
func NewDashboardModelWithWelcome(scanner DashboardScanner, manager DashboardManager, profiles []provider.CredentialProfile) DashboardModel {
	m := NewDashboardModel(scanner, manager, profiles)
	m.screen = screenWelcome
	m.scanning = false
	return m
}

// Init starts the async scan unless the welcome choice is still pending.
func (m DashboardModel) Init() tea.Cmd {
	if m.screen == screenWelcome {
		return nil
	}
	return m.scanCmd()
}

// --- Command constructors (all I/O in closure, never in Update) ---

func (m DashboardModel) scanCmd() tea.Cmd {
	return func() tea.Msg {
		report, err := m.scanner.Scan(context.Background())
		return scanResultMsg{report: report, err: err}
	}
}

func (m DashboardModel) readinessCmd() tea.Cmd {
	report := m.report
	profiles := m.profiles
	return func() tea.Msg {
		items := ComputeReadiness(manifest.AllProviders(), report, profiles)
		return providerReadinessMsg{items: items}
	}
}

func (m DashboardModel) offlineValidationCmd() tea.Cmd {
	mgr := m.manager
	profiles := m.profiles
	var prov provider.MCPProvider
	if len(m.readiness) > 0 && m.selectedProv < len(m.readiness) {
		prov, _ = provider.DefaultRegistry().Get(string(m.readiness[m.selectedProv].Meta.ID))
	}
	return func() tea.Msg {
		if mgr == nil || prov == nil {
			return validationResultMsg{live: false, err: nil, report: validate.Report{}}
		}
		rep, err := mgr.Validate(context.Background(), prov, profiles, false)
		return validationResultMsg{report: rep, live: false, err: err}
	}
}

func (m DashboardModel) liveValidationCmd() tea.Cmd {
	mgr := m.manager
	profiles := m.profiles
	var prov provider.MCPProvider
	if len(m.readiness) > 0 && m.selectedProv < len(m.readiness) {
		prov, _ = provider.DefaultRegistry().Get(string(m.readiness[m.selectedProv].Meta.ID))
	}
	return func() tea.Msg {
		if mgr == nil || prov == nil {
			return validationResultMsg{live: true, err: nil, report: validate.Report{}}
		}
		rep, err := mgr.Validate(context.Background(), prov, profiles, true)
		return validationResultMsg{report: rep, live: true, err: err}
	}
}

func (m DashboardModel) planCmd() tea.Cmd {
	mgr := m.manager
	profiles := m.profiles
	selected := m.buildAppSelection()
	targetFiles := m.selectedTargetFiles()
	assignmentCount := len(profiles)
	if assignmentCount == 0 {
		if prov, ok := m.selectedProvider(); ok && len(prov.RequiredCredentials()) == 0 {
			assignmentCount = 1
		}
	}
	prov, _ := m.selectedProvider()
	credRefs := credentialRefsForProfiles(profiles)
	return func() tea.Msg {
		if mgr == nil || prov == nil {
			return planCreatedMsg{err: errorf("no provider selected")}
		}
		executionPlan, err := mgr.PrepareProviderWithTargetFiles(prov, profiles, selected, app.DefaultAssignments(selected, assignmentCount), targetFiles)
		if err != nil {
			return planCreatedMsg{err: err}
		}
		planID, err := app.NewPlanID()
		if err != nil {
			return planCreatedMsg{err: err}
		}
		savedPlan, err := mgr.BuildSavedPlan(executionPlan, app.SavedPlanOptions{
			PlanID:            planID,
			CreatedAt:         time.Now().UTC(),
			UsyncVersion:      "tui",
			ProviderID:        string(prov.ID()),
			UseInputVariables: true,
			Credentials:       credRefs,
		})
		if err != nil {
			return planCreatedMsg{err: err}
		}
		store, err := app.NewPlanStore(mgr.HomeDir())
		if err != nil {
			return planCreatedMsg{err: err}
		}
		path, err := store.Save(savedPlan, "")
		if err != nil {
			return planCreatedMsg{err: err}
		}
		return planCreatedMsg{plan: savedPlan, path: path}
	}
}

func (m DashboardModel) preflightCmd() tea.Cmd {
	mgr := m.manager
	plan := m.currentPlan
	supplied := credentialSupplyMap(m.profiles)
	return func() tea.Msg {
		if mgr == nil || plan == nil {
			return preflightResultMsg{err: errorf("no plan available")}
		}
		preflight, err := mgr.PreflightSavedPlan(*plan, app.SavedPlanApplyOptions{
			Credentials: supplied,
		})
		return preflightResultMsg{preflight: preflight, err: err}
	}
}

func (m DashboardModel) applyCmd() tea.Cmd {
	mgr := m.manager
	plan := m.currentPlan
	supplied := credentialSupplyMap(m.profiles)
	return func() tea.Msg {
		if mgr == nil || plan == nil {
			return dashApplyResultMsg{err: errorf("no plan available")}
		}
		result, err := mgr.ApplySavedPlan(*plan, app.SavedPlanApplyOptions{
			AutoApprove: true,
			Credentials: supplied,
		})
		return dashApplyResultMsg{result: result, err: err}
	}
}

// credentialRefsForProfiles builds CredentialRef descriptors from TUI profiles
// for inclusion in SavedPlanOptions.Credentials. The Label field matches
// profile.Label so buildPlanOperation can link operations to refs by label.
func credentialRefsForProfiles(profiles []provider.CredentialProfile) []app.CredentialRef {
	seen := make(map[string]bool)
	var refs []app.CredentialRef
	for _, profile := range profiles {
		for key := range profile.Values {
			id := key
			if profile.Label != "" {
				id = key + ":" + profile.Label
			}
			if seen[id] {
				continue
			}
			seen[id] = true
			refs = append(refs, app.CredentialRef{
				Key:   key,
				Label: profile.Label,
			})
		}
	}
	return refs
}

// credentialSupplyMap builds the map[string]string for SavedPlanApplyOptions.Credentials.
// Keys are the ref IDs computed by cloneCredentialRefs (key+":"+label), with a
// plain-key fallback entry for single-profile providers.
func credentialSupplyMap(profiles []provider.CredentialProfile) map[string]string {
	out := make(map[string]string)
	for _, profile := range profiles {
		for key, value := range profile.Values {
			refID := key
			if profile.Label != "" {
				refID = key + ":" + profile.Label
			}
			out[refID] = value
			if _, exists := out[key]; !exists {
				out[key] = value
			}
		}
	}
	return out
}

// buildAppSelection converts selected target rows to the map type needed by PrepareProvider.
func (m DashboardModel) buildAppSelection() map[config.AppID]bool {
	entries := allTargetEntries(m.report, m.resolvedConflicts, m.includeWorkspace)
	selectedTargets := m.effectiveSelectedTargets(entries)
	sel := make(map[config.AppID]bool)
	for _, entry := range entries {
		if entry.isConflict {
			continue
		}
		if selectedTargets[entry.id] {
			sel[config.AppID(entry.clientID)] = true
		}
	}
	return sel
}

func (m DashboardModel) selectedProvider() (provider.MCPProvider, bool) {
	if len(m.readiness) == 0 || m.selectedProv < 0 || m.selectedProv >= len(m.readiness) {
		return nil, false
	}
	return provider.DefaultRegistry().Get(string(m.readiness[m.selectedProv].Meta.ID))
}

func (m DashboardModel) providerAtCursor() (provider.MCPProvider, bool) {
	if len(m.readiness) == 0 || m.providerCursor < 0 || m.providerCursor >= len(m.readiness) {
		return nil, false
	}
	return provider.DefaultRegistry().Get(string(m.readiness[m.providerCursor].Meta.ID))
}

func (m DashboardModel) selectedProviderNeedsCredentials() bool {
	prov, ok := m.selectedProvider()
	return ok && m.providerNeedsCredentials(prov)
}

func (m DashboardModel) cursorProviderNeedsCredentials() bool {
	prov, ok := m.providerAtCursor()
	return ok && m.providerNeedsCredentials(prov)
}

func (m DashboardModel) providerNeedsCredentials(prov provider.MCPProvider) bool {
	if prov == nil || len(prov.RequiredCredentials()) == 0 {
		return false
	}
	for _, profile := range m.profiles {
		if profile.ProviderID == prov.ID() {
			return false
		}
	}
	return true
}

func (m DashboardModel) selectedTargetCount() int {
	count := 0
	entries := allTargetEntries(m.report, m.resolvedConflicts, m.includeWorkspace)
	selectedTargets := m.effectiveSelectedTargets(entries)
	for _, entry := range entries {
		if !entry.isConflict && selectedTargets[entry.id] {
			count++
		}
	}
	return count
}

func (m DashboardModel) selectedTargetFiles() app.TargetFileOverrides {
	entries := allTargetEntries(m.report, m.resolvedConflicts, m.includeWorkspace)
	selectedTargets := m.effectiveSelectedTargets(entries)
	out := make(app.TargetFileOverrides)
	for _, entry := range entries {
		if entry.isConflict || !selectedTargets[entry.id] || entry.path == "" {
			continue
		}
		out[config.AppID(entry.clientID)] = append(out[config.AppID(entry.clientID)], targetFileForEntry(entry))
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// errorf is a minimal error constructor used inside tea.Cmd closures.
func errorf(msg string) error {
	return &dashboardError{msg}
}

type dashboardError struct{ msg string }

func (e *dashboardError) Error() string { return e.msg }

// --- Update ---

func (m DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.recorder != nil {
			recordKey(m.recorder, msg, m.Snapshot(), "")
		}
		next, cmd := m.handleKey(msg)
		if m.recorder != nil {
			nextDashboard, _ := next.(DashboardModel)
			if cmd != nil && isTeaQuit(cmd) && !nextDashboard.RouteToWizard {
				m.recorder.Record(RecordEntry{Kind: "final"})
				_ = m.recorder.Close()
			}
		}
		return next, cmd
	case scanResultMsg:
		m.scanning = false
		m.report = msg.report
		m.err = msg.err
		if msg.err == nil {
			if m.manager != nil {
				m.readiness = ComputeReadiness(manifest.AllProviders(), m.report, m.profiles)
				rendered := RenderedProviderIndices(m.readiness, hasConflictClient(m.report))
				m.providerCursor = clampProviderCursor(rendered, m.providerCursor)
				m.selectedProv = clampProviderCursor(rendered, m.selectedProv)
			}
			m.selectedTargets = defaultSelectedTargets(m.report, m.resolvedConflicts, m.includeWorkspace)
			m.selectedClients = deriveSelectedClients(
				allTargetEntries(m.report, m.resolvedConflicts, m.includeWorkspace),
				m.selectedTargets,
			)
			m = m.applyResolutions()
			m = m.clampClientCursor()
		}
	case providerReadinessMsg:
		m.computingReady = false
		m.readiness = msg.items
		m.readinessErr = msg.err
		if msg.err == nil {
			m.selectedProv = firstReadyIndex(m.readiness)
			m.providerCursor = m.selectedProv
			m.selectedTargets = defaultSelectedTargets(m.report, m.resolvedConflicts, m.includeWorkspace)
			m.selectedClients = deriveSelectedClients(
				allTargetEntries(m.report, m.resolvedConflicts, m.includeWorkspace),
				m.selectedTargets,
			)
			m = m.applyResolutions()
		}
	case validationResultMsg:
		m.validating = false
		m.validErr = msg.err
		if msg.err == nil {
			m.validReport = &msg.report
		}
		// Offline pass with no failures → advance to target selection.
		if !msg.live && msg.err == nil && !msg.report.HasFailures() {
			m.screen = screenTargetSelect
		}
	case planCreatedMsg:
		m.planning = false
		m.planErr = msg.err
		if msg.err == nil {
			m.currentPlan = &msg.plan
			m.planPath = msg.path
			m.preflighting = true
			return m, m.preflightCmd()
		}
	case preflightResultMsg:
		m.preflighting = false
		if msg.err != nil {
			m.planErr = msg.err
		} else {
			m.planPreflight = &msg.preflight
			m.screen = screenPlanPreview
		}
	case dashApplyResultMsg:
		m.applying = false
		m.applyErr = msg.err
		if msg.err == nil {
			m.applyResult = &msg.result
		}
		m.screen = screenApplyResult
		// Rescan after apply.
		m.scanning = true
		return m, m.scanCmd()
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m DashboardModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Global keys work on every screen.
	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?":
		m.showHelp = !m.showHelp
		return m, nil
	}

	switch m.screen {
	case screenWelcome:
		return m.handleKeyWelcome(key)
	case screenDoctor:
		return m.handleKeyDoctor(key)
	case screenProviderReady:
		return m.handleKeyProviderReady(msg)
	case screenTargetSelect:
		return m.handleKeyTargetSelect(msg)
	case screenConflictResolve:
		return m.handleKeyConflictResolve(key)
	case screenCredentialEntry:
		return m.handleKeyCredentialEntry(msg)
	case screenPlanPreview:
		return m.handleKeyPlanPreview(key)
	case screenApplyResult:
		return m.handleKeyApplyResult(key)
	}
	return m, nil
}

func (m DashboardModel) handleKeyWelcome(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up":
		if m.welcomeCursor > 0 {
			m.welcomeCursor--
		}
	case "down":
		if m.welcomeCursor < 1 {
			m.welcomeCursor++
		}
	case "d":
		return m.startDoctorMode()
	case "w":
		m.RouteToWizard = true
		return m, tea.Quit
	case "enter":
		if m.welcomeCursor == 1 {
			m.RouteToWizard = true
			return m, tea.Quit
		}
		return m.startDoctorMode()
	}
	return m, nil
}

func (m DashboardModel) startDoctorMode() (tea.Model, tea.Cmd) {
	m.screen = screenDoctor
	m.placeholderMsg = ""
	if m.scanning {
		return m, nil
	}
	m.scanning = true
	m.err = nil
	return m, m.scanCmd()
}

func (m DashboardModel) handleKeyDoctor(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "r":
		if !m.scanning {
			m.scanning = true
			m.err = nil
			m.placeholderMsg = ""
			return m, m.scanCmd()
		}
	case "w":
		// DM-P66: wizard is always available, even when scan error is present
		m.RouteToWizard = true
		return m, tea.Quit
	case "p", "enter":
		if !m.scanning && m.err == nil && m.manager != nil {
			m.screen = screenProviderReady
			m.computingReady = true
			return m, m.readinessCmd()
		}
	}
	return m, nil
}

func (m DashboardModel) handleKeyProviderReady(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	hasConflicts := hasConflictClient(m.report)
	rendered := RenderedProviderIndices(m.readiness, hasConflicts)
	hasSelectable := len(rendered) > 0

	switch key {
	case "esc":
		m.validating = false
		m.validErr = nil
		m.screen = screenDoctor
	case "r":
		if hasConflicts {
			m.screen = screenTargetSelect
			m.planErr = nil
		}
	case "up":
		if hasSelectable {
			m.providerCursor = prevRenderedIndex(rendered, m.providerCursor)
		}
	case "k":
		if m.cursorProviderNeedsCredentials() {
			return m.openCredentialEntry(screenProviderReady, m.providerCursor)
		}
		if hasSelectable {
			m.providerCursor = prevRenderedIndex(rendered, m.providerCursor)
		}
	case "down", "j":
		if hasSelectable {
			m.providerCursor = nextRenderedIndex(rendered, m.providerCursor)
		}
	case "v":
		if !hasSelectable {
			return m, nil
		}
		if !m.validating {
			m.selectedProv = m.providerCursor
			m.validating = true
			return m, m.liveValidationCmd()
		}
	case "enter":
		if !hasSelectable {
			// Phase 13 DM-P40: no provider is renderable. Treat Enter as a synonym for [r]
			// so the user is never left with a footer key that triggers invisible validation.
			if hasConflicts {
				m.screen = screenTargetSelect
				m.planErr = nil
			} else if m.cursorProviderNeedsCredentials() {
				return m.openCredentialEntry(screenProviderReady, m.providerCursor)
			}
			return m, nil
		}
		if !m.validating {
			m.selectedProv = m.providerCursor
			m.validating = true
			return m, m.offlineValidationCmd()
		}
	}
	return m, nil
}

// prevRenderedIndex returns the previous index in rendered before current, or
// the first if current is at or below the first.
func prevRenderedIndex(rendered []int, current int) int {
	if len(rendered) == 0 {
		return current
	}
	var pos int
	for i, idx := range rendered {
		if idx >= current {
			pos = i
			break
		}
		pos = i
	}
	if pos > 0 {
		return rendered[pos-1]
	}
	return rendered[0]
}

// nextRenderedIndex returns the next index in rendered after current, or the
// last if current is at or above the last.
func nextRenderedIndex(rendered []int, current int) int {
	if len(rendered) == 0 {
		return current
	}
	for _, idx := range rendered {
		if idx > current {
			return idx
		}
	}
	return rendered[len(rendered)-1]
}

func clampProviderCursor(rendered []int, current int) int {
	if len(rendered) == 0 {
		return 0
	}
	if current <= rendered[0] {
		return rendered[0]
	}
	for i := 1; i < len(rendered); i++ {
		if current < rendered[i] {
			return rendered[i-1]
		}
	}
	return rendered[len(rendered)-1]
}

func (m DashboardModel) handleKeyTargetSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	entries := allTargetEntries(m.report, m.resolvedConflicts, m.includeWorkspace)
	switch key {
	case "esc":
		m.planErr = nil
		m.screen = screenProviderReady
	case "up":
		if m.clientCursor > 0 {
			m.clientCursor--
		}
	case "k":
		onConflict := m.clientCursor < len(entries) && entries[m.clientCursor].isConflict
		if m.selectedProviderNeedsCredentials() && !onConflict {
			return m.openCredentialEntry(screenTargetSelect, m.selectedProv)
		}
		if m.clientCursor > 0 {
			m.clientCursor--
		}
	case "down", "j":
		if m.clientCursor < len(entries)-1 {
			m.clientCursor++
		}
	case " ":
		if m.clientCursor < len(entries) && !entries[m.clientCursor].isConflict {
			id := entries[m.clientCursor].id
			if m.selectedTargets == nil {
				m.selectedTargets = defaultSelectedTargets(m.report, m.resolvedConflicts, m.includeWorkspace)
			}
			if m.selectedTargets[id] {
				delete(m.selectedTargets, id)
			} else {
				m.selectedTargets[id] = true
			}
			m.selectedClients = deriveSelectedClients(entries, m.selectedTargets)
			m.planErr = nil
		}
	case "r":
		if m.clientCursor < len(entries) {
			entry := entries[m.clientCursor]
			if entry.isConflict {
				m = m.openConflictResolve(entry)
				return m, nil
			}
		}
		if index := firstConflictEntryIndex(entries); index >= 0 {
			m.clientCursor = index
			m = m.openConflictResolve(entries[index])
			return m, nil
		}
	case "enter":
		if m.clientCursor < len(entries) {
			entry := entries[m.clientCursor]
			if entry.isConflict {
				m = m.openConflictResolve(entry)
				return m, nil
			}
		}
		if !m.planning {
			if m.selectedProviderNeedsCredentials() {
				m.planErr = nil
				return m, nil
			}
			if m.selectedTargetCount() == 0 {
				m.planErr = nil
				return m, nil
			}
			m.planning = true
			m.planErr = nil
			return m, m.planCmd()
		}
	case "i":
		previousEntries := entries
		m.includeWorkspace = !m.includeWorkspace
		m = m.reconcileTargetSelection(previousEntries)
	}
	return m, nil
}

func (m DashboardModel) openCredentialEntry(returnTo dashboardScreen, readinessIndex int) (tea.Model, tea.Cmd) {
	if readinessIndex < 0 || readinessIndex >= len(m.readiness) {
		return m, nil
	}
	prov, ok := provider.DefaultRegistry().Get(string(m.readiness[readinessIndex].Meta.ID))
	if !ok || !m.providerNeedsCredentials(prov) {
		return m, nil
	}
	m.selectedProv = readinessIndex
	m.credReturnTo = returnTo
	m.credEntry = newCredentialEntryState(prov, m.readiness[readinessIndex].Meta)
	m.screen = screenCredentialEntry
	return m, nil
}

func (m DashboardModel) openConflictResolve(entry targetEntry) DashboardModel {
	for i := range m.report.Clients {
		if manifest.ClientID(m.report.Clients[i].ID) == entry.clientID {
			c := m.report.Clients[i] // copy, not pointer to slice element
			m.resolveTarget = &c
			break
		}
	}
	m.screen = screenConflictResolve
	return m
}

func (m DashboardModel) handleKeyConflictResolve(key string) (tea.Model, tea.Cmd) {
	if m.resolveTarget == nil {
		m.screen = screenTargetSelect
		return m, nil
	}
	candidates := conflictCandidatesForDisplay(*m.resolveTarget)
	id := manifest.ClientID(m.resolveTarget.ID)
	switch key {
	case "esc":
		m.screen = screenTargetSelect
		m.resolveTarget = nil
	case "s":
		m.resolvedConflicts = setResolution(m.resolvedConflicts, id, ConflictResolution{Skipped: true})
		m = m.reconcileTargetSelection(nil)
		m.screen = screenTargetSelect
		m.resolveTarget = nil
		m = m.clampClientCursor()
	case "1":
		if len(candidates) >= 1 {
			m.resolvedConflicts = setResolution(m.resolvedConflicts, id, ConflictResolution{
				ChosenPath:  candidates[0].Path,
				ChosenLabel: candidates[0].Label,
			})
			m = m.reconcileTargetSelection(nil)
			m.screen = screenTargetSelect
			m.resolveTarget = nil
		}
	case "2":
		if len(candidates) >= 2 {
			m.resolvedConflicts = setResolution(m.resolvedConflicts, id, ConflictResolution{
				ChosenPath:  candidates[1].Path,
				ChosenLabel: candidates[1].Label,
			})
			m = m.reconcileTargetSelection(nil)
			m.screen = screenTargetSelect
			m.resolveTarget = nil
		}
	}
	return m, nil
}

func (m DashboardModel) handleKeyPlanPreview(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "n", "esc":
		m.screen = screenTargetSelect
	case "y", "enter":
		if !m.applying {
			m.applying = true
			m.applyErr = nil
			return m, m.applyCmd()
		}
	}
	return m, nil
}

func (m DashboardModel) handleKeyApplyResult(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "r":
		if !m.scanning {
			m.scanning = true
			return m, m.scanCmd()
		}
	case "esc":
		// DM-P65: Esc is a no-op on ApplyResult; only forward keys are r (rescan) and q (quit)
		return m, nil
	}
	return m, nil
}

// firstReadyIndex returns the index of the first ready or no-key-needed provider.
// Ready and no-key providers are rendered first in the provider list, so this
// ensures the cursor starts at the top of the visual list.
func firstReadyIndex(items []ProviderReadinessItem) int {
	for i, item := range items {
		if item.State == ProviderStateReady || item.State == ProviderStateNoKeyNeeded {
			return i
		}
	}
	return 0
}

// allTargetEntries returns a unified ordered list for screenTargetSelect:
// 1. Eligible clients (high/medium, present, not conflict).
// 2. Resolved-non-skipped conflict clients (now eligible).
// 3. Unresolved conflict clients (cursor reachable but not selectable).
// Skipped conflicts are omitted entirely.
func allTargetEntries(report doctor.Report, resolved map[manifest.ClientID]ConflictResolution, includeWorkspace bool) []targetEntry {
	var entries []targetEntry
	// Section 1: standard eligible clients.
	for _, c := range report.Clients {
		if c.Confidence == doctor.ConfidenceConflict || c.Confidence == doctor.ConfidenceLow {
			continue
		}
		if !c.Installed && c.EffectivePath == "" {
			continue
		}
		entries = append(entries, targetEntriesForClient(c, includeWorkspace)...)
	}
	// Section 2: resolved (non-skipped) conflict clients — treated as eligible.
	for _, c := range report.Clients {
		if c.Confidence != doctor.ConfidenceConflict {
			continue
		}
		r, ok := resolved[manifest.ClientID(c.ID)]
		if ok && !r.Skipped {
			entries = append(entries, targetEntryForResolution(c, r))
		}
	}
	// Section 3: unresolved conflict clients — visible but not selectable.
	for _, c := range report.Clients {
		if c.Confidence != doctor.ConfidenceConflict {
			continue
		}
		if _, ok := resolved[manifest.ClientID(c.ID)]; !ok {
			entry := targetEntry{
				clientID:   manifest.ClientID(c.ID),
				name:       c.Name,
				isConflict: true,
			}
			entry.id = targetEntryID(entry)
			entries = append(entries, targetEntry{
				id:         entry.id,
				clientID:   entry.clientID,
				name:       entry.name,
				isConflict: entry.isConflict,
			})
		}
	}
	return entries
}

func targetEntriesForClient(c doctor.ClientFinding, includeWorkspace bool) []targetEntry {
	var entries []targetEntry
	for _, cand := range c.Candidates {
		if !targetCandidateVisible(cand, includeWorkspace) {
			continue
		}
		entry := targetEntryFromCandidate(c, cand)
		entries = append(entries, entry)
	}
	if len(entries) > 0 {
		return entries
	}

	entry := targetEntry{
		clientID: manifest.ClientID(c.ID),
		name:     c.Name,
		label:    c.Name,
		path:     c.EffectivePath,
		scope:    manifest.ScopeUser,
		kind:     config.FileKindMCPServers,
		exists:   c.EffectivePath != "",
	}
	entry.id = targetEntryID(entry)
	return []targetEntry{entry}
}

func targetEntryForResolution(c doctor.ClientFinding, r ConflictResolution) targetEntry {
	for _, cand := range c.Candidates {
		if cand.Path == r.ChosenPath {
			return targetEntryFromCandidate(c, cand)
		}
	}
	entry := targetEntry{
		clientID: manifest.ClientID(c.ID),
		name:     c.Name,
		label:    r.ChosenLabel,
		path:     r.ChosenPath,
		scope:    manifest.ScopeUser,
		kind:     config.FileKindMCPServers,
		exists:   r.ChosenPath != "",
	}
	if entry.label == "" {
		entry.label = c.Name
	}
	entry.id = targetEntryID(entry)
	return entry
}

func targetEntryFromCandidate(c doctor.ClientFinding, cand doctor.CandidateFinding) targetEntry {
	manifestCandidate, _ := manifestCandidateFor(c.ID, cand.Label)
	kind := config.FileKindMCPServers
	creatable := false
	gitWarning := false
	if manifestCandidate.Label != "" {
		kind = fileKindForManifestMutation(manifestCandidate.MutationKind)
		creatable = manifestCandidate.Creatable
		gitWarning = manifestCandidate.GitWarning
	}
	scope := cand.Scope
	if scope == "" {
		scope = manifest.ScopeUser
	}
	entry := targetEntry{
		clientID:   manifest.ClientID(c.ID),
		name:       c.Name,
		label:      cand.Label,
		path:       cand.Path,
		scope:      scope,
		kind:       kind,
		exists:     cand.Exists,
		creatable:  creatable,
		gitWarning: gitWarning,
	}
	if entry.label == "" {
		entry.label = c.Name
	}
	entry.id = targetEntryID(entry)
	return entry
}

func targetCandidateVisible(cand doctor.CandidateFinding, includeWorkspace bool) bool {
	if cand.Path == "" {
		return false
	}
	if cand.Scope == manifest.ScopeManaged {
		return false
	}
	if (cand.Scope == manifest.ScopeProject || cand.Scope == manifest.ScopeWorkspace) && !includeWorkspace {
		return false
	}
	return cand.Exists || cand.Writable || cand.ParseOK
}

func manifestCandidateFor(clientID manifest.ClientID, label string) (manifest.ConfigCandidate, bool) {
	for _, client := range manifest.AllClients() {
		if client.ID != clientID {
			continue
		}
		for _, cand := range client.Candidates {
			if cand.Label == label {
				return cand, true
			}
		}
	}
	return manifest.ConfigCandidate{}, false
}

func fileKindForManifestMutation(kind manifest.MutationKind) config.FileKind {
	switch kind {
	case manifest.MutationBareMCPServers:
		return config.FileKindBareMCPServers
	case manifest.MutationNamedServer:
		return config.FileKindNamedServer
	case manifest.MutationCodexTOML:
		return config.FileKindCodexTOML
	case manifest.MutationClaudeCodeCLI:
		return config.FileKindClaudeCodeCLI
	default:
		return config.FileKindMCPServers
	}
}

func targetEntryID(entry targetEntry) string {
	id := string(entry.clientID)
	if entry.label != "" {
		id += "|" + entry.label
	}
	if entry.path != "" {
		id += "|" + entry.path
	}
	if entry.isConflict {
		id += "|conflict"
	}
	return id
}

func targetFileForEntry(entry targetEntry) config.TargetFile {
	return config.TargetFile{
		Label:      targetEntryLabel(entry),
		Path:       entry.path,
		Kind:       entry.kind,
		Exists:     entry.exists,
		Creatable:  entry.creatable,
		Scope:      string(entry.scope),
		GitWarning: entry.gitWarning,
	}
}

func targetEntryLabel(entry targetEntry) string {
	if entry.label == "" || entry.label == entry.name {
		return entry.name
	}
	return entry.name + " " + entry.label
}

func defaultSelectedTargets(report doctor.Report, resolved map[manifest.ClientID]ConflictResolution, includeWorkspace bool) map[string]bool {
	selected := make(map[string]bool)
	for _, entry := range allTargetEntries(report, resolved, includeWorkspace) {
		if !entry.isConflict {
			selected[entry.id] = true
		}
	}
	return selected
}

func deriveSelectedClients(entries []targetEntry, selectedTargets map[string]bool) map[manifest.ClientID]bool {
	selected := make(map[manifest.ClientID]bool)
	for _, entry := range entries {
		if !entry.isConflict && selectedTargets[entry.id] {
			selected[entry.clientID] = true
		}
	}
	return selected
}

func (m DashboardModel) effectiveSelectedTargets(entries []targetEntry) map[string]bool {
	if m.selectedTargets != nil {
		return m.selectedTargets
	}
	selected := make(map[string]bool)
	for _, entry := range entries {
		if !entry.isConflict && m.selectedClients[entry.clientID] {
			selected[entry.id] = true
		}
	}
	if len(selected) > 0 {
		return selected
	}
	return defaultSelectedTargets(m.report, m.resolvedConflicts, m.includeWorkspace)
}

func (m DashboardModel) reconcileTargetSelection(previousEntries []targetEntry) DashboardModel {
	entries := allTargetEntries(m.report, m.resolvedConflicts, m.includeWorkspace)
	previous := m.selectedTargets
	if previous == nil {
		previous = defaultSelectedTargets(m.report, m.resolvedConflicts, m.includeWorkspace)
	}
	previousEntryIDs := make(map[string]bool, len(previousEntries))
	for _, entry := range previousEntries {
		previousEntryIDs[entry.id] = true
	}
	next := make(map[string]bool)
	for _, entry := range entries {
		if entry.isConflict {
			continue
		}
		_, wasVisible := previousEntryIDs[entry.id]
		if previous[entry.id] || entry.clientIDSelectedByResolution(m.resolvedConflicts) || (len(previousEntries) > 0 && !wasVisible) {
			next[entry.id] = true
		}
	}
	m.selectedTargets = next
	m.selectedClients = deriveSelectedClients(entries, next)
	m = m.clampClientCursor()
	return m
}

func (entry targetEntry) clientIDSelectedByResolution(resolved map[manifest.ClientID]ConflictResolution) bool {
	r, ok := resolved[entry.clientID]
	if !ok || r.Skipped {
		return false
	}
	return r.ChosenPath == entry.path
}

// conflictCandidatesForDisplay returns the first two candidates that exist or are symlinks.
func conflictCandidatesForDisplay(c doctor.ClientFinding) []doctor.CandidateFinding {
	var out []doctor.CandidateFinding
	for _, cand := range c.Candidates {
		if cand.Exists || cand.IsSymlink {
			out = append(out, cand)
			if len(out) == 2 {
				break
			}
		}
	}
	return out
}

func firstConflictEntryIndex(entries []targetEntry) int {
	for i, entry := range entries {
		if entry.isConflict {
			return i
		}
	}
	return -1
}

// setResolution initialises the map if nil and stores the resolution.
func setResolution(m map[manifest.ClientID]ConflictResolution, id manifest.ClientID, r ConflictResolution) map[manifest.ClientID]ConflictResolution {
	if m == nil {
		m = make(map[manifest.ClientID]ConflictResolution)
	}
	m[id] = r
	return m
}

func (m DashboardModel) clampClientCursor() DashboardModel {
	entries := allTargetEntries(m.report, m.resolvedConflicts, m.includeWorkspace)
	if len(entries) == 0 {
		m.clientCursor = 0
		return m
	}
	if m.clientCursor >= len(entries) {
		m.clientCursor = len(entries) - 1
	}
	if m.clientCursor < 0 {
		m.clientCursor = 0
	}
	return m
}

// applyResolutions re-adds resolved (non-skipped) conflict clients to selectedClients.
// Called after each rescan so resolutions survive the refresh.
func (m DashboardModel) applyResolutions() DashboardModel {
	return m.reconcileTargetSelection(nil)
}
