package provider

import "testing"

func TestCodeGraphProvider_Metadata(t *testing.T) {
	p := NewCodeGraphProvider()

	if p.ID() != "codegraph" {
		t.Errorf("expected ID codegraph, got %q", p.ID())
	}
	if p.Name() != "CodeGraph" {
		t.Errorf("expected name CodeGraph, got %q", p.Name())
	}
	if p.Description() == "" {
		t.Error("expected non-empty description")
	}
	if len(p.RequiredCredentials()) != 0 {
		t.Fatalf("expected no required credentials (CodeGraph is fully local), got %d", len(p.RequiredCredentials()))
	}
}

func TestCodeGraphProvider_GenerateConfig(t *testing.T) {
	p := NewCodeGraphProvider()

	cfg, err := p.GenerateConfig(nil)
	if err != nil {
		t.Fatalf("GenerateConfig returned error: %v", err)
	}

	if cfg.Type != TransportStdio {
		t.Errorf("expected TransportStdio, got %s", cfg.Type)
	}
	if cfg.Command != "npx" {
		t.Errorf("expected command npx, got %q", cfg.Command)
	}
	if len(cfg.Args) != 3 || cfg.Args[0] != "-y" || cfg.Args[1] != "@colbymchenry/codegraph" || cfg.Args[2] != "mcp" {
		t.Errorf("expected args [-y @colbymchenry/codegraph mcp], got %v", cfg.Args)
	}
	if len(cfg.Env) != 0 {
		t.Errorf("expected no env vars (no credentials needed), got %v", cfg.Env)
	}
	if len(cfg.Headers) != 0 {
		t.Errorf("expected no headers, got %v", cfg.Headers)
	}
	if cfg.URL != "" {
		t.Errorf("expected no URL for stdio provider, got %q", cfg.URL)
	}
	if cfg.Runtime == nil || cfg.Runtime.Type != "npm" {
		t.Errorf("expected npm runtime, got %v", cfg.Runtime)
	}
}
