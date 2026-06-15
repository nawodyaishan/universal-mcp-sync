package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/teatest"
	"github.com/nawodyaishan/universal-mcp-sync/pkg/doctor"
)

func TestDashboardTeatest(t *testing.T) {
	scanner := &FakeScanner{
		Report: doctor.Report{Platform: "test-platform"},
	}

	model := NewDashboardModel(scanner, nil, nil)
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(80, 24))

	var capturedOutput string

	// Wait for the scan to finish using the shared helper; capture the output.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		s := string(bts)
		if strings.Contains(s, "System Status") {
			capturedOutput = s
			return true
		}
		return false
	}, teatest.WithDuration(time.Second*3))

	if !strings.Contains(capturedOutput, "System Status") {
		t.Errorf("expected 'System Status' in output, got %q", capturedOutput)
	}
	if !strings.Contains(capturedOutput, "No AI clients detected") {
		t.Errorf("expected 'No AI clients detected' in output, got %q", capturedOutput)
	}

	tm.Type("q")
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}
