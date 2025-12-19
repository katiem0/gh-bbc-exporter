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

var (
	DefaultPartSize           int64  = 100 * 1024 * 1024  // 100 MB
	DefaultMultipartThreshold int64  = 5000 * 1024 * 1024 // 5 GB
	uploadsBaseURL            string = "https://uploads.github.com/organizations/%d/gei/archive"
	uploadsHost               string = "https://uploads.github.com"
)

func (g *APIGetter) UploadArchiveToGitHub(orgID int, archivePath string, logger *zap.Logger) (string, error) {
	logger.Debug("Starting archive upload to GitHub",
		zap.Int("orgID", orgID),
		zap.String("archivePath", archivePath))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	logger.Debug("Opening archive file", zap.String("path", archivePath))
	file, err := os.Open(archivePath)
	if err != nil {
		logger.Debug("Failed to open archive file", zap.Error(err))
		return "", fmt.Errorf("failed to open archive file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil && !strings.Contains(err.Error(), "file already closed") {
			logger.Warn("Failed to close archive file", zap.Error(err))
		}
	}()

	fileInfo, err := file.Stat()
	if err != nil {
		logger.Debug("Failed to get file info", zap.Error(err))
		return "", fmt.Errorf("failed to get file info: %w", err)
	}

	fileName := filepath.Base(archivePath)
	fileSize := fileInfo.Size()

	logger.Info("Archive file info",
		zap.String("name", fileName),
		zap.Int64("size", fileSize))

	logger.Debug("Comparing file size to multipart threshold",
		zap.Int64("fileSize", fileSize),
		zap.Int64("threshold", DefaultMultipartThreshold))

	if fileSize < DefaultMultipartThreshold {
		logger.Info("Using single-file upload (file < 5 GB)")
		logger.Debug("Initiating single-file upload",
			zap.Int64("fileSizeBytes", fileSize),
			zap.String("fileSizeMB", fmt.Sprintf("%.2f MB", float64(fileSize)/(1024*1024))))
		return g.uploadSingleFile(ctx, orgID, file, fileName, fileSize, logger)
	}

	logger.Info("Using multipart upload (file >= 5 GB)")
	logger.Debug("Initiating multipart upload",
		zap.Int64("fileSizeBytes", fileSize),
		zap.String("fileSizeGB", fmt.Sprintf("%.2f GB", float64(fileSize)/(1024*1024*1024))),
		zap.Int64("partSize", DefaultPartSize),
		zap.Int("estimatedParts", int((fileSize+DefaultPartSize-1)/DefaultPartSize)))
	return g.uploadMultipartFile(ctx, orgID, file, fileName, fileSize, logger)
}

func (g *APIGetter) uploadSingleFile(ctx context.Context, orgID int, file *os.File, fileName string, fileSize int64, logger *zap.Logger) (string, error) {
	uploadURL := fmt.Sprintf("%s?name=%s", fmt.Sprintf(uploadsBaseURL, orgID), url.QueryEscape(fileName))

	logger.Debug("Uploading file in single request",
		zap.String("url", uploadURL),
		zap.Int64("size", fileSize),
		zap.Int("orgID", orgID))

	httpClient := &http.Client{
		Timeout: 60 * time.Minute,
	}

	logger.Debug("Creating HTTP POST request for single-file upload")
	req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, file)
	if err != nil {
		logger.Debug("Failed to create HTTP request", zap.Error(err))
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("User-Agent", "gh-bbc-exporter")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", g.authToken))
	req.ContentLength = fileSize

	logger.Debug("Request headers set",
		zap.String("Content-Type", "application/octet-stream"),
		zap.String("User-Agent", "gh-bbc-exporter"),
		zap.Int64("Content-Length", fileSize))

	logger.Debug("Executing HTTP request for upload")
	startTime := time.Now()
	resp, err := httpClient.Do(req)
	uploadDuration := time.Since(startTime)
	if err != nil {
		logger.Debug("Failed to upload file",
			zap.Error(err),
			zap.Duration("duration", uploadDuration))
		return "", fmt.Errorf("failed to upload file: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("Failed to close response body", zap.Error(err))
		}
	}()

	logger.Debug("Received response from upload",
		zap.Int("statusCode", resp.StatusCode),
		zap.Duration("duration", uploadDuration))

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		logger.Debug("Unexpected response status",
			zap.Int("statusCode", resp.StatusCode),
			zap.String("responseBody", string(body)))
		return "", fmt.Errorf("unexpected response status: %d, body: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Debug("Failed to read response body", zap.Error(err))
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	logger.Debug("Parsing upload response", zap.String("responseBody", string(body)))

	var result struct {
		GUID      string `json:"guid"`
		NodeID    string `json:"node_id"`
		Name      string `json:"name"`
		Size      int    `json:"size"`
		URI       string `json:"uri"`
		CreatedAt string `json:"created_at"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		logger.Debug("Failed to decode response", zap.Error(err))
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	logger.Info("Upload successful",
		zap.String("guid", result.GUID),
		zap.String("uri", result.URI))

	logger.Debug("Single-file upload completed",
		zap.String("guid", result.GUID),
		zap.String("nodeID", result.NodeID),
		zap.String("name", result.Name),
		zap.Int("size", result.Size),
		zap.String("uri", result.URI),
		zap.String("createdAt", result.CreatedAt),
		zap.Duration("totalDuration", uploadDuration))

	return result.URI, nil
}

func (g *APIGetter) uploadMultipartFile(ctx context.Context, orgID int, file *os.File, fileName string, fileSize int64, logger *zap.Logger) (string, error) {
	logger.Debug("Starting multipart upload process",
		zap.Int("orgID", orgID),
		zap.String("fileName", fileName),
		zap.Int64("fileSize", fileSize))

	startTime := time.Now()

	// Step 1: Start multipart upload
	logger.Debug("Step 1: Initiating multipart upload session")
	guid, uploadID, nextLocation, err := g.startMultipartUpload(ctx, orgID, fileName, fileSize, logger)
	if err != nil {
		logger.Debug("Failed to start multipart upload", zap.Error(err))
		return "", fmt.Errorf("failed to start multipart upload: %w", err)
	}

	logger.Info("Multipart upload started",
		zap.String("guid", guid),
		zap.String("uploadID", uploadID))

	logger.Debug("Multipart upload session created",
		zap.String("guid", guid),
		zap.String("uploadID", uploadID),
		zap.String("initialLocation", nextLocation))

	// Step 2: Upload parts
	logger.Debug("Step 2: Beginning part uploads")
	partNumber := 1
	var uploadedBytes int64 = 0
	lastLocation := nextLocation
	totalParts := int((fileSize + DefaultPartSize - 1) / DefaultPartSize)

	for uploadedBytes < fileSize {
		// Calculate the size of this part
		currentPartSize := DefaultPartSize
		if fileSize-uploadedBytes < DefaultPartSize {
			currentPartSize = fileSize - uploadedBytes
		}

		logger.Debug("Preparing part for upload",
			zap.Int("partNumber", partNumber),
			zap.Int("totalParts", totalParts),
			zap.Int64("partSize", currentPartSize),
			zap.Int64("uploadedSoFar", uploadedBytes),
			zap.Int64("remaining", fileSize-uploadedBytes))

		// Read the part into memory
		partBuf := make([]byte, currentPartSize)
		n, err := file.Read(partBuf)
		if err != nil && err != io.EOF {
			logger.Debug("Failed to read file part",
				zap.Int("partNumber", partNumber),
				zap.Error(err))
			return "", fmt.Errorf("failed to read file part: %w", err)
		}
		if int64(n) != currentPartSize {
			logger.Debug("Actual bytes read differs from expected",
				zap.Int("expected", int(currentPartSize)),
				zap.Int("actual", n))
			partBuf = partBuf[:n]
		}

		logger.Info("Uploading part",
			zap.Int("part", partNumber),
			zap.Int("bytes", n))

		// Save the previous location for the finalization step
		lastLocation = nextLocation

		// Upload this part
		partStartTime := time.Now()
		nextLocation, err = g.uploadPart(ctx, nextLocation, partBuf, partNumber, logger)
		partDuration := time.Since(partStartTime)
		if err != nil {
			logger.Debug("Failed to upload part",
				zap.Int("partNumber", partNumber),
				zap.Duration("duration", partDuration),
				zap.Error(err))
			return "", fmt.Errorf("failed to upload part %d: %w", partNumber, err)
		}

		uploadedBytes += int64(n)
		logger.Debug("Part uploaded successfully",
			zap.Int("partNumber", partNumber),
			zap.Int("bytesUploaded", n),
			zap.Int64("totalUploaded", uploadedBytes),
			zap.Float64("progressPercent", float64(uploadedBytes)/float64(fileSize)*100),
			zap.Duration("partDuration", partDuration),
			zap.String("nextLocation", nextLocation))

		partNumber++

		// If this is the last part, break the loop
		if uploadedBytes >= fileSize || nextLocation == "" {
			logger.Debug("All parts uploaded",
				zap.Int64("totalBytes", uploadedBytes),
				zap.Int("totalParts", partNumber-1))
			break
		}
	}

	// Step 3: Complete multipart upload
	logger.Debug("Step 3: Finalizing multipart upload",
		zap.String("lastLocation", lastLocation))
	logger.Info("Finalizing upload...")
	uri, err := g.completeMultipartUpload(ctx, lastLocation, logger)
	if err != nil {
		logger.Debug("Failed to complete multipart upload", zap.Error(err))
		return "", fmt.Errorf("failed to complete multipart upload: %w", err)
	}

	totalDuration := time.Since(startTime)
	logger.Info("Multipart upload completed", zap.String("uri", uri))
	logger.Debug("Multipart upload finished",
		zap.String("uri", uri),
		zap.Int64("totalBytes", uploadedBytes),
		zap.Int("totalParts", partNumber-1),
		zap.Duration("totalDuration", totalDuration),
		zap.Float64("avgMBps", float64(uploadedBytes)/(1024*1024)/totalDuration.Seconds()))

	return uri, nil
}

func (g *APIGetter) startMultipartUpload(ctx context.Context, orgID int, fileName string, fileSize int64, logger *zap.Logger) (string, string, string, error) {
	uploadURL := fmt.Sprintf("%s/blobs/uploads", fmt.Sprintf(uploadsBaseURL, orgID))

	logger.Debug("Preparing multipart upload initiation request",
		zap.String("url", uploadURL),
		zap.String("fileName", fileName),
		zap.Int64("fileSize", fileSize))

	body := map[string]interface{}{
		"content_type": "application/octet-stream",
		"name":         fileName,
		"size":         fileSize,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		logger.Debug("Failed to marshal JSON body", zap.Error(err))
		return "", "", "", fmt.Errorf("failed to marshal JSON body: %w", err)
	}

	logger.Debug("Request body prepared", zap.String("body", string(bodyBytes)))

	logger.Debug("Sending multipart upload initiation request via REST client",
		zap.String("method", "POST"),
		zap.String("url", uploadURL))

	resp, err := g.restClient.RequestWithContext(ctx, "POST", uploadURL, bytes.NewReader(bodyBytes))
	if err != nil {
		logger.Debug("Failed to start upload", zap.Error(err))
		return "", "", "", fmt.Errorf("failed to start upload: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("Failed to close response body", zap.Error(err))
		}
	}()

	logger.Debug("Received response for multipart initiation",
		zap.Int("statusCode", resp.StatusCode))

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		logger.Debug("Unexpected response status for multipart initiation",
			zap.Int("statusCode", resp.StatusCode),
			zap.String("responseBody", string(body)))
		return "", "", "", fmt.Errorf("unexpected response status: %d, body: %s", resp.StatusCode, string(body))
	}

	location := resp.Header.Get("Location")
	if location == "" {
		logger.Debug("Missing Location header in response")
		return "", "", "", fmt.Errorf("missing Location header in response")
	}

	logger.Debug("Received Location header", zap.String("location", location))

	guid := ""
	uploadID := ""

	for _, part := range []string{"guid", "upload_id"} {
		parts := strings.Split(location, part+"=")
		if len(parts) > 1 {
			valueParts := strings.Split(parts[1], "&")
			if len(valueParts) > 0 {
				switch part {
				case "guid":
					guid = valueParts[0]
				case "upload_id":
					uploadID = valueParts[0]
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
	uploadURL := uploadsHost + location

	logger.Debug("Uploading part via REST client",
		zap.Int("partNumber", partNumber),
		zap.Int("dataSize", len(data)),
		zap.String("url", uploadURL))

	logger.Debug("Sending part upload request",
		zap.Int("partNumber", partNumber),
		zap.String("method", "PATCH"),
		zap.Int("contentLength", len(data)))

	startTime := time.Now()
	resp, err := g.restClient.RequestWithContext(ctx, "PATCH", uploadURL, bytes.NewReader(data))
	duration := time.Since(startTime)
	if err != nil {
		logger.Debug("Failed to upload part",
			zap.Int("partNumber", partNumber),
			zap.Duration("duration", duration),
			zap.Error(err))
		return "", fmt.Errorf("failed to upload part %d: %w", partNumber, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("Failed to close response body", zap.Error(err))
		}
	}()

	logger.Debug("Received response for part upload",
		zap.Int("partNumber", partNumber),
		zap.Int("statusCode", resp.StatusCode),
		zap.Duration("duration", duration))

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		logger.Debug("Unexpected response status for part upload",
			zap.Int("partNumber", partNumber),
			zap.Int("statusCode", resp.StatusCode),
			zap.String("responseBody", string(body)))
		return "", fmt.Errorf("unexpected response status for part %d: %d, body: %s", partNumber, resp.StatusCode, string(body))
	}

	nextLocation := resp.Header.Get("Location")
	logger.Debug("Part upload completed",
		zap.Int("partNumber", partNumber),
		zap.String("nextLocation", nextLocation),
		zap.Duration("duration", duration))

	return nextLocation, nil
}

func (g *APIGetter) completeMultipartUpload(ctx context.Context, lastLocation string, logger *zap.Logger) (string, error) {
	finalizeURL := uploadsHost + lastLocation

	logger.Debug("Completing multipart upload via REST client",
		zap.String("finalizeURL", finalizeURL))

	logger.Debug("Sending finalize request",
		zap.String("method", "PUT"),
		zap.String("url", finalizeURL))

	startTime := time.Now()
	resp, err := g.restClient.RequestWithContext(ctx, "PUT", finalizeURL, nil)
	duration := time.Since(startTime)
	if err != nil {
		logger.Debug("Failed to finalize upload",
			zap.Duration("duration", duration),
			zap.Error(err))
		return "", fmt.Errorf("failed to finalize upload: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("Failed to close response body", zap.Error(err))
		}
	}()

	logger.Debug("Received response for finalize request",
		zap.Int("statusCode", resp.StatusCode),
		zap.Duration("duration", duration))

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		logger.Debug("Unexpected finalize response status",
			zap.Int("statusCode", resp.StatusCode),
			zap.String("responseBody", string(body)))
		return "", fmt.Errorf("unexpected finalize response status: %d, body: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Debug("Failed to read finalize response body", zap.Error(err))
		return "", fmt.Errorf("failed to read finalize response body: %w", err)
	}

	logger.Debug("Parsing finalize response", zap.String("responseBody", string(body)))

	var result struct {
		GUID      string `json:"guid"`
		NodeID    string `json:"node_id"`
		Name      string `json:"name"`
		Size      int    `json:"size"`
		URI       string `json:"uri"`
		CreatedAt string `json:"created_at"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		logger.Debug("Failed to decode finalize response", zap.Error(err))
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	logger.Debug("Multipart upload finalized",
		zap.String("guid", result.GUID),
		zap.String("nodeID", result.NodeID),
		zap.String("name", result.Name),
		zap.Int("size", result.Size),
		zap.String("uri", result.URI),
		zap.String("createdAt", result.CreatedAt))

	return result.URI, nil
}
