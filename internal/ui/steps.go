package ui

import (
	"fmt"
	"os"
	"time"
)

// Steps tracks a sequence of named operations, showing which are
// done (✓), which is active (spinner-like), and which are pending.
//
// The visual pattern:
//
//	✓ Read file
//	✓ Encrypt
//	⠸ Upload          ← currently active (would animate, but we keep it simple)
//	◦ Confirm          ← pending
//
// We intentionally keep this simple: no in-place multi-line rewriting
// (which requires terminal height detection and cursor movement escape codes).
// Instead, each step prints on its own line as it completes. The current
// step uses the spinner.
type Steps struct {
	steps   []string
	current int
	spinner *Spinner
	started time.Time // tracks total elapsed time
}

// NewSteps creates a step tracker with the given step names.
func NewSteps(steps []string) *Steps {
	return &Steps{
		steps:   steps,
		current: -1, // nothing started yet
		spinner: NewSpinner(),
		started: time.Now(),
	}
}

// Start begins the next step. If there's a previous step running,
// it gets marked as complete first.
func (s *Steps) Start(stepName string) {
	if IsPiped() {
		fmt.Fprintf(os.Stderr, "%s...\n", stepName)
		return
	}

	// Complete the previous step if one was running
	if s.current >= 0 {
		s.spinner.Stop(s.steps[s.current])
	}

	s.current++
	s.spinner.Start(stepName)
}

// Complete marks the current step as done and stops the spinner.
func (s *Steps) Complete(msg string) {
	if IsPiped() {
		return
	}
	if msg == "" && s.current >= 0 {
		msg = s.steps[s.current]
	}
	s.spinner.Stop(msg)
}

// Elapsed returns the time since the first step started.
// Displayed at the end for that polished "completed in 1.2s" feel.
func (s *Steps) Elapsed() string {
	d := time.Since(s.started)
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
