package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
)

const (
	DefaultPartSize           int64 = 100 * 1024 * 1024  // 100 MB
	DefaultMultipartThreshold int64 = 5000 * 1024 * 1024 // 5 GB
	uploadsBaseURL                  = "https://uploads.github.com/organizations/%d/gei/archive"
)

func (g *APIGetter) UploadArchiveToGitHub(orgID int, archivePath string, logger *zap.Logger) (string, error) {
	// Create context with timeout (60 minutes default)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	file, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("failed to open archive file: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to get file info: %w", err)
	}

	fileName := filepath.Base(archivePath)
	fileSize := fileInfo.Size()

	logger.Info("Archive file info",
		zap.String("name", fileName),
		zap.Int64("size", fileSize))

	if fileSize < DefaultMultipartThreshold {
		logger.Info("Using single-file upload (file < 5 GB)")
		return g.uploadSingleFile(ctx, orgID, file, fileName, fileSize, logger)
	}

	logger.Info("Using multipart upload (file >= 5 GB)")
	return g.uploadMultipartFile(ctx, orgID, file, fileName, fileSize, logger)
}

func (g *APIGetter) uploadSingleFile(ctx context.Context, orgID int, file *os.File, fileName string, fileSize int64, logger *zap.Logger) (string, error) {
	uploadURL := fmt.Sprintf("%s?name=%s", fmt.Sprintf(uploadsBaseURL, orgID), url.QueryEscape(fileName))

	logger.Debug("Uploading file in single request",
		zap.String("url", uploadURL),
		zap.Int64("size", fileSize))

	// Create HTTP client for direct upload
	httpClient := &http.Client{
		Timeout: 60 * time.Minute,
	}

	req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, file)
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("User-Agent", "gh-bbc-exporter")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("GH_PAT")))
	req.ContentLength = fileSize

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected response status: %d, body: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var result struct {
		GUID      string `json:"guid"`
		NodeID    string `json:"node_id"`
		Name      string `json:"name"`
		Size      int    `json:"size"`
		URI       string `json:"uri"`
		CreatedAt string `json:"created_at"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	logger.Info("Upload successful",
		zap.String("guid", result.GUID),
		zap.String("uri", result.URI))

	return result.URI, nil
}

func (g *APIGetter) uploadMultipartFile(ctx context.Context, orgID int, file *os.File, fileName string, fileSize int64, logger *zap.Logger) (string, error) {
	// Step 1: Start multipart upload
	guid, uploadID, nextLocation, err := g.startMultipartUpload(ctx, orgID, fileName, fileSize, logger)
	if err != nil {
		return "", fmt.Errorf("failed to start multipart upload: %w", err)
	}

	logger.Info("Multipart upload started",
		zap.String("guid", guid),
		zap.String("uploadID", uploadID))

	// Step 2: Upload parts
	partNumber := 1
	var uploadedBytes int64 = 0
	var lastLocation string = nextLocation

	for uploadedBytes < fileSize {
		// Calculate the size of this part
		currentPartSize := DefaultPartSize
		if fileSize-uploadedBytes < DefaultPartSize {
			currentPartSize = fileSize - uploadedBytes
		}

		// Read the part into memory
		partBuf := make([]byte, currentPartSize)
		n, err := file.Read(partBuf)
		if err != nil && err != io.EOF {
			return "", fmt.Errorf("failed to read file part: %w", err)
		}
		if int64(n) != currentPartSize {
			partBuf = partBuf[:n]
		}

		logger.Info("Uploading part",
			zap.Int("part", partNumber),
			zap.Int("bytes", n))

		// Save the previous location for the finalization step
		lastLocation = nextLocation

		// Upload this part
		nextLocation, err = g.uploadPart(ctx, nextLocation, partBuf, partNumber, logger)
		if err != nil {
			return "", fmt.Errorf("failed to upload part %d: %w", partNumber, err)
		}

		uploadedBytes += int64(n)
		partNumber++

		// If this is the last part, break the loop
		if uploadedBytes >= fileSize || nextLocation == "" {
			break
		}
	}

	// Step 3: Complete multipart upload
	logger.Info("Finalizing upload...")
	uri, err := g.completeMultipartUpload(ctx, lastLocation, logger)
	if err != nil {
		return "", fmt.Errorf("failed to complete multipart upload: %w", err)
	}

	logger.Info("Multipart upload completed", zap.String("uri", uri))

	return uri, nil
}

func (g *APIGetter) startMultipartUpload(ctx context.Context, orgID int, fileName string, fileSize int64, logger *zap.Logger) (string, string, string, error) {
	uploadURL := fmt.Sprintf("%s/blobs/uploads", fmt.Sprintf(uploadsBaseURL, orgID))

	body := map[string]interface{}{
		"content_type": "application/octet-stream",
		"name":         fileName,
		"size":         fileSize,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to marshal JSON body: %w", err)
	}

	// Create HTTP client for direct upload
	httpClient := &http.Client{
		Timeout: 60 * time.Minute,
	}

	req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "gh-bbc-exporter")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("GH_PAT")))
	req.Header.Set("GraphQL-Features", "octoshift_github_owned_storage")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to start upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return "", "", "", fmt.Errorf("unexpected response status: %d, body: %s", resp.StatusCode, string(body))
	}

	// Get the Location header from the response
	location := resp.Header.Get("Location")
	if location == "" {
		return "", "", "", fmt.Errorf("missing Location header in response")
	}

	// Parse out the guid and upload_id from location
	// Location format: /organizations/{org_id}/gei/archive/blobs/uploads?part_number=1&guid=<guid>&upload_id=<upload_id>
	guid := ""
	uploadID := ""

	for _, part := range []string{"guid", "upload_id"} {
		parts := strings.Split(location, part+"=")
		if len(parts) > 1 {
			parts = strings.Split(parts[1], "&")
			if len(parts) > 0 {
				if part == "guid" {
					guid = parts[0]
				} else if part == "upload_id" {
					uploadID = parts[0]
				}
			}
		}
	}

	logger.Debug("Upload started",
		zap.String("guid", guid),
		zap.String("uploadID", uploadID),
		zap.String("location", location))

	return guid, uploadID, location, nil
}

func (g *APIGetter) uploadPart(ctx context.Context, location string, data []byte, partNumber int, logger *zap.Logger) (string, error) {
	uploadURL := "https://uploads.github.com" + location

	// Create HTTP client for direct upload
	httpClient := &http.Client{
		Timeout: 60 * time.Minute,
	}

	req, err := http.NewRequestWithContext(ctx, "PATCH", uploadURL, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to create PATCH request: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("User-Agent", "gh-bbc-exporter")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("GH_PAT")))
	req.Header.Set("GraphQL-Features", "octoshift_github_owned_storage")
	req.ContentLength = int64(len(data))

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to upload part %d: %w", partNumber, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected response status for part %d: %d, body: %s", partNumber, resp.StatusCode, string(body))
	}

	// Get the next location from the response header
	nextLocation := resp.Header.Get("Location")

	return nextLocation, nil
}

func (g *APIGetter) completeMultipartUpload(ctx context.Context, lastLocation string, logger *zap.Logger) (string, error) {
	finalizeURL := "https://uploads.github.com" + lastLocation

	// Create HTTP client for direct upload
	httpClient := &http.Client{
		Timeout: 60 * time.Minute,
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", finalizeURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create finalize request: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("User-Agent", "gh-bbc-exporter")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("GH_PAT")))
	req.Header.Set("GraphQL-Features", "octoshift_github_owned_storage")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to finalize upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected finalize response status: %d, body: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read finalize response body: %w", err)
	}

	var result struct {
		GUID      string `json:"guid"`
		NodeID    string `json:"node_id"`
		Name      string `json:"name"`
		Size      int    `json:"size"`
		URI       string `json:"uri"`
		CreatedAt string `json:"created_at"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.URI, nil
}
