package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nawodyaishan/universal-mcp-sync/pkg/app"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/config"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/exa"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/provider"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/tui"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/validate"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/version"
)

// dashboardManagerAdapter wraps *app.Manager and validate.Service to satisfy tui.DashboardManager.
type dashboardManagerAdapter struct {
	*app.Manager
	validator validate.Service
}

func (a dashboardManagerAdapter) Validate(
	ctx context.Context,
	prov provider.MCPProvider,
	profiles []provider.CredentialProfile,
	live bool,
) (validate.Report, error) {
	return a.validator.ValidateProfiles(ctx, prov, profiles, live)
}

func (a dashboardManagerAdapter) HomeDir() string { return a.Manager.HomeDir }

func loadInitialKeys(keysCSV, keysFile string) ([]string, string, error) {
	if keysCSV != "" {
		keys, err := exa.ParseKeysCSV(keysCSV)
		return keys, keysCSV, err
	}
	if keysFile != "" {
		keys, err := exa.ParseKeysFile(keysFile)
		if err != nil {
			return nil, "", err
		}
		data, err := os.ReadFile(keysFile)
		if err != nil {
			return nil, "", err
		}
		return keys, string(data), nil
	}
	return nil, "", nil
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "sync" {
		args = args[1:]
	}
	if len(args) > 0 {
		switch args[0] {
		case "show":
			return runShow(args[1:], stdout, stderr)
		case "plan":
			return runPlanCommand(args[1:], stdout, stderr)
		case "apply":
			return runApplyCommand(args[1:], stdout, stderr)
		case "validate":
			return runValidateCommand(args[1:], stdout, stderr)
		case "doctor":
			return runDoctorCommand(args[1:], stdout, stderr)
		case "providers":
			return runProvidersCommand(args[1:], stdout, stderr)
		case "replay":
			return runReplayCommand(args[1:], stdout, stderr)
		}
	}

	flags := flag.NewFlagSet("usync", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var keysFile string
	var keysCSV string
	var homeDir string
	var dryRun bool
	var apply bool
	var showVersion bool
	var wizard bool

	var recordPath string
	var recordEnabled bool

	flags.StringVar(&keysFile, "keys-file", "", "path to a file containing Exa API keys")
	flags.StringVar(&keysCSV, "keys", "", "comma-separated Exa API keys")
	flags.StringVar(&homeDir, "home-dir", "", "override the target home directory for testing")
	flags.BoolVar(&dryRun, "dry-run", false, "print the redacted plan without writing files")
	flags.BoolVar(&apply, "apply", false, "apply updates without launching the TUI")
	flags.BoolVar(&showVersion, "version", false, "print version information and exit")
	flags.BoolVar(&wizard, "wizard", false, "launch the legacy setup wizard")
	flags.BoolVar(&recordEnabled, "record", false, "record the interactive session to a JSONL transcript")
	flags.StringVar(&recordPath, "record-path", "", "override the transcript path (default: artifacts/journeys/usync-<ts>.jsonl)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if recordPath != "" {
		recordEnabled = true
	}

	if recordEnabled {
		if apply || dryRun {
			_, _ = fmt.Fprintln(stderr, "--record cannot be combined with --apply or --dry-run (non-interactive modes)")
			return 2
		}
		if keysCSV != "" || keysFile != "" {
			_, _ = fmt.Fprintln(stderr, "--record cannot be combined with --keys or --keys-file (interactive only)")
			return 2
		}
	}

	if showVersion {
		_, _ = fmt.Fprintln(stdout, version.String())
		return 0
	}

	if dryRun && apply {
		_, _ = fmt.Fprintln(stderr, "--dry-run and --apply cannot be used together")
		return 2
	}

	manager, err := app.NewManager(homeDir, nil, nil)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}

	if os.Getenv("USYNC_DEBUG") == "true" {
		_, _ = fmt.Fprintf(stderr, "DEBUG: detected %d apps\n", len(manager.Apps))
		for _, a := range manager.Apps {
			_, _ = fmt.Fprintf(stderr, "DEBUG: app %s id=%s files=%d\n", a.Name, a.ID, len(a.Files))
		}
	}

	initialKeys, initialRaw, err := loadInitialKeys(keysCSV, keysFile)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}

	if !apply && !dryRun {
		var finalModel tea.Model
		var rec *tui.SessionRecorder
		if wizard {
			model := tui.NewWizardModel(manager, initialKeys, initialRaw)
			if recordEnabled {
				var recErr error
				rec, recErr = tui.NewSessionRecorder(recordPath)
				if recErr != nil {
					_, _ = fmt.Fprintf(stderr, "--record: %v\n", recErr)
					return 1
				}
				defer func() { _ = rec.Close() }()
				model = model.WithRecorder(rec)
				_, _ = fmt.Fprintf(stderr, "recording session to %s\n", rec.Path())
			}
			program := tea.NewProgram(model, tea.WithAltScreen())
			finalModel, err = program.Run()
		} else {
			workspaceDir, _ := os.Getwd()
			scanner := tui.NewProductionScanner(homeDir, workspaceDir)
			vs, vsErr := validate.NewService(manager.HomeDir)
			var dashMgr tui.DashboardManager
			if vsErr == nil {
				dashMgr = dashboardManagerAdapter{Manager: manager, validator: vs}
			}
			var dashProfiles []provider.CredentialProfile
			if len(initialKeys) > 0 {
				dashProfiles = []provider.CredentialProfile{{
					ProviderID: "exa",
					Values:     map[string]string{"EXA_API_KEY": initialKeys[0]},
					Label:      exa.RedactKey(initialKeys[0]),
				}}
			}
			model := tui.NewDashboardModelWithWelcome(scanner, dashMgr, dashProfiles)
			if recordEnabled {
				var recErr error
				rec, recErr = tui.NewSessionRecorder(recordPath)
				if recErr != nil {
					_, _ = fmt.Fprintf(stderr, "--record: %v\n", recErr)
					return 1
				}
				defer func() { _ = rec.Close() }()
				model = model.WithRecorder(rec)
				_, _ = fmt.Fprintf(stderr, "recording session to %s\n", rec.Path())
			}
			program := tea.NewProgram(model, tea.WithAltScreen())
			finalModel, err = program.Run()

			if m, ok := finalModel.(tui.DashboardModel); ok && m.RouteToWizard {
				wizardModel := tui.NewWizardModel(manager, initialKeys, initialRaw)
				if rec != nil {
					wizardModel = wizardModel.WithRecorder(rec)
				}
				program := tea.NewProgram(wizardModel, tea.WithAltScreen())
				finalModel, err = program.Run()
			}
		}

		if err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return 1
		}
		if finalTyped, ok := finalModel.(tui.Model); ok && finalTyped.Err() != nil {
			_, _ = fmt.Fprintln(stderr, finalTyped.Err())
			return 1
		}
		return 0
	}

	if len(initialKeys) == 0 {
		_, _ = fmt.Fprintln(stderr, "non-interactive mode requires --keys or --keys-file")
		return 1
	}

	selected := mapAllSelected(manager.Apps)
	if os.Getenv("USYNC_DEBUG") == "true" {
		_, _ = fmt.Fprintf(stderr, "DEBUG: selected %d apps\n", len(selected))
	}
	assignments := app.DefaultAssignments(selected, len(initialKeys))

	plan, err := manager.Prepare(initialKeys, selected, assignments)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}

	if os.Getenv("USYNC_DEBUG") == "true" {
		_, _ = fmt.Fprintf(stderr, "DEBUG: plan has %d operations\n", len(plan.Operations))
	}

	if dryRun {
		_, _ = fmt.Fprintln(stdout, app.FormatPlan(plan))
		return 0
	}

	result, err := manager.Apply(plan)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}

	_, _ = fmt.Fprintln(stdout, app.FormatApplyResult(result))
	return 0
}

func mapAllSelected(apps []config.AppConfig) map[config.AppID]bool {
	selected := make(map[config.AppID]bool, len(apps))
	for _, appConfig := range apps {
		selected[appConfig.ID] = true
	}
	return selected
}
