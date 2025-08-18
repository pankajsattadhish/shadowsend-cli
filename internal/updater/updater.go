package updater

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/pankajsattadhish/shadowsend-cli/internal/ui"
)

const (
	repo          = "pankajsattadhish/shadowsend-cli"
	checkFile     = ".last_update_check"
	checkInterval = 24 * time.Hour
)

type githubRelease struct {
	TagName string `json:"tag_name"`
}

func configDir() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".config", "shadowsend")
	os.MkdirAll(dir, 0755)
	return dir
}

// GetLatestVersion fetches the latest release tag from GitHub.
func GetLatestVersion() (string, error) {
	resp, err := http.Get(fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	return release.TagName, nil
}

// CheckForUpdateQuietly runs a non-blocking version check once per day.
// Shows a subtle hint if a new version is available.
func CheckForUpdateQuietly(currentVersion string) {
	if ui.IsPiped() {
		return
	}

	checkPath := filepath.Join(configDir(), checkFile)

	// Check if we already checked recently
	if info, err := os.Stat(checkPath); err == nil {
		if time.Since(info.ModTime()) < checkInterval {
			// Read cached result
			data, err := os.ReadFile(checkPath)
			if err == nil {
				latest := strings.TrimSpace(string(data))
				if latest != "" && latest != "v"+currentVersion && latest != currentVersion {
					ui.UpdateHint(latest)
				}
			}
			return
		}
	}

	// Check in background — don't slow down the command
	go func() {
		latest, err := GetLatestVersion()
		if err != nil {
			return
		}
		// Cache the result
		os.WriteFile(checkPath, []byte(latest), 0600)

		if latest != "v"+currentVersion && latest != currentVersion {
			ui.UpdateHint(latest)
		}
	}()
}

// RunUpdate checks for a new version and self-updates if available.
func RunUpdate(currentVersion string) {
	ui.Status("CHECKING_FOR_UPDATES...")

	latest, err := GetLatestVersion()
	if err != nil {
		ui.Error(fmt.Sprintf("VERSION_CHECK_FAILED: %s", err))
		os.Exit(1)
	}

	currentTag := "v" + currentVersion
	if latest == currentTag || latest == currentVersion {
		ui.Success(fmt.Sprintf("ALREADY_UP_TO_DATE: %s", currentVersion))
		return
	}

	ui.Progress(fmt.Sprintf("UPDATE_AVAILABLE: %s -> %s", currentVersion, latest))

	// Ask for permission
	fmt.Fprintf(os.Stderr, "\nInstall update? [y/N] ")
	var answer string
	fmt.Scanln(&answer)
	answer = strings.TrimSpace(strings.ToLower(answer))

	if answer != "y" && answer != "yes" {
		ui.Status("UPDATE_CANCELLED")
		return
	}

	// Determine platform
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	version := strings.TrimPrefix(latest, "v")
	tarball := fmt.Sprintf("shadowsend_%s_%s_%s.tar.gz", version, goos, goarch)
	downloadURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, latest, tarball)

	ui.Progress("DOWNLOADING_UPDATE...")

	// Download to temp
	tmpDir, err := os.MkdirTemp("", "shadowsend-update-*")
	if err != nil {
		ui.Error(fmt.Sprintf("TEMP_DIR_FAILED: %s", err))
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	tarPath := filepath.Join(tmpDir, tarball)
	resp, err := http.Get(downloadURL)
	if err != nil {
		ui.Error(fmt.Sprintf("DOWNLOAD_FAILED: %s", err))
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		ui.Error(fmt.Sprintf("DOWNLOAD_FAILED: HTTP %d", resp.StatusCode))
		os.Exit(1)
	}

	out, err := os.Create(tarPath)
	if err != nil {
		ui.Error(fmt.Sprintf("WRITE_FAILED: %s", err))
		os.Exit(1)
	}
	io.Copy(out, resp.Body)
	out.Close()

	// Verify SHA256 checksum before extracting
	ui.Progress("VERIFYING_CHECKSUM...")
	checksumURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/checksums.txt", repo, latest)
	if err := verifyChecksum(tarPath, tarball, checksumURL); err != nil {
		ui.Error(fmt.Sprintf("CHECKSUM_FAILED: %s", err))
		os.Exit(1)
	}

	// Extract with safe tar handling
	ui.Progress("EXTRACTING...")
	if err := extractTarGz(tarPath, tmpDir); err != nil {
		ui.Error(fmt.Sprintf("EXTRACT_FAILED: %s", err))
		os.Exit(1)
	}

	// Find current binary path
	execPath, err := os.Executable()
	if err != nil {
		ui.Error(fmt.Sprintf("CANNOT_LOCATE_BINARY: %s", err))
		os.Exit(1)
	}
	execPath, _ = filepath.EvalSymlinks(execPath)

	newBinary := filepath.Join(tmpDir, "shadowsend")

	// Replace binary
	ui.Progress("INSTALLING...")
	if err := replaceBinary(execPath, newBinary); err != nil {
		ui.Error(fmt.Sprintf("INSTALL_FAILED: %s — try running with sudo", err))
		os.Exit(1)
	}

	// Clear cached check
	os.Remove(filepath.Join(configDir(), checkFile))

	ui.Success(fmt.Sprintf("UPDATED: %s -> %s", currentVersion, latest))
}

func replaceBinary(dest, src string) error {
	// Guard: ensure dest is a regular file, not a symlink
	destInfo, err := os.Lstat(dest)
	if err != nil {
		return fmt.Errorf("cannot stat %s: %w", dest, err)
	}
	if destInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to replace symlink at %s", dest)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Try direct write first
	destFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		// If permission denied, try with temp file + rename
		// O_EXCL ensures atomic creation — fails if file already exists (prevents symlink attacks)
		tmpDest := dest + ".new"
		destFile, err = os.OpenFile(tmpDest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0755)
		if err != nil {
			return fmt.Errorf("cannot write to %s: %w", dest, err)
		}
		if _, err := io.Copy(destFile, srcFile); err != nil {
			destFile.Close()
			os.Remove(tmpDest)
			return err
		}
		destFile.Close()
		return os.Rename(tmpDest, dest)
	}

	if _, err := io.Copy(destFile, srcFile); err != nil {
		destFile.Close()
		return err
	}
	destFile.Close()
	return os.Chmod(dest, 0755)
}

func verifyChecksum(filePath, fileName, checksumURL string) error {
	// Download checksums.txt from the release
	resp, err := http.Get(checksumURL)
	if err != nil {
		return fmt.Errorf("failed to fetch checksums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("checksums not available (HTTP %d)", resp.StatusCode)
	}

	// Find the expected hash for our tarball
	var expectedHash string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		// Format: "sha256hash  filename"
		parts := strings.Fields(scanner.Text())
		if len(parts) == 2 && parts[1] == fileName {
			expectedHash = parts[0]
			break
		}
	}
	if expectedHash == "" {
		return fmt.Errorf("no checksum found for %s", fileName)
	}

	// Compute SHA256 of the downloaded file
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("failed to hash file: %w", err)
	}
	actualHash := hex.EncodeToString(h.Sum(nil))

	if actualHash != expectedHash {
		return fmt.Errorf("hash mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

func extractTarGz(tarPath, destDir string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip error: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar error: %w", err)
		}

		// Sanitize: use only the base name, reject paths with ..
		clean := filepath.Clean(header.Name)
		if strings.Contains(clean, "..") {
			return fmt.Errorf("unsafe path in tar: %s", header.Name)
		}

		target := filepath.Join(destDir, filepath.Base(clean))

		switch header.Typeflag {
		case tar.TypeReg:
			out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode)&0755)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		case tar.TypeDir:
			// skip directories — we only need the binary
			continue
		case tar.TypeSymlink, tar.TypeLink:
			// reject symlinks entirely — known attack vector
			return fmt.Errorf("unsafe entry in tar: symlink %s", header.Name)
		}
	}
	return nil
}

