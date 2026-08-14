package ui

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// Non-terminal output degrades to plain text: sections, lines, summary,
// and the spinner fallback all render labels without a live terminal.
func TestRunPlainRendering(t *testing.T) {
	var buf bytes.Buffer
	r := NewRun(&buf)

	r.Section("Scanning system")
	r.Line("%-40s %s", "User cache", "1.2 GB")
	r.Faint("This is a DRY RUN")
	r.Summary("Potential space to free: 1.2 GB")
	r.Spinner("Cleaning User cache", func() error { return nil })

	out := buf.String()
	for _, want := range []string{
		"Scanning system",
		"User cache",
		"1.2 GB",
		"This is a DRY RUN",
		"Potential space to free: 1.2 GB",
		"Cleaning User cache...",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("plain output missing %q:\n%s", want, out)
		}
	}
}

func TestRunSpinnerRunsFn(t *testing.T) {
	var buf bytes.Buffer
	ran := false
	r := NewRun(&buf)
	err := r.Spinner("Working", func() error {
		ran = true
		return nil
	})
	if !ran {
		t.Error("spinner did not run the work function")
	}
	if err != nil {
		t.Errorf("spinner returned unexpected error: %v", err)
	}
}

func TestRunSpinnerPropagatesError(t *testing.T) {
	var buf bytes.Buffer
	want := errors.New("boom")
	r := NewRun(&buf)
	got := r.Spinner("Working", func() error { return want })
	if got != want {
		t.Errorf("spinner error = %v, want %v", got, want)
	}
}
