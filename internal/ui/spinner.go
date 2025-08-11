package ui

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Braille dot frames — each is a single-width Unicode character.
// They cycle smoothly because each frame differs by one dot from the next,
// creating a "rotating" illusion at ~80ms per frame.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner animates a status message on a single line.
// It uses \r (carriage return) to overwrite the same line each frame,
// giving the illusion of animation without scrolling the terminal.
type Spinner struct {
	mu      sync.Mutex
	msg     string    // current message displayed next to the spinner
	running bool      // is the animation goroutine active?
	done    chan struct{} // signals the goroutine to stop
}

// NewSpinner creates a spinner but does NOT start it yet.
// We separate creation from starting so you can reuse one spinner
// across multiple steps (stop one message, start another).
func NewSpinner() *Spinner {
	return &Spinner{}
}

// Start begins the animation with the given message.
// If piped, it just prints the message once (no animation).
func (s *Spinner) Start(msg string) {
	if IsPiped() {
		fmt.Fprintf(os.Stderr, "%s\n", msg)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// If already running, just update the message — don't spawn another goroutine.
	if s.running {
		s.msg = msg
		return
	}

	s.msg = msg
	s.running = true
	s.done = make(chan struct{})

	go s.animate()
}

// Stop halts the animation and prints a final message on that line.
// The \r clears the spinner frame, and \n locks the line in place
// so the next output appears below it.
func (s *Spinner) Stop(finalMsg string) {
	if IsPiped() {
		return
	}

	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.done)
	s.mu.Unlock()

	// Clear the line and print the final message.
	// \033[2K erases the entire current line (more reliable than padding with spaces).
	fmt.Fprintf(os.Stderr, "\r\033[2K  %s%s%s %s%s\n", bold, cyan, "✓", finalMsg, reset)
}

// animate is the goroutine that cycles through spinner frames.
// It runs until Stop() closes the done channel.
func (s *Spinner) animate() {
	frame := 0
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.mu.Lock()
			msg := s.msg
			s.mu.Unlock()

			// \r returns cursor to column 0. We then overwrite with the current frame.
			// The spinner character is cyan, the message is muted — creates visual hierarchy.
			fmt.Fprintf(os.Stderr, "\r  %s%s%s %s%s%s",
				cyan, spinnerFrames[frame%len(spinnerFrames)], reset,
				muted, msg, reset,
			)

			frame++
		}
	}
}
