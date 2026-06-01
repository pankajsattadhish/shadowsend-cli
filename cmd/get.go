package cmd

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pankajsattadhish/shadowsend-cli/internal/api"
	"github.com/pankajsattadhish/shadowsend-cli/internal/crypto"
	"github.com/pankajsattadhish/shadowsend-cli/internal/ui"
)

func runGet(args []string) {
	if len(args) == 0 {
		ui.Error("NO_URL_SPECIFIED")
		fmt.Fprintf(os.Stderr, "Usage: shadowsend get <url>\n")
		os.Exit(1)
	}

	rawURL := args[0]

	// Parse the URL to extract file ID and key
	parsed, err := url.Parse(rawURL)
	if err != nil {
		ui.Error(fmt.Sprintf("INVALID_URL: %s", err))
		os.Exit(1)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		ui.Error("INVALID_URL: URL must start with http or https")
		os.Exit(1)
	}

	// Extract file ID from path: /f/{id}
	pathParts := strings.Split(strings.TrimPrefix(parsed.Path, "/"), "/")
	if len(pathParts) < 2 || pathParts[0] != "f" {
		ui.Error("INVALID_URL: expected format https://shadow-send.vercel.app/f/{id}#{key}")
		os.Exit(1)
	}
	fileID := pathParts[1]
	matched, _ := regexp.MatchString("^[a-zA-Z0-9]+$", fileID)
	if !matched {
		ui.Error("INVALID_FILE_ID: file ID must be alphanumeric")
		os.Exit(1)
	}

	// Extract key from fragment
	keyString := parsed.Fragment
	if keyString == "" {
		ui.Error("MISSING_DECRYPTION_KEY: URL must contain #key fragment")
		os.Exit(1)
	}

	// Determine API base URL from the share URL
	baseURL := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
	envURL := os.Getenv("SHADOWSEND_API_URL")
	if envURL != "" {
		baseURL = envURL
	}

	client := api.New(baseURL)

	steps := ui.NewSteps([]string{
		"LOCATE",
		"DOWNLOAD",
		"DECRYPT",
		"SAVE",
	})

	// ── Step 1: Fetch metadata ──
	steps.Start("LOCATING TRANSMISSION")
	meta, err := client.GetFileMetadata(fileID)
	if err != nil {
		ui.Error(err.Error())
		os.Exit(1)
	}
	steps.Complete("LOCATED")

	ui.FileInfoBox(meta.FileName, formatSize(meta.FileSize), meta.ExpiresAt)
	fmt.Fprintln(os.Stderr)

	// ── Step 2: Download with progress ──
	steps.Start("DOWNLOADING CIPHERTEXT")
	dlResp, err := client.DownloadFile(fileID)
	if err != nil {
		ui.Error(fmt.Sprintf("DOWNLOAD_FAILED: %s", err))
		os.Exit(1)
	}
	defer dlResp.Body.Close()

	// Wrap the response body with a progress reader.
	// ContentLength comes from the HTTP Content-Length header.
	// If the server doesn't send it (-1), the bar won't show percentage
	// but the download still works.
	pr := ui.NewProgressReader(dlResp.Body, dlResp.ContentLength, "DOWNLOADING")

	const maxDownloadSize = 5 << 30 // 5 GB
	ciphertext, err := io.ReadAll(io.LimitReader(pr, maxDownloadSize+1))
	if err != nil {
		ui.Error(fmt.Sprintf("DOWNLOAD_FAILED: %s", err))
		os.Exit(1)
	}
	if int64(len(ciphertext)) > maxDownloadSize {
		ui.Error("file exceeds maximum size of 5GB")
		os.Exit(1)
	}
	pr.Finish()

	// ── Step 3: Decrypt ──
	steps.Start("DECRYPTING — AES-256-GCM")
	key, err := crypto.ImportKey(keyString)
	if err != nil {
		ui.Error(fmt.Sprintf("INVALID_KEY: %s", err))
		os.Exit(1)
	}

	// Auto-detect format (streaming or legacy)
	plaintext, err := crypto.DecryptAuto(ciphertext, key)
	if err != nil {
		ui.Error(fmt.Sprintf("DECRYPTION_FAILED: %s", err))
		os.Exit(1)
	}

	// ── Step 4: Save to disk ──
	steps.Start("SAVING")
	outputPath := filepath.Base(meta.FileName)
	if outputPath == "." || outputPath == "/" || outputPath == ".." {
		outputPath = "shadowsend_download"
	}
	// If file already exists, increment a counter until we find a free name.
	// Handles the case where both report.pdf and report_1.pdf already exist.
	if _, err := os.Stat(outputPath); err == nil {
		ext := filepath.Ext(outputPath)
		base := strings.TrimSuffix(outputPath, ext)
		for i := 1; ; i++ {
			candidate := fmt.Sprintf("%s_%d%s", base, i, ext)
			if _, err := os.Stat(candidate); os.IsNotExist(err) {
				outputPath = candidate
				break
			}
		}
	}

	if err := os.WriteFile(outputPath, plaintext, 0644); err != nil {
		ui.Error(fmt.Sprintf("WRITE_FAILED: %s", err))
		os.Exit(1)
	}
	steps.Complete("SAVED")

	// ── Step 5: If .txt file, print to stdout ──
	isNote := strings.HasSuffix(strings.ToLower(meta.FileName), ".txt")
	if isNote {
		ui.NoteContentBox(len(plaintext), string(plaintext))
		fmt.Fprintln(os.Stderr)
	}

	ui.Success(fmt.Sprintf("DECRYPTION COMPLETE — %s → %s", steps.Elapsed(), outputPath))
}
