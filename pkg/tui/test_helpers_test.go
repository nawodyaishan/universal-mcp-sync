package tui

import (
	"bytes"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/teatest"
)

func waitForText(t *testing.T, tm *teatest.TestModel, text string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte(text))
	}, teatest.WithDuration(2*time.Second), teatest.WithCheckInterval(25*time.Millisecond))
}

func waitForAll(t *testing.T, tm *teatest.TestModel, values ...string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		for _, v := range values {
			if !bytes.Contains(b, []byte(v)) {
				return false
			}
		}
		return true
	}, teatest.WithDuration(2*time.Second), teatest.WithCheckInterval(25*time.Millisecond))
}
