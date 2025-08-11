package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pankajsattadhish/shadowsend-cli/internal/ui"
	"github.com/pankajsattadhish/shadowsend-cli/internal/updater"
)

var version = "0.1.2"

func Execute() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	command := os.Args[1]

	// Background update check for non-update commands
	if command != "update" && command != "version" && command != "--version" {
		updater.CheckForUpdateQuietly(version)
		// Give the goroutine a moment to print if cached
		time.Sleep(10 * time.Millisecond)
	}

	switch command {
	case "send":
		runSend(os.Args[2:])
	case "get":
		runGet(os.Args[2:])
	case "note":
		runNote(os.Args[2:])
	case "update":
		updater.RunUpdate(version)
	case "version", "--version", "-v":
		fmt.Printf("shadowsend %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		// If it looks like a file path, treat it as `send <file>`
		if !strings.HasPrefix(command, "-") {
			runSend(os.Args[1:])
		} else {
			ui.Error(fmt.Sprintf("UNKNOWN_COMMAND: %s", command))
			printUsage()
			os.Exit(1)
		}
	}
}

func printUsage() {
	ui.Banner()

	// Section headers get the accent color, commands stay default (white).
	// This creates visual hierarchy without being noisy.
	c := "\033[38;2;0;255;209m" // cyan accent
	m := "\033[38;2;85;85;85m"  // muted
	r := "\033[0m"              // reset

	fmt.Fprintf(os.Stderr, `%sUSAGE%s
  shadowsend send <file> [--expiry 1h|6h|24h]    Encrypt & upload a file
  shadowsend get <url>                            Download & decrypt a file
  shadowsend note <text> [--expiry 1h|6h|24h]    Encrypt & upload a text note
  shadowsend update                               Check for updates and self-update
  shadowsend <file>                               Shorthand for send
  shadowsend version                              Print version

%sEXAMPLES%s
  shadowsend send report.pdf                      Upload with 24h expiry (default)
  shadowsend send logs.tar.gz --expiry 1h         Upload with 1h expiry
  shadowsend get https://shadowsend.com/f/abc123#key    Download & decrypt
  shadowsend report.pdf | pbcopy                  Upload & copy link to clipboard
  shadowsend note "API key: sk-123"               Upload a text note
  echo "secret" | shadowsend note                 Pipe text to upload

  %s── pankajsattadhish@gmail.com · https://shadowsend.com ──%s

`, c, r, c, r, m, r)
}
