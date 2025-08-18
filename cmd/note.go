package cmd

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/pankajsattadhish/shadowsend-cli/internal/api"
	"github.com/pankajsattadhish/shadowsend-cli/internal/crypto"
	"github.com/pankajsattadhish/shadowsend-cli/internal/ui"
)

func runNote(args []string) {
	expiryHours := 24 // default
	var textContent string

	// Parse arguments
	for i, arg := range args {
		if arg == "--expiry" && i+1 < len(args) {
			switch args[i+1] {
			case "1h":
				expiryHours = 1
			case "6h":
				expiryHours = 6
			case "24h":
				expiryHours = 24
			default:
				ui.Error(fmt.Sprintf("INVALID_EXPIRY: %s (use 1h, 6h, or 24h)", args[i+1]))
				os.Exit(1)
			}
			break
		}
	}

	// Get text content from args or stdin
	if len(args) > 0 && !strings.HasPrefix(args[0], "--") {
		// Text provided as argument
		textContent = strings.Join(args, " ")
		// Remove --expiry flag from content if present
		if idx := strings.Index(textContent, "--expiry"); idx > 0 {
			textContent = strings.TrimSpace(textContent[:idx])
		}
	} else {
		// Read from stdin
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			// Piped input
			buf := new(bytes.Buffer)
			_, err := buf.ReadFrom(os.Stdin)
			if err != nil {
				ui.Error(fmt.Sprintf("FAILED_TO_READ_STDIN: %s", err))
				os.Exit(1)
			}
			textContent = buf.String()
		} else {
			ui.Error("NO_TEXT_PROVIDED")
			fmt.Fprintf(os.Stderr, "Usage: shadowsend note <text> [--expiry 1h|6h|24h]\n")
			fmt.Fprintf(os.Stderr, "       echo \"text\" | shadowsend note\n")
			os.Exit(1)
		}
	}

	// Check size limit (10KB)
	const maxSize = 10 * 1024
	if len(textContent) > maxSize {
		ui.Error(fmt.Sprintf("TEXT_TOO_LARGE: %d bytes (max %d)", len(textContent), maxSize))
		os.Exit(1)
	}

	if strings.TrimSpace(textContent) == "" {
		ui.Error("EMPTY_TEXT")
		os.Exit(1)
	}

	// Step tracker
	steps := ui.NewSteps([]string{
		"ENCRYPT",
		"TRANSMIT",
		"CONFIRM",
	})

	// Show note metadata
	steps.Start("ENCRYPTING — AES-256-GCM")
	ui.NoteInfoBox(len(textContent), fmt.Sprintf("%dH", expiryHours))
	fmt.Fprintln(os.Stderr)

	// Step 1: Encrypt
	key, err := crypto.GenerateKey()
	if err != nil {
		ui.Error(fmt.Sprintf("KEY_GENERATION_FAILED: %s", err))
		os.Exit(1)
	}

	// Use streaming encryption (consistent with files)
	ciphertext, err := crypto.EncryptStream([]byte(textContent), key)
	if err != nil {
		ui.Error(fmt.Sprintf("ENCRYPTION_FAILED: %s", err))
		os.Exit(1)
	}

	keyString := crypto.ExportKey(key)
	steps.Complete("ENCRYPTED")

	// Step 2: Upload
	steps.Start("INITIATING TRANSMISSION")

	baseURL := os.Getenv("SHADOWSEND_API_URL")
	client := api.New(baseURL)

	// Note stored as note.txt
	fileName := "note.txt"
	fileSize := int64(len(ciphertext))

	initResp, err := client.InitUpload(fileName, fileSize, expiryHours)
	if err != nil {
		ui.Error(fmt.Sprintf("TRANSMISSION_INIT_FAILED: %s", err))
		os.Exit(1)
	}

	steps.Complete("INITIATED")

	ciphertextLen := int64(len(ciphertext))
	pr := ui.NewProgressReader(bytes.NewReader(ciphertext), ciphertextLen, "TRANSMITTING")
	if err := client.UploadToStorage(initResp.UploadURL, initResp.Token, pr, ciphertextLen); err != nil {
		ui.Error(fmt.Sprintf("TRANSMISSION_FAILED: %s", err))
		os.Exit(1)
	}
	pr.Finish()

	// Step 3: Confirm
	steps.Start("CONFIRMING")
	if err := client.ConfirmUpload(initResp.ID, fileName, fileSize, expiryHours); err != nil {
		ui.Error(fmt.Sprintf("CONFIRMATION_FAILED: %s", err))
		os.Exit(1)
	}
	steps.Complete("CONFIRMED")

	// Build share URL
	shareURL := fmt.Sprintf("%s/f/%s#%s", client.BaseURL, initResp.ID, keyString)

	ui.Success(fmt.Sprintf("TRANSMISSION COMPLETE — %s", steps.Elapsed()))

	// Pipe-friendly: bare URL to stdout, decorated box to stderr
	ui.URL(shareURL)
	ui.URLBox(shareURL, fmt.Sprintf("%dh", expiryHours))
}

