package ui

import (
	"fmt"
	"os"
)

// ANSI color codes
const (
	cyan  = "\033[38;2;0;255;209m" // #00FFD1 — accent
	red   = "\033[38;2;255;68;68m" // #ff4444 — danger
	muted = "\033[38;2;85;85;85m"  // #555555 — muted
	dim   = "\033[2m"
	bold  = "\033[1m"
	reset = "\033[0m"
)

// IsPiped returns true if stdout is not a terminal (output is being piped).
func IsPiped() bool {
	fi, _ := os.Stdout.Stat()
	return (fi.Mode() & os.ModeCharDevice) == 0
}

// Status prints a status message in muted style.
func Status(msg string) {
	if IsPiped() {
		return
	}
	fmt.Fprintf(os.Stderr, "%s%s%s\n", muted, msg, reset)
}

// Progress prints a progress/action message in cyan.
func Progress(msg string) {
	if IsPiped() {
		return
	}
	fmt.Fprintf(os.Stderr, "%s%s%s\n", cyan, msg, reset)
}

// Success prints a success message in cyan + bold.
func Success(msg string) {
	if IsPiped() {
		return
	}
	fmt.Fprintf(os.Stderr, "%s%s%s%s\n", bold, cyan, msg, reset)
}

// Error prints an error message in red.
func Error(msg string) {
	fmt.Fprintf(os.Stderr, "%s%sERROR: %s%s\n", bold, red, msg, reset)
}

// URL prints the share URL — this goes to stdout (for piping).
func URL(url string) {
	if IsPiped() {
		fmt.Print(url)
	} else {
		fmt.Fprintf(os.Stdout, "\n%s%s%s%s\n\n", bold, cyan, url, reset)
	}
}

// Banner prints the ShadowSend branding with a vertical gradient.
// The gradient shifts from bright cyan (#00FFD1) at the top to a deeper
// teal (#00B8A9) at the bottom. Each line gets its own RGB color via
// 24-bit ANSI escape: \033[38;2;R;G;Bm
// This only works on terminals that support "truecolor" (most modern ones do).
func Banner() {
	if IsPiped() {
		return
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  %s╔══ ShadowSend ═══╗%s\n", cyan, reset)
	fmt.Fprintf(os.Stderr, "  %s║ encrypted file  %s║%s\n", muted, " ", reset)
	fmt.Fprintf(os.Stderr, "  %s║ sharing that    %s║%s\n", muted, " ", reset)
	fmt.Fprintf(os.Stderr, "  %s║ self-destructs  %s║%s\n", muted, " ", reset)
	fmt.Fprintf(os.Stderr, "  %s╚═════════════════╝%s\n\n", cyan, reset)
}

// UpdateHint prints a subtle update notification.
func UpdateHint(latest string) {
	if IsPiped() {
		return
	}
	fmt.Fprintf(os.Stderr, "%sUPDATE_AVAILABLE: %s — run 'shadowsend update' to install%s\n", muted, latest, reset)
}

// FileInfo prints file metadata.
func FileInfo(name string, size string, expiry string) {
	if IsPiped() {
		return
	}
	fmt.Fprintf(os.Stderr, "%sFILE: %s%s\n", muted, reset, name)
	fmt.Fprintf(os.Stderr, "%sSIZE: %s%s\n", muted, reset, size)
	fmt.Fprintf(os.Stderr, "%sEXPIRY: %s%s\n", muted, reset, expiry)
}
