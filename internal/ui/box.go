package ui

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ansiRegex matches ANSI escape sequences (CSI sequences like \033[38;2;0;255;209m).
// We strip these to measure the visible width of a string — ANSI codes are
// instructions to the terminal, not printable characters.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// visibleLen returns the length of a string after stripping ANSI escape codes.
// This is what the user actually sees — the terminal consumes the escapes
// and only renders the printable characters.
func visibleLen(s string) int {
	return len(ansiRegex.ReplaceAllString(s, ""))
}

// Box renders lines inside a Unicode box-drawing frame.
// Box-drawing characters (─│┌┐└┘) are part of the "Box Drawing" Unicode
// block (U+2500–U+257F). They're supported by virtually every modern terminal.
//
//	┌──────────────────────────┐
//	│  FILE   report.pdf       │
//	│  SIZE   3.2 MB           │
//	└──────────────────────────┘
func Box(lines []string) {
	if IsPiped() {
		return
	}

	// Find the longest *visible* line so we can pad all others to the same width.
	// We use visibleLen() instead of len() because lines may contain ANSI escape
	// codes (e.g., color sequences) that are invisible to the user but add bytes.
	maxVisible := 0
	for _, line := range lines {
		if vl := visibleLen(line); vl > maxVisible {
			maxVisible = vl
		}
	}
	width := maxVisible + 4 // 2 chars padding on each side

	// ─ is U+2500 (horizontal line), repeated to fill the width.
	top := fmt.Sprintf("  %s┌%s┐%s", muted, strings.Repeat("─", width), reset)
	bot := fmt.Sprintf("  %s└%s┘%s", muted, strings.Repeat("─", width), reset)

	fmt.Fprintf(os.Stderr, "%s\n", top)
	for _, line := range lines {
		// Right-pad using visible length so the closing │ aligns correctly
		// even when lines contain ANSI codes of different lengths.
		pad := maxVisible - visibleLen(line)
		padded := line + strings.Repeat(" ", pad)
		fmt.Fprintf(os.Stderr, "  %s│%s  %s  %s│%s\n", muted, reset, padded, muted, reset)
	}
	fmt.Fprintf(os.Stderr, "%s\n", bot)
}

// URLBox renders the share URL in an emphasized box — this is the
// "hero" moment of the CLI. The URL is what the user came for.
func URLBox(url string, expiry string) {
	if IsPiped() {
		// When piped, only the bare URL goes to stdout (already handled by URL()).
		return
	}

	lines := []string{
		fmt.Sprintf("%s%s%s", bold+cyan, url, reset),
		"",
		fmt.Sprintf("%sexpires in %s%s", dim, expiry, reset),
	}

	fmt.Fprintln(os.Stderr)
	Box(lines)
	fmt.Fprintln(os.Stderr)
}

// FileInfoBox renders file metadata in a structured box.
func FileInfoBox(name string, size string, expiry string) {
	if IsPiped() {
		return
	}

	lines := []string{
		fmt.Sprintf("%sFILE%s   %s", muted, reset, name),
		fmt.Sprintf("%sSIZE%s   %s", muted, reset, size),
		fmt.Sprintf("%sEXPIRY%s %s", muted, reset, expiry),
	}

	fmt.Fprintln(os.Stderr)
	Box(lines)
}

// NoteInfoBox renders note metadata in a structured box.
func NoteInfoBox(charCount int, expiry string) {
	if IsPiped() {
		return
	}

	lines := []string{
		fmt.Sprintf("%sNOTE%s   %d characters", muted, reset, charCount),
		fmt.Sprintf("%sEXPIRY%s %s", muted, reset, expiry),
	}

	fmt.Fprintln(os.Stderr)
	Box(lines)
}

// NoteContentBox renders note content in a structured box.
// When stdout is piped by user (e.g., shadowsend get url | pbcopy), outputs bare content.
// Otherwise, shows formatted box to stderr.
func NoteContentBox(charCount int, content string) {
	if IsPiped() {
		// User is piping — output bare content to stdout
		fmt.Print(content)
		return
	}

	// Truncate content if too long for display
	maxDisplay := 500
	displayContent := content
	if len(content) > maxDisplay {
		displayContent = content[:maxDisplay] + "..."
	}

	lines := []string{
		fmt.Sprintf("%sNOTE%s   %d characters", muted, reset, charCount),
		"",
		displayContent,
	}

	fmt.Fprintln(os.Stderr)
	Box(lines)
}
