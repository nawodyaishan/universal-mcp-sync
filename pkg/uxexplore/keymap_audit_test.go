package uxexplore

import (
	"path/filepath"
	"testing"
)

func TestParseHandlerKeys_ExtractsLiteralCases(t *testing.T) {
	path := filepath.Join("..", "tui", "dashboard.go")
	handlers, err := ParseHandlerKeys(path)
	if err != nil {
		t.Fatalf("ParseHandlerKeys: %v", err)
	}
	for _, screen := range []string{"Welcome", "Doctor", "ProviderReady", "TargetSelect", "ConflictResolve", "CredentialEntry", "PlanPreview", "ApplyResult"} {
		if _, ok := handlers[screen]; !ok {
			t.Errorf("handler %s missing from AST scan", screen)
		}
	}
	// Doctor accepts r, w, p, enter.
	if _, ok := handlers["Doctor"]["r"]; !ok {
		t.Errorf("Doctor must accept \"r\"")
	}
}

func TestAuditKeymap_EmptyTracesEmitsKeysFromHandlersOnly(t *testing.T) {
	handlers := HandlerKeyMap{
		"Test": {"a": {}, "b": {}},
	}
	findings := AuditKeymap(handlers, nil)
	// With no traces, every non-global handler key is unadvertised.
	if len(findings) != 2 {
		t.Fatalf("want 2 unadvertised, got %d", len(findings))
	}
	for _, f := range findings {
		if f.Kind != FindingUnadvertisedKey {
			t.Errorf("kind = %v", f.Kind)
		}
	}
}
