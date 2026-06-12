package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nawodyaishan/universal-mcp-sync/pkg/client"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/config"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/provider"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/redact"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/verify"
)

type CommandRunner interface {
	LookPath(name string) (string, error)
	Run(name string, args ...string) (string, error)
}

type WriteConfigFunc func(path string, data []byte, now time.Time) (config.WriteOutcome, error)

type ExecutionPlan struct {
	Operations []Operation
	Warnings   []string
}

type Operation struct {
	AppID           config.AppID
	AppName         string
	FileLabel       string
	Path            string
	Kind            config.FileKind
	Scope           string
	GitWarning      bool
	CredentialLabel string
	ProviderID      string
	Config          provider.MCPConfig
	BackupPath      string
	WillCreate      bool
	SkipReason      string
	CLIAddArgs      []string
	CLIRemoveArgs   []string
}

type TargetPathOverrides map[config.AppID]string
type TargetFileOverrides map[config.AppID][]config.TargetFile

type ApplyResult struct {
	Plan           ExecutionPlan
	Warnings       []string
	BackupPaths    []string
	Verification   []verify.Result
	UpdatedTargets []string
	SkippedTargets []string // files skipped because content is byte-identical to existing
	RolledBack     []string
	RollbackFailed []string
}

type Manager struct {
	HomeDir     string
	Apps        []config.AppConfig
	Now         func() time.Time
	Runner      CommandRunner
	Logger      *slog.Logger
	WriteConfig WriteConfigFunc
}

type osRunner struct{}

type preparedWrite struct {
	op      Operation
	content []byte
	skipped bool // true when bytes.Equal(existing file bytes, content)
}

func NewManager(homeDir string, now func() time.Time, runner CommandRunner) (*Manager, error) {
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home directory: %w", err)
		}
	}
	if now == nil {
		now = time.Now
	}
	if runner == nil {
		runner = osRunner{}
	}

	apps, err := detectAppConfigs(homeDir)
	if err != nil {
		return nil, err
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if os.Getenv("USYNC_DEBUG") == "true" {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}

	return &Manager{
		HomeDir:     homeDir,
		Apps:        apps,
		Now:         now,
		Runner:      runner,
		Logger:      logger,
		WriteConfig: config.WriteWithBackup,
	}, nil
}

func detectAppConfigs(homeDir string) ([]config.AppConfig, error) {
	if targetOS := os.Getenv("USYNC_TARGET_OS"); targetOS != "" {
		return config.DetectAppConfigsForOS(homeDir, targetOS)
	}
	return config.DetectAppConfigs(homeDir)
}

func (m *Manager) PrepareProvider(
	prov provider.MCPProvider,
	profiles []provider.CredentialProfile,
	selected map[config.AppID]bool,
	assignments map[config.AppID]int,
) (ExecutionPlan, error) {
	return m.PrepareProviderWithTargetFiles(prov, profiles, selected, assignments, nil)
}

func (m *Manager) PrepareProviderWithTargetPaths(
	prov provider.MCPProvider,
	profiles []provider.CredentialProfile,
	selected map[config.AppID]bool,
	assignments map[config.AppID]int,
	targetPaths TargetPathOverrides,
) (ExecutionPlan, error) {
	targetFiles, err := targetFilesFromPaths(m.Apps, targetPaths)
	if err != nil {
		return ExecutionPlan{}, err
	}
	return m.PrepareProviderWithTargetFiles(prov, profiles, selected, assignments, targetFiles)
}

func (m *Manager) PrepareProviderWithTargetFiles(
	prov provider.MCPProvider,
	profiles []provider.CredentialProfile,
	selected map[config.AppID]bool,
	assignments map[config.AppID]int,
	targetFiles TargetFileOverrides,
) (ExecutionPlan, error) {
	if len(profiles) == 0 {
		if len(prov.RequiredCredentials()) > 0 {
			return ExecutionPlan{}, fmt.Errorf("at least one credential profile is required")
		}
		profiles = []provider.CredentialProfile{{
			ProviderID: prov.ID(),
			Values:     map[string]string{},
			Label:      "Default",
		}}
		if len(assignments) == 0 {
			assignments = DefaultAssignments(selected, len(profiles))
		}
	}

	plan := ExecutionPlan{}
	prereqChecked := make(map[int]bool)
	prereqWarningByProfile := make(map[int]string)
	for _, appConfig := range m.Apps {
		if !selected[appConfig.ID] {
			continue
		}
		if files := targetFiles[appConfig.ID]; len(files) > 0 {
			appConfig.Files = append([]config.TargetFile(nil), files...)
		}

		index, ok := assignments[appConfig.ID]
		if !ok {
			return ExecutionPlan{}, fmt.Errorf("missing credential assignment for %s", appConfig.Name)
		}
		if index < 0 || index >= len(profiles) {
			return ExecutionPlan{}, fmt.Errorf("invalid credential assignment for %s", appConfig.Name)
		}

		profile := profiles[index]
		cfg, err := prov.GenerateConfig(profile.Values)
		if err != nil {
			return ExecutionPlan{}, fmt.Errorf("generate config for %s: %w", prov.ID(), err)
		}

		m.logDebug("preparing app", "app", appConfig.Name, "id", appConfig.ID, "transport", cfg.Type)

		if !prereqChecked[index] {
			prereqChecked[index] = true
			prereqWarningByProfile[index] = providerPrerequisiteWarning(prov.ID(), cfg, m.Runner)
			if prereqWarningByProfile[index] != "" {
				plan.Warnings = append(plan.Warnings, prereqWarningByProfile[index])
			}
		}
		if prereqWarningByProfile[index] != "" {
			m.logDebug("skipping app due to prerequisite failure", "app", appConfig.Name)
			continue
		}

		if appConfig.ID == config.AppClaudeCode {
			m.logDebug("handling Claude Code", "app", appConfig.Name)
			targetPath, targetScope, gitWarning := claudeCodeTarget(appConfig)
			op := Operation{
				AppID:           appConfig.ID,
				AppName:         appConfig.Name,
				FileLabel:       "Claude Code CLI",
				Path:            targetPath,
				Kind:            config.FileKindClaudeCodeCLI,
				Scope:           targetScope,
				GitWarning:      gitWarning,
				CredentialLabel: profile.Label,
				ProviderID:      prov.ID(),
				Config:          cfg,
				CLIRemoveArgs:   []string{"mcp", "remove", prov.ID(), "-s", claudeCodeCLIScope(targetScope)},
				CLIAddArgs:      buildClaudeCodeAddArgs(prov.ID(), cfg, targetScope),
			}
			if _, err := m.Runner.LookPath("claude"); err != nil {
				op.SkipReason = "claude CLI not found; skipping direct mutation of ~/.claude.json"
				plan.Warnings = append(plan.Warnings, op.SkipReason)
			}
			plan.Operations = append(plan.Operations, op)
			continue
		}

		if !client.CanHandle(appConfig.ID, cfg.Type) {
			m.logDebug("client cannot handle transport", "app", appConfig.Name, "transport", cfg.Type)
			plan.Warnings = append(plan.Warnings, fmt.Sprintf(
				"%s does not support %s transport — skipping %s",
				appConfig.Name, cfg.Type, prov.ID(),
			))
			continue
		}

		m.logDebug("handling files for app", "app", appConfig.Name, "fileCount", len(appConfig.Files))
		for _, file := range appConfig.Files {
			m.logDebug("adding operation for file", "app", appConfig.Name, "file", file.Label, "path", file.Path)
			fileCfg := client.Adapt(appConfig.ID, cfg)
			plan.Operations = append(plan.Operations, Operation{
				AppID:           appConfig.ID,
				AppName:         appConfig.Name,
				FileLabel:       file.Label,
				Path:            file.Path,
				Kind:            file.Kind,
				Scope:           file.Scope,
				GitWarning:      file.GitWarning,
				CredentialLabel: profile.Label,
				ProviderID:      prov.ID(),
				Config:          fileCfg,
				BackupPath:      backupPathFor(file, m.Now()),
				WillCreate:      !file.Exists,
			})
		}
	}

	return plan, nil
}

func targetFilesFromPaths(apps []config.AppConfig, targetPaths TargetPathOverrides) (TargetFileOverrides, error) {
	if len(targetPaths) == 0 {
		return nil, nil
	}
	out := make(TargetFileOverrides)
	for appID, targetPath := range targetPaths {
		if targetPath == "" {
			continue
		}
		appConfig, ok := appConfigByID(apps, appID)
		if !ok {
			return nil, fmt.Errorf("%s: selected app config not found", appID)
		}
		var err error
		appConfig, err = appConfigForTargetPath(appConfig, targetPath)
		if err != nil {
			return nil, err
		}
		out[appID] = append([]config.TargetFile(nil), appConfig.Files...)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func appConfigByID(apps []config.AppConfig, id config.AppID) (config.AppConfig, bool) {
	for _, appConfig := range apps {
		if appConfig.ID == id {
			return appConfig, true
		}
	}
	return config.AppConfig{}, false
}

func appConfigForTargetPath(appConfig config.AppConfig, targetPath string) (config.AppConfig, error) {
	for _, file := range appConfig.Files {
		if filepath.Clean(file.Path) == filepath.Clean(targetPath) {
			appConfig.Files = []config.TargetFile{file}
			return appConfig, nil
		}
	}
	return config.AppConfig{}, fmt.Errorf("%s: resolved target path is not a known config candidate", appConfig.Name)
}

func providerPrerequisiteWarning(providerID string, cfg provider.MCPConfig, runner CommandRunner) string {
	if cfg.Type != provider.TransportStdio || cfg.Command != "docker" {
		return ""
	}
	if _, err := runner.LookPath("docker"); err != nil {
		return fmt.Sprintf("%s requires Docker, but the docker CLI was not found; install Docker and retry", providerID)
	}
	if _, err := runner.Run("docker", "info", "--format", "{{.ServerVersion}}"); err != nil {
		return fmt.Sprintf("%s requires Docker to be running; docker info failed: %s", providerID, redact.Text(err.Error()))
	}
	return ""
}

func buildClaudeCodeAddArgs(providerID string, cfg provider.MCPConfig, scope string) []string {
	cliScope := claudeCodeCLIScope(scope)
	if cfg.Type == provider.TransportStdio {
		args := []string{"mcp", "add", "-s", cliScope, providerID, "--", cfg.Command}
		args = append(args, cfg.Args...)
		return args
	}
	return []string{"mcp", "add", "--transport", string(cfg.Type), "-s", cliScope, providerID, cfg.URL}
}

func claudeCodeTarget(appConfig config.AppConfig) (string, string, bool) {
	if len(appConfig.Files) == 0 {
		return "", "user", false
	}
	file := appConfig.Files[0]
	scope := file.Scope
	if scope == "" {
		scope = "user"
	}
	return file.Path, scope, file.GitWarning
}

func claudeCodeCLIScope(scope string) string {
	switch scope {
	case "project", "local", "user":
		return scope
	default:
		return "user"
	}
}

func (m *Manager) Prepare(keys []string, selected map[config.AppID]bool, assignments map[config.AppID]int) (ExecutionPlan, error) {
	if len(keys) == 0 {
		return ExecutionPlan{}, fmt.Errorf("at least one API key is required")
	}

	prov := provider.NewExaProvider()
	profiles := make([]provider.CredentialProfile, len(keys))
	for i, key := range keys {
		profiles[i] = provider.CredentialProfile{
			ProviderID: prov.ID(),
			Values: map[string]string{
				"EXA_API_KEY": key,
			},
			Label: redact.Key(key),
		}
	}

	return m.PrepareProvider(prov, profiles, selected, assignments)
}

func (m *Manager) Apply(plan ExecutionPlan) (ApplyResult, error) {
	result := ApplyResult{Plan: plan}
	m.logInfo("apply start", "operations", len(plan.Operations))

	prepared, cliOps, seenApps, err := m.prepareOperations(plan, &result)
	if err != nil {
		m.logError("apply preflight failed", "error", err.Error())
		return result, err
	}

	outcomes := make([]config.WriteOutcome, 0, len(prepared))
	seenFiles := make(map[string]config.FileKind, len(prepared))
	for _, item := range prepared {
		if item.skipped {
			m.logDebug("skipping identical write", "path", item.op.Path)
			result.SkippedTargets = append(result.SkippedTargets, item.op.Path)
			seenFiles[item.op.Path] = item.op.Kind
			continue
		}
		m.logDebug("writing target", "app", item.op.AppName, "target", item.op.FileLabel, "path", item.op.Path)
		outcome, err := m.WriteConfig(item.op.Path, item.content, m.Now())
		if err != nil {
			result.Warnings = append(result.Warnings, rollbackWarnings(m.rollback(outcomes, &result))...)
			m.logError("apply file write failed", "path", item.op.Path, "error", err.Error())
			return result, fmt.Errorf("%s (%s): %w", item.op.AppName, item.op.FileLabel, err)
		}

		outcomes = append(outcomes, outcome)
		seenFiles[item.op.Path] = item.op.Kind
		if outcome.BackupPath != "" {
			result.BackupPaths = append(result.BackupPaths, outcome.BackupPath)
		}
		result.UpdatedTargets = append(result.UpdatedTargets, item.op.Path)
	}

	for _, op := range cliOps {
		m.logInfo("running external cli update", "app", op.AppName)
		switch op.Kind {
		case config.FileKindCodexCLIAdd:
			args := buildCodexCLIAddArgs(op.ProviderID, op.Config)
			if _, runErr := m.Runner.Run("codex", args...); runErr != nil {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("codex mcp add %s: %v", op.ProviderID, runErr))
			} else {
				result.UpdatedTargets = append(result.UpdatedTargets, "codex mcp add "+op.ProviderID)
				seenApps[op.AppID] = true
			}
		default:
			if err := m.applyClaudeCode(op, &result); err != nil {
				m.logError("external cli update failed", "app", op.AppName, "error", err.Error())
				return result, err
			}
		}
	}

	result.Verification = append(result.Verification, verifyFiles(prepared)...)

	type cliVerifyKey struct {
		appID      config.AppID
		providerID string
	}
	seen := make(map[cliVerifyKey]bool)
	for _, op := range cliOps {
		key := cliVerifyKey{op.AppID, op.ProviderID}
		if seen[key] {
			continue
		}
		seen[key] = true
		switch op.AppID {
		case config.AppClaudeCode:
			result.Verification = append(result.Verification,
				verify.VerifyOptionalCLI(m.Runner, "claude", "mcp", "get", op.ProviderID))
		}
	}
	// Non-CLI app verifications from seenApps
	for appID := range seenApps {
		// Find the provider ID from any operation for this app
		provID := ""
		for _, op := range plan.Operations {
			if op.AppID == appID && op.SkipReason == "" {
				provID = op.ProviderID
				break
			}
		}
		if provID == "" {
			continue
		}
		key := cliVerifyKey{appID, provID}
		if seen[key] {
			continue
		}
		seen[key] = true
		switch appID {
		case config.AppCodexCLI:
			result.Verification = append(result.Verification,
				verify.VerifyOptionalCLI(m.Runner, "codex", "mcp", "get", provID))
		case config.AppAntigravityCLI:
			result.Verification = append(result.Verification,
				verify.VerifyOptionalCLI(m.Runner, "antigravity", "mcp", "get", provID))
		}
	}

	m.logInfo("apply complete", "updated_targets", len(result.UpdatedTargets))
	return result, nil
}

func (m *Manager) applyClaudeCode(op Operation, result *ApplyResult) error {
	if _, err := m.Runner.Run("claude", op.CLIRemoveArgs...); err != nil {
		warning := fmt.Sprintf("claude mcp remove %s: %s", op.ProviderID, redact.Text(err.Error()))
		result.Warnings = append(result.Warnings, warning)
		m.logWarn("claude remove failed before add", "error", warning)
	}
	if _, err := m.Runner.Run("claude", op.CLIAddArgs...); err != nil {
		return fmt.Errorf("claude mcp add %s: %s", op.ProviderID, redact.Text(err.Error()))
	}
	result.UpdatedTargets = append(result.UpdatedTargets, "claude mcp add "+op.ProviderID)
	return nil
}

func DefaultAssignments(selected map[config.AppID]bool, keyCount int) map[config.AppID]int {
	assignments := make(map[config.AppID]int)
	if keyCount <= 0 {
		return assignments
	}

	if keyCount == 1 {
		for appID, isSelected := range selected {
			if isSelected {
				assignments[appID] = 0
			}
		}
		return assignments
	}

	if keyCount == 2 {
		preferred := map[config.AppID]int{
			config.AppClaudeDesktop:  0,
			config.AppAntigravityCLI: 0,
			config.AppCodexCLI:       0,
			config.AppClaudeCode:     1,
			config.AppAntigravity:    1,
		}
		for appID, isSelected := range selected {
			if isSelected {
				assignments[appID] = preferred[appID]
			}
		}
		return assignments
	}

	index := 0
	for _, appID := range config.AppOrder {
		if !selected[appID] {
			continue
		}
		assignments[appID] = index % keyCount
		index++
	}
	return assignments
}

func FormatPlan(plan ExecutionPlan) string {
	var builder strings.Builder
	builder.WriteString("MCP sync plan\n")
	builder.WriteString("=============\n")
	for _, warning := range plan.Warnings {
		builder.WriteString("warning: " + warning + "\n")
	}
	for _, op := range plan.Operations {
		fmt.Fprintf(&builder, "- %s: %s\n", op.AppName, op.FileLabel)
		fmt.Fprintf(&builder, "  credential: %s\n", op.CredentialLabel)
		if op.SkipReason != "" {
			fmt.Fprintf(&builder, "  skip: %s\n", op.SkipReason)
			continue
		}
		if op.Path != "" {
			fmt.Fprintf(&builder, "  path: %s\n", op.Path)
		}
		if op.WillCreate {
			builder.WriteString("  backup: not applicable (new file)\n")
		} else if op.BackupPath != "" {
			fmt.Fprintf(&builder, "  backup: %s\n", op.BackupPath)
		}
	}
	return strings.TrimRight(builder.String(), "\n")
}

func FormatApplyResult(result ApplyResult) string {
	var builder strings.Builder
	builder.WriteString("MCP sync result\n")
	builder.WriteString("===============\n")

	for _, warning := range result.Warnings {
		builder.WriteString("warning: " + warning + "\n")
	}

	if len(result.BackupPaths) > 0 {
		builder.WriteString("Backups\n")
		for _, backup := range result.BackupPaths {
			builder.WriteString("- " + backup + "\n")
		}
	}

	if len(result.UpdatedTargets) > 0 {
		builder.WriteString("Updated\n")
		for _, target := range result.UpdatedTargets {
			builder.WriteString("- " + target + "\n")
		}
	}

	if len(result.SkippedTargets) > 0 {
		fmt.Fprintf(&builder, "Unchanged (%d)\n", len(result.SkippedTargets))
		for _, target := range result.SkippedTargets {
			builder.WriteString("- " + target + "\n")
		}
	}

	if len(result.RolledBack) > 0 {
		builder.WriteString("Rolled Back\n")
		for _, target := range result.RolledBack {
			builder.WriteString("- " + target + "\n")
		}
	}

	if len(result.RollbackFailed) > 0 {
		builder.WriteString("Rollback Failed\n")
		for _, target := range result.RollbackFailed {
			builder.WriteString("- " + target + "\n")
		}
	}

	if len(result.Verification) > 0 {
		builder.WriteString("Verification\n")
		for _, item := range result.Verification {
			fmt.Fprintf(&builder, "- [%s] %s\n", item.Status, item.Target)
			for _, detail := range item.Details {
				builder.WriteString("  " + detail + "\n")
			}
		}
	}

	builder.WriteString("Restart the affected apps to reload the updated MCP configuration.\n")
	return strings.TrimRight(builder.String(), "\n")
}

func backupPathFor(file config.TargetFile, now time.Time) string {
	if !file.Exists {
		return ""
	}
	return config.BuildBackupPath(file.Path, now)
}

func (osRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func (osRunner) Run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return redact.Text(string(output)), errors.New(redact.Text(strings.TrimSpace(string(output))))
	}
	return redact.Text(string(output)), nil
}

func (m *Manager) prepareOperations(plan ExecutionPlan, result *ApplyResult) ([]preparedWrite, []Operation, map[config.AppID]bool, error) {
	prepared := make([]preparedWrite, 0, len(plan.Operations))
	cliOps := make([]Operation, 0, 1)
	seenApps := make(map[config.AppID]bool)

	m.logDebug("preparing operations", "count", len(plan.Operations))

	for _, op := range plan.Operations {
		if op.SkipReason != "" {
			m.logDebug("skipping operation due to SkipReason", "app", op.AppName, "reason", op.SkipReason)
			result.Warnings = append(result.Warnings, op.SkipReason)
			continue
		}

		seenApps[op.AppID] = true
		switch op.Kind {
		case config.FileKindMCPServers, config.FileKindBareMCPServers, config.FileKindNamedServer, config.FileKindCodexTOML:
			m.logDebug("preparing file operation", "app", op.AppName, "path", op.Path)
			item, err := m.prepareFileOperation(op, "")
			if err != nil {
				return nil, nil, nil, err
			}
			prepared = append(prepared, item)
		case config.FileKindClaudeCodeCLI, config.FileKindCodexCLIAdd:
			m.logDebug("preparing CLI operation", "app", op.AppName)
			cliOps = append(cliOps, op)
		default:
			return nil, nil, nil, fmt.Errorf("unsupported operation kind %q", op.Kind)
		}
	}

	return prepared, cliOps, seenApps, nil
}

func (m *Manager) prepareFileOperation(op Operation, planID string) (preparedWrite, error) {
	if err := validateOperationPath(m.HomeDir, op.Scope, op.Path); err != nil {
		return preparedWrite{}, fmt.Errorf("%s (%s): %w", op.AppName, op.FileLabel, err)
	}

	data, existed, err := config.ReadFileOrEmpty(op.Path)
	if err != nil {
		return preparedWrite{}, err
	}

	m.logDebug("read existing config", "path", op.Path, "existed", existed, "size", len(data))

	op.Config.Headers = client.HeadersFor(op.AppID, op.Config.Headers) // augment per-client

	// _usync provenance marker — excluded from Antigravity targets (serverUrl format).
	isAntigravity := op.AppID == config.AppAntigravityCLI || op.AppID == config.AppAntigravity
	var usyncExtra map[string]any
	if !isAntigravity {
		usyncExtra = config.UsyncMeta(planID, m.Now().UTC().Format(time.RFC3339))
	}

	var updated []byte
	switch op.Kind {
	case config.FileKindMCPServers:
		rootKey := "mcpServers"
		urlFieldName := "url"
		extra := usyncExtra

		switch op.AppID {
		case config.AppAntigravityCLI, config.AppAntigravity, config.AppWindsurf:
			urlFieldName = "serverUrl"
		case config.AppRooCode:
			kindExtra := map[string]any{"type": "stdio"}
			if op.Config.Type != provider.TransportStdio {
				kindExtra = map[string]any{"type": "streamable-http"}
			}
			// merge kindExtra into usyncExtra
			extra = mergeExtra(usyncExtra, kindExtra)
		}

		m.logDebug("updating MCPServers JSON", "app", op.AppID, "rootKey", rootKey, "urlField", urlFieldName)
		updated, err = config.UpdateMCPServersJSON(data, op.ProviderID, rootKey, urlFieldName, op.Config, extra)
	case config.FileKindBareMCPServers:
		urlFieldName := "url"
		if op.AppID == config.AppAntigravityCLI {
			urlFieldName = "serverUrl"
		}
		m.logDebug("updating BareMCPServers JSON", "app", op.AppID, "urlField", urlFieldName)
		updated, err = config.UpdateBareMCPServersJSON(data, op.ProviderID, urlFieldName, op.Config, usyncExtra)
	case config.FileKindNamedServer:
		if op.AppID == config.AppOpenCode {
			updated, err = config.UpdateOpenCodeJSON(data, op.ProviderID, op.Config)
			break
		}
		rootKey := ""
		urlFieldName := "url"
		extra := usyncExtra

		switch op.AppID {
		case config.AppVSCode:
			rootKey = "servers"
			if op.Config.Type != provider.TransportStdio {
				extra = mergeExtra(usyncExtra, map[string]any{"type": "http"})
			}
		case config.AppZed:
			rootKey = "context_servers"
		case config.AppAntigravity:
			// Backward compatibility: Antigravity used to be a named server but now we use FileKindMCPServers
			// with serverUrl if it's nested. If it's a legacy standalone file, this path still works.
			urlFieldName = "serverUrl"
		}
		m.logDebug("updating NamedServer JSON", "app", op.AppID, "rootKey", rootKey, "urlField", urlFieldName)
		updated, err = config.UpdateNamedServerJSON(data, op.ProviderID, rootKey, urlFieldName, op.Config, extra)
	case config.FileKindCodexTOML:
		m.logDebug("updating Codex TOML", "app", op.AppID)
		updated, err = config.UpdateCodexTOML(data, op.ProviderID, op.Config)
	default:
		err = fmt.Errorf("unsupported file operation kind %q", op.Kind)
	}
	if err != nil {
		return preparedWrite{}, fmt.Errorf("%s (%s): %w", op.AppName, op.FileLabel, err)
	}

	m.logDebug("prepared content", "app", op.AppName, "size", len(updated))
	skipped := len(data) > 0 && bytes.Equal(data, updated)
	return preparedWrite{op: op, content: updated, skipped: skipped}, nil
}

// buildCodexCLIAddArgs returns args for `codex mcp add`.
// Stdio: codex mcp add <name> -- <command> [args...]
// HTTP:  codex mcp add --url <url> --bearer-token-env-var <envvar> <name>
func buildCodexCLIAddArgs(providerID string, cfg provider.MCPConfig) []string {
	if cfg.Type == provider.TransportStdio {
		args := []string{"mcp", "add", providerID, "--"}
		args = append(args, cfg.Command)
		return append(args, cfg.Args...)
	}
	args := []string{"mcp", "add", "--url", cfg.URL}
	for k := range cfg.Headers {
		args = append(args, "--bearer-token-env-var", k)
		break // first header key only
	}
	return append(args, providerID)
}

// mergeExtra merges src into dst, returning dst. dst may be nil.
func mergeExtra(dst, src map[string]any) map[string]any {
	if dst == nil {
		dst = make(map[string]any, len(src))
	}
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (m *Manager) rollback(outcomes []config.WriteOutcome, result *ApplyResult) []string {
	warnings := make([]string, 0)
	for index := len(outcomes) - 1; index >= 0; index-- {
		outcome := outcomes[index]
		if err := config.RollbackWrite(outcome); err != nil {
			message := fmt.Sprintf("%s: %s", outcome.Path, redact.Text(err.Error()))
			result.RollbackFailed = append(result.RollbackFailed, message)
			warnings = append(warnings, "rollback failed for "+message)
			m.logError("rollback failed", "path", outcome.Path, "error", err.Error())
			continue
		}
		result.RolledBack = append(result.RolledBack, outcome.Path)
		m.logWarn("rollback restored file", "path", outcome.Path)
	}
	return warnings
}

func verifyFiles(prepared []preparedWrite) []verify.Result {
	// Sort by path for consistent output
	sort.Slice(prepared, func(i, j int) bool {
		return prepared[i].op.Path < prepared[j].op.Path
	})

	results := make([]verify.Result, 0, len(prepared))
	for _, item := range prepared {
		results = append(results, verify.VerifyProviderFile(item.op.Path, item.op.Kind, item.op.ProviderID, item.op.Config))
	}
	return results
}

func validatePathWithinHome(homeDir, target string) error {
	if target == "" {
		return fmt.Errorf("empty target path")
	}

	cleanHome, err := canonicalPath(homeDir)
	if err != nil {
		return fmt.Errorf("resolve home path: %w", err)
	}
	cleanTarget, err := canonicalPath(target)
	if err != nil {
		return fmt.Errorf("resolve target path: %w", err)
	}
	rel, err := filepath.Rel(cleanHome, cleanTarget)
	if err != nil {
		return fmt.Errorf("resolve target path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("target path escapes configured home directory")
	}
	return nil
}

func validateOperationPath(homeDir, scope, target string) error {
	switch scope {
	case "project", "workspace":
		if target == "" {
			return fmt.Errorf("empty target path")
		}
		if _, err := canonicalPath(target); err != nil {
			return fmt.Errorf("resolve target path: %w", err)
		}
		return nil
	default:
		return validatePathWithinHome(homeDir, target)
	}
}

func canonicalPath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	missingSuffix := make([]string, 0, 4)
	current := absPath
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			if len(missingSuffix) == 0 {
				return resolved, nil
			}
			for index := len(missingSuffix) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, missingSuffix[index])
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}

		parent := filepath.Dir(current)
		if parent == current {
			return absPath, nil
		}
		missingSuffix = append(missingSuffix, filepath.Base(current))
		current = parent
	}
}

func rollbackWarnings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	return values
}

func (m *Manager) logDebug(msg string, attrs ...any) {
	m.log(slog.LevelDebug, msg, attrs...)
}

func (m *Manager) logInfo(msg string, attrs ...any) {
	m.log(slog.LevelInfo, msg, attrs...)
}

func (m *Manager) logWarn(msg string, attrs ...any) {
	m.log(slog.LevelWarn, msg, attrs...)
}

func (m *Manager) logError(msg string, attrs ...any) {
	m.log(slog.LevelError, msg, attrs...)
}

func (m *Manager) log(level slog.Level, msg string, attrs ...any) {
	if m.Logger == nil {
		return
	}
	m.Logger.Log(context.Background(), level, redact.Text(msg), redactAttrs(attrs)...)
}

func redactAttrs(attrs []any) []any {
	redacted := make([]any, 0, len(attrs))
	for _, attr := range attrs {
		if value, ok := attr.(string); ok {
			redacted = append(redacted, redact.Text(value))
			continue
		}
		redacted = append(redacted, attr)
	}
	return redacted
}
