package app

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nawodyaishan/universal-mcp-sync/pkg/config"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/manifest"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/provider"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/redact"
)

const (
	SavedPlanSchemaVersion = 2

	PlanActionCreate   = "create"
	PlanActionUpdate   = "update"
	PlanActionSkip     = "skip"
	PlanActionConflict = "conflict"

	PlanManagerFile = "file"
	PlanManagerCLI  = "cli"

	PlanCurrentSHAMissing = "missing"

	defaultPlanTTL = 24 * time.Hour
)

type SavedPlan struct {
	SchemaVersion int             `json:"schema_version"`
	PlanID        string          `json:"plan_id"`
	CreatedAt     time.Time       `json:"created_at"`
	ExpiresAt     time.Time       `json:"expires_at"`
	UsyncVersion  string          `json:"usync_version"`
	ProviderID    string          `json:"provider_id"`
	Credentials   []CredentialRef `json:"credential_refs"`
	ContentHash   string          `json:"content_hash,omitempty"` // sha256 of all Redacted strings in order
	Operations    []PlanOperation `json:"operations"`
	Warnings      []string        `json:"warnings,omitempty"`
	DoctorSummary DoctorSummary   `json:"doctor_summary"`
}

type CredentialRef struct {
	ID     string `json:"id,omitempty"`
	Key    string `json:"key"`
	Label  string `json:"label"`
	EnvVar string `json:"env_var"`
}

type PlanOperation struct {
	TargetID      string   `json:"target_id"`
	TargetName    string   `json:"target_name"`
	TargetScope   string   `json:"target_scope,omitempty"`
	Action        string   `json:"action"`
	ProviderID    string   `json:"provider_id,omitempty"`
	CredentialRef string   `json:"credential_ref,omitempty"`
	FileKind      string   `json:"file_kind,omitempty"`
	FilePath      string   `json:"file_path,omitempty"`
	BackupPath    string   `json:"backup_path,omitempty"`
	CurrentSHA    string   `json:"current_sha,omitempty"`
	Transport     string   `json:"transport"`
	Manager       string   `json:"manager"`
	CLICommand    []string `json:"cli_command,omitempty"`
	Redacted      string   `json:"redacted"`
	IsSymlink     bool     `json:"is_symlink"`
	ResolvedPath  string   `json:"resolved_path,omitempty"`
	WillCreate    bool     `json:"will_create,omitempty"`
	GitWarning    bool     `json:"git_warning,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
	// VSCodeInputs holds the input variable definitions to merge into the VS Code
	// mcp.json root-level "inputs" array when UseInputVariables is set.
	VSCodeInputs []config.VSCodeInput `json:"vscode_inputs,omitempty"`
}

type DoctorSummary struct {
	ClientsDetected int      `json:"clients_detected"`
	ClientsReady    int      `json:"clients_ready"`
	Conflicts       int      `json:"conflicts"`
	Warnings        []string `json:"warnings,omitempty"`
}

type SavedPlanOptions struct {
	PlanID            string
	CreatedAt         time.Time
	UsyncVersion      string
	ProviderID        string
	Credentials       []CredentialRef
	Doctor            DoctorSummary
	UseInputVariables bool // emit ${input:id} for VS Code HTTP NamedServer targets (default false)
	UseEnvExpansion   bool // emit ${VAR:-} for Claude Code project-scope targets (default false)
}

func (m *Manager) BuildSavedPlan(plan ExecutionPlan, opts SavedPlanOptions) (SavedPlan, error) {
	if opts.PlanID == "" {
		return SavedPlan{}, fmt.Errorf("missing plan id")
	}
	if opts.ProviderID == "" {
		return SavedPlan{}, fmt.Errorf("missing provider id")
	}
	if opts.CreatedAt.IsZero() {
		return SavedPlan{}, fmt.Errorf("missing plan creation time")
	}

	saved := SavedPlan{
		SchemaVersion: SavedPlanSchemaVersion,
		PlanID:        opts.PlanID,
		CreatedAt:     opts.CreatedAt.UTC(),
		ExpiresAt:     opts.CreatedAt.UTC().Add(defaultPlanTTL),
		UsyncVersion:  opts.UsyncVersion,
		ProviderID:    opts.ProviderID,
		Credentials:   cloneCredentialRefs(opts.Credentials),
		Warnings:      append([]string(nil), plan.Warnings...),
		DoctorSummary: opts.Doctor,
		Operations:    make([]PlanOperation, 0, len(plan.Operations)),
	}

	credentialRefsByLabel := make(map[string]CredentialRef, len(saved.Credentials))
	for _, ref := range saved.Credentials {
		credentialRefsByLabel[ref.Label] = ref
	}

	for _, op := range plan.Operations {
		planOp, err := m.buildPlanOperation(op, credentialRefsByLabel, opts)
		if err != nil {
			return SavedPlan{}, err
		}
		saved.Operations = append(saved.Operations, planOp)
	}

	// Compute plan content integrity hash over all Redacted strings in order.
	h := sha256.New()
	for _, op := range saved.Operations {
		_, _ = io.WriteString(h, op.Redacted)
	}
	saved.ContentHash = "sha256:" + hex.EncodeToString(h.Sum(nil))

	return saved, nil
}

func (m *Manager) buildPlanOperation(op Operation, credentialRefsByLabel map[string]CredentialRef, opts SavedPlanOptions) (PlanOperation, error) {
	credentialRefID := ""
	if ref, ok := credentialRefsByLabel[op.CredentialLabel]; ok {
		credentialRefID = ref.ID
	}

	planOp := PlanOperation{
		TargetID:      string(op.AppID),
		TargetName:    op.AppName,
		TargetScope:   op.Scope,
		ProviderID:    op.ProviderID,
		CredentialRef: credentialRefID,
		FileKind:      string(op.Kind),
		Transport:     string(op.Config.Type),
		GitWarning:    op.GitWarning,
		Warnings:      []string{},
	}

	if op.SkipReason != "" {
		planOp.Action = PlanActionSkip
		planOp.Redacted = redact.Text(fmt.Sprintf("%s: skip %s", op.AppName, op.SkipReason))
		planOp.Warnings = append(planOp.Warnings, op.SkipReason)
	} else if op.Kind == config.FileKindClaudeCodeCLI {
		planOp.Action = PlanActionUpdate
		planOp.Manager = PlanManagerCLI
		planOp.FilePath = op.Path
		planOp.CLICommand = redactStrings(append([]string{"claude"}, op.CLIAddArgs...))
		planOp.Redacted = redact.Text(fmt.Sprintf("%s: update %s [cli, credential=%s]", op.AppName, op.ProviderID, op.CredentialLabel))
		return planOp, nil
	} else {
		planOp.Manager = PlanManagerFile
		planOp.FilePath = op.Path
		planOp.BackupPath = op.BackupPath
		planOp.WillCreate = op.WillCreate
		if op.WillCreate {
			planOp.Action = PlanActionCreate
		} else {
			planOp.Action = PlanActionUpdate
		}
		sha, err := currentSHA(op.Path)
		if err != nil {
			return PlanOperation{}, fmt.Errorf("%s (%s): %w", op.AppName, op.FileLabel, err)
		}
		planOp.CurrentSHA = sha
		isSymlink, resolvedPath, err := symlinkStatus(op.Path)
		if err != nil {
			return PlanOperation{}, fmt.Errorf("%s (%s): %w", op.AppName, op.FileLabel, err)
		}
		planOp.IsSymlink = isSymlink
		planOp.ResolvedPath = resolvedPath
		planOp.Redacted = redact.Text(fmt.Sprintf("%s: %s %s [%s, credential=%s]", op.AppName, planOp.Action, op.ProviderID, op.Config.Type, op.CredentialLabel))
	}

	// VS Code HTTP secret indirection (FR-17): store VSCodeInputs metadata on the plan
	// operation so that buildOperationFromSavedPlan can substitute ${input:id} at apply time
	// and prepareSavedPlan can merge the inputs block into the VS Code config root.
	if opts.UseInputVariables &&
		op.AppID == config.AppVSCode &&
		op.Kind == config.FileKindNamedServer &&
		op.Config.Type != provider.TransportStdio {
		inputID := strings.ToLower(strings.ReplaceAll(op.ProviderID, "_", "-")) + "-api-key"
		planOp.VSCodeInputs = []config.VSCodeInput{{
			Type:        "promptString",
			ID:          inputID,
			Description: op.ProviderID + " API Key",
			Password:    true,
		}}
		planOp.Redacted = redact.Text(fmt.Sprintf(
			"%s: %s %s [vscode-input:${input:%s}]",
			op.AppName, planOp.Action, op.ProviderID, inputID,
		))
	}

	// Claude Code project-scope env expansion (FR-18).
	if opts.UseEnvExpansion &&
		op.AppID == config.AppClaudeCode &&
		op.Scope == string(manifest.ScopeProject) {
		planOp.Redacted = redact.Text(fmt.Sprintf(
			"%s: %s %s [env-expansion, credential=%s]",
			op.AppName, planOp.Action, op.ProviderID, op.CredentialLabel,
		))
	}

	if len(planOp.Warnings) == 0 {
		planOp.Warnings = nil
	}

	return planOp, nil
}

func currentSHA(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		sum := sha256.Sum256(data)
		return "sha256:" + hex.EncodeToString(sum[:]), nil
	}
	if os.IsNotExist(err) {
		return PlanCurrentSHAMissing, nil
	}
	return "", fmt.Errorf("read target for hash %s: %w", path, err)
}

func symlinkStatus(path string) (bool, string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, "", nil
		}
		return false, "", fmt.Errorf("lstat %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return false, "", nil
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return true, "", fmt.Errorf("resolve symlink %s: %w", path, err)
	}
	return true, resolved, nil
}

func cloneCredentialRefs(values []CredentialRef) []CredentialRef {
	if len(values) == 0 {
		return nil
	}
	out := make([]CredentialRef, len(values))
	copy(out, values)
	for i := range out {
		if out[i].ID == "" {
			out[i].ID = defaultCredentialRefID(out[i].Key, out[i].Label)
		}
	}
	return out
}

func defaultCredentialRefID(key, label string) string {
	if key == "" {
		return label
	}
	if label == "" {
		return key
	}
	return key + ":" + label
}

func redactStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = redact.Text(value)
	}
	return out
}
