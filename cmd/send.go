package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pankajsattadhish/shadowsend-cli/internal/api"
	"github.com/pankajsattadhish/shadowsend-cli/internal/crypto"
	"github.com/pankajsattadhish/shadowsend-cli/internal/ui"
)

func runSend(args []string) {
	if len(args) == 0 {
		ui.Error("NO_FILE_SPECIFIED")
		fmt.Fprintf(os.Stderr, "Usage: shadowsend send <file> [--expiry 1h|6h|24h]\n")
		os.Exit(1)
	}

	filePath := args[0]
	expiryHours := 24 // default

	// Parse --expiry flag
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
		}
	}

	// Step tracker gives the user a sense of the full pipeline.
	// Each Start() call auto-completes the previous step.
	steps := ui.NewSteps([]string{
		"READ FILE",
		"ENCRYPT",
		"TRANSMIT",
		"CONFIRM",
	})

	// ── Step 1: Read ──
	steps.Start("READING FILE")
	data, err := os.ReadFile(filePath)
	if err != nil {
		ui.Error(fmt.Sprintf("FAILED_TO_READ_FILE: %s", err))
		os.Exit(1)
	}

	fileName := filepath.Base(filePath)
	fileSize := int64(len(data))

	// Show file metadata in a box — this is the first visual "wow" moment.
	steps.Complete("READ FILE")
	ui.FileInfoBox(fileName, formatSize(fileSize), fmt.Sprintf("%dH", expiryHours))
	fmt.Fprintln(os.Stderr)

	// ── Step 2: Encrypt ──
	steps.Start("ENCRYPTING — AES-256-GCM STREAMING")
	key, err := crypto.GenerateKey()
	if err != nil {
		ui.Error(fmt.Sprintf("KEY_GENERATION_FAILED: %s", err))
		os.Exit(1)
	}

	// Use streaming encryption for memory efficiency
	ciphertext, err := crypto.EncryptStream(data, key)
	if err != nil {
		ui.Error(fmt.Sprintf("ENCRYPTION_FAILED: %s", err))
		os.Exit(1)
	}

	keyString := crypto.ExportKey(key)

	// ── Step 3: Upload ──
	steps.Start("INITIATING TRANSMISSION")

	baseURL := os.Getenv("SHADOWSEND_API_URL")
	client := api.New(baseURL)

	initResp, err := client.InitUpload(fileName, fileSize, expiryHours)
	if err != nil {
		ui.Error(fmt.Sprintf("TRANSMISSION_INIT_FAILED: %s", err))
		os.Exit(1)
	}

	steps.Complete("INITIATED")

	// Wrap the ciphertext in a ProgressReader so the upload shows a live bar.
	// bytes.NewReader implements io.Reader; ProgressReader wraps it.
	ciphertextLen := int64(len(ciphertext))
	pr := ui.NewProgressReader(bytes.NewReader(ciphertext), ciphertextLen, "TRANSMITTING")
	if err := client.UploadToStorage(initResp.UploadURL, initResp.Token, pr, ciphertextLen); err != nil {
		ui.Error(fmt.Sprintf("TRANSMISSION_FAILED: %s", err))
		os.Exit(1)
	}
	pr.Finish()

	// ── Step 4: Confirm ──
	steps.Start("CONFIRMING")
	if err := client.ConfirmUpload(initResp.ID, fileName, fileSize, expiryHours); err != nil {
		ui.Error(fmt.Sprintf("CONFIRMATION_FAILED: %s", err))
		os.Exit(1)
	}
	steps.Complete("CONFIRMED")

	// Build share URL and present it — the hero moment.
	shareURL := fmt.Sprintf("%s/f/%s#%s", client.BaseURL, initResp.ID, keyString)

	ui.Success(fmt.Sprintf("TRANSMISSION COMPLETE — %s", steps.Elapsed()))

	// Pipe-friendly: bare URL to stdout, decorated box to stderr.
	ui.URL(shareURL)
	ui.URLBox(shareURL, fmt.Sprintf("%dh", expiryHours))
}

func formatSize(bytes int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
