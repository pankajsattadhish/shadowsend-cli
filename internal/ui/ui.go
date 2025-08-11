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

	lines := []string{
		"  ██████╗ ██╗  ██╗███╗   ██╗████████╗███╗   ███╗",
		"  ██╔══██╗██║  ██║████╗  ██║╚══██╔══╝████╗ ████║",
		"  ██████╔╝███████║██╔██╗ ██║   ██║   ██╔████╔██║",
		"  ██╔═══╝ ██╔══██║██║╚██╗██║   ██║   ██║╚██╔╝██║",
		"  ██║     ██║  ██║██║ ╚████║   ██║   ██║ ╚═╝ ██║",
		"  ╚═╝     ╚═╝  ╚═╝╚═╝  ╚═══╝   ╚═╝   ╚═╝     ╚═╝",
	}

	// Gradient: top is R=0, G=255, B=209 (#00FFD1)
	//           bot is R=0, G=184, B=169 (#00B8A9)
	// We interpolate G and B across the lines. R stays 0.
	fmt.Fprintln(os.Stderr)
	for i, line := range lines {
		// t goes from 0.0 (first line) to 1.0 (last line)
		t := float64(i) / float64(len(lines)-1)
		g := int(255 - t*71) // 255 → 184
		b := int(209 - t*40) // 209 → 169
		color := fmt.Sprintf("\033[38;2;0;%d;%dm", g, b)
		fmt.Fprintf(os.Stderr, "%s%s%s\n", color, line, reset)
	}
	fmt.Fprintf(os.Stderr, "%s  encrypted file sharing that self-destructs%s\n\n", muted, reset)
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
