package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const DefaultBaseURL = "https://shadowsend.com"

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func New(baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// InitUploadResponse is the response from POST /api/upload.
type InitUploadResponse struct {
	ID        string `json:"id"`
	UploadURL string `json:"upload_url"`
	Token     string `json:"token"`
}

// FileMetadata is the response from GET /api/file/[id].
type FileMetadata struct {
	ID        string `json:"id"`
	FileName  string `json:"file_name"`
	FileSize  int64  `json:"file_size"`
	ExpiresAt string `json:"expires_at"`
	CreatedAt string `json:"created_at"`
}

// InitUpload requests a presigned upload URL from the server.
func (c *Client) InitUpload(fileName string, fileSize int64, expiryHours int) (*InitUploadResponse, error) {
	body, err := json.Marshal(map[string]interface{}{
		"file_name":    fileName,
		"file_size":    fileSize,
		"expiry_hours": expiryHours,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal body: %w", err)
	}

	req, err := http.NewRequest("POST", c.BaseURL+"/api/upload", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-ShadowSend-Client", "cli")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("upload init failed (HTTP %d)", resp.StatusCode)
	}

	var result InitUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// UploadToStorage uploads encrypted data directly to Supabase Storage via presigned URL.
// The body parameter is an io.Reader — this lets callers wrap it with a progress
// tracker (e.g., ui.ProgressReader) without changing this function's internals.
// contentLength is needed because HTTP PUT with presigned URLs requires Content-Length.
func (c *Client) UploadToStorage(uploadURL string, token string, body io.Reader, contentLength int64) error {
	req, err := http.NewRequest("PUT", uploadURL, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.ContentLength = contentLength
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("storage upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("storage upload failed (HTTP %d)", resp.StatusCode)
	}

	return nil
}

// ConfirmUpload creates the DB record after a successful storage upload.
func (c *Client) ConfirmUpload(id, fileName string, fileSize int64, expiryHours int) error {
	body, err := json.Marshal(map[string]interface{}{
		"id":           id,
		"file_name":    fileName,
		"file_size":    fileSize,
		"expiry_hours": expiryHours,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal body: %w", err)
	}

	req, err := http.NewRequest("POST", c.BaseURL+"/api/upload/confirm", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-ShadowSend-Client", "cli")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("confirm request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("confirm failed (HTTP %d)", resp.StatusCode)
	}

	return nil
}

// GetFileMetadata fetches file info from the server.
func (c *Client) GetFileMetadata(id string) (*FileMetadata, error) {
	resp, err := c.HTTPClient.Get(c.BaseURL + "/api/file/" + id)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 410 {
		return nil, fmt.Errorf("TRANSMISSION_EXPIRED: DATA PURGED")
	}
	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("TRANSMISSION_NOT_FOUND")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get file info (HTTP %d)", resp.StatusCode)
	}

	var meta FileMetadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &meta, nil
}

// DownloadResponse holds the download stream and its size, allowing
// callers to wrap the body with a progress reader before consuming it.
type DownloadResponse struct {
	Body          io.ReadCloser
	ContentLength int64
}

// DownloadFile starts downloading the encrypted blob from the server.
// It returns a DownloadResponse so the caller can wrap Body with a progress
// reader. The caller is responsible for closing Body.
func (c *Client) DownloadFile(id string) (*DownloadResponse, error) {
	resp, err := c.HTTPClient.Get(c.BaseURL + "/api/download/" + id)
	if err != nil {
		return nil, fmt.Errorf("download request failed: %w", err)
	}

	if resp.StatusCode == 410 {
		resp.Body.Close()
		return nil, fmt.Errorf("TRANSMISSION_EXPIRED: DATA PURGED")
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, fmt.Errorf("download failed (HTTP %d)", resp.StatusCode)
	}

	return &DownloadResponse{
		Body:          resp.Body,
		ContentLength: resp.ContentLength,
	}, nil
}
