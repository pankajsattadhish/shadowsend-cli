package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
)

const barWidth = 30 // number of characters in the bar (█ and ░)

// ProgressReader wraps an io.Reader and renders a progress bar to stderr
// as data flows through it. This is the "decorator pattern" — it adds
// behavior (progress tracking) without modifying the underlying reader.
//
// Usage:
//
//	pr := ui.NewProgressReader(body, totalSize, "UPLOADING")
//	io.Copy(destination, pr)  // progress bar updates automatically
//	pr.Finish()               // prints the completed line
type ProgressReader struct {
	reader  io.Reader
	total   int64
	current int64
	label   string
}

// NewProgressReader wraps an existing reader with progress tracking.
// total is the expected number of bytes (used to calculate percentage).
// label is what shows next to the bar (e.g., "UPLOADING", "DOWNLOADING").
func NewProgressReader(r io.Reader, total int64, label string) *ProgressReader {
	return &ProgressReader{
		reader: r,
		total:  total,
		label:  label,
	}
}

// Read implements io.Reader. Every time the consumer reads a chunk,
// we update the progress bar. This is why the decorator pattern works
// so well here — the caller just does io.Copy() as normal.
func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.current += int64(n)

	if !IsPiped() && pr.total > 0 {
		pr.render()
	}

	return n, err
}

// render draws the progress bar on the current line.
// Called on every Read(), but since terminal writes are fast and
// reads happen in chunks (typically 32KB), this isn't a bottleneck.
func (pr *ProgressReader) render() {
	ratio := float64(pr.current) / float64(pr.total)
	if ratio > 1.0 {
		ratio = 1.0
	}

	filled := int(ratio * barWidth)
	empty := barWidth - filled

	// ██ for filled, ░ for empty — these are full-width block characters.
	// The percentage and byte count give precise info alongside the visual.
	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	pct := int(ratio * 100)

	fmt.Fprintf(os.Stderr, "\r  %s%s%s %s%s%s %s%3d%% %s/%s%s",
		cyan, bar, reset,
		muted, pr.label, reset,
		dim, pct, formatBytes(pr.current), formatBytes(pr.total), reset,
	)
}

// Finish prints the completed bar and moves to the next line.
func (pr *ProgressReader) Finish() {
	if IsPiped() {
		return
	}
	// Draw one final full bar, then newline to lock it in place.
	bar := strings.Repeat("█", barWidth)
	fmt.Fprintf(os.Stderr, "\r\033[2K  %s%s%s %s%s %s%s\n",
		cyan, bar, reset,
		bold, pr.label, formatBytes(pr.total), reset,
	)
}

// formatBytes converts bytes to human-readable form.
// Duplicated from cmd/ intentionally — ui package shouldn't import cmd.
func formatBytes(b int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1fGB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1fMB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1fKB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%dB", b)
	}
}
