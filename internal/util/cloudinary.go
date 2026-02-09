package util

import (
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"yourapp/internal/config"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/google/uuid"
)

const (
	tmpDirName = "tmp"
)

type CloudinaryClient struct {
	cld *cloudinary.Cloudinary
	cfg *config.Config
}

func NewCloudinaryClient(cfg *config.Config) (*CloudinaryClient, error) {
	if cfg.CloudinaryCloudName == "" || cfg.CloudinaryAPIKey == "" || cfg.CloudinaryAPISecret == "" {
		return nil, fmt.Errorf("cloudinary credentials not configured")
	}

	cld, err := cloudinary.NewFromParams(cfg.CloudinaryCloudName, cfg.CloudinaryAPIKey, cfg.CloudinaryAPISecret)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cloudinary: %w", err)
	}

	return &CloudinaryClient{
		cld: cld,
		cfg: cfg,
	}, nil
}

// CompressImage compresses an image file and saves to tmp directory
func (c *CloudinaryClient) CompressImage(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	var img image.Image
	ext := strings.ToLower(filepath.Ext(filePath))

	if ext == ".jpg" || ext == ".jpeg" {
		img, err = jpeg.Decode(file)
		if err != nil {
			return "", fmt.Errorf("error decoding JPEG: %w", err)
		}
	} else if ext == ".png" {
		img, err = png.Decode(file)
		if err != nil {
			return "", fmt.Errorf("error decoding PNG: %w", err)
		}
	} else if ext == ".webp" {
		// For WebP, upload directly without compression
		return filePath, nil
	} else {
		return "", fmt.Errorf("unsupported image format: %s", ext)
	}

	// Ensure tmp directory exists
	tmpDir, err := ensureTmpDir()
	if err != nil {
		return "", err
	}

	// Create compressed file in tmp directory
	compressedPath := filepath.Join(tmpDir, uuid.New().String()+".compressed.jpg")
	compressedFile, err := os.Create(compressedPath)
	if err != nil {
		return "", fmt.Errorf("error creating compressed file: %w", err)
	}
	defer compressedFile.Close()

	err = jpeg.Encode(compressedFile, img, &jpeg.Options{Quality: 80})
	if err != nil {
		return "", fmt.Errorf("error encoding compressed image: %w", err)
	}

	return compressedPath, nil
}

// UploadImage uploads an image file to Cloudinary (delivered as WebP)
func (c *CloudinaryClient) UploadImage(filePath string) (string, error) {
	ctx := context.Background()

	result, err := c.cld.Upload.Upload(ctx, filePath, uploader.UploadParams{
		Folder:         c.cfg.CloudinaryFolder,
		Transformation: "q_auto,f_webp,w_1280", // WebP format, auto quality, max width 1280
		ResourceType:   "image",
	})

	if err != nil {
		return "", fmt.Errorf("error uploading to cloudinary: %w", err)
	}

	// Inject transformation into URL so image is served as WebP
	url := result.SecureURL
	url = strings.Replace(url, "/upload/", "/upload/f_webp,q_auto,w_1280/", 1)
	return url, nil
}

// ensureTmpDir ensures the tmp directory exists
func ensureTmpDir() (string, error) {
	// Get current working directory or use relative path
	wd, err := os.Getwd()
	if err != nil {
		// Fallback to temp directory if can't get working directory
		tmpDir := filepath.Join(os.TempDir(), tmpDirName)
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create tmp directory: %w", err)
		}
		return tmpDir, nil
	}

	tmpDir := filepath.Join(wd, tmpDirName)
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create tmp directory: %w", err)
	}
	return tmpDir, nil
}

// ProcessFileFromMemory processes a file from memory (binary data)
func (c *CloudinaryClient) ProcessFileFromMemory(fileData []byte, filename string) (string, error) {
	// Ensure tmp directory exists
	tmpDir, err := ensureTmpDir()
	if err != nil {
		return "", err
	}

	// Create temporary file in tmp directory
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".jpg"
	}
	tempFile := filepath.Join(tmpDir, uuid.New().String()+ext)

	err = os.WriteFile(tempFile, fileData, 0644)
	if err != nil {
		return "", fmt.Errorf("error writing temp file: %w", err)
	}
	defer os.Remove(tempFile) // Clean up temp file

	// Compress
	compressedPath, err := c.CompressImage(tempFile)
	if err != nil {
		// If compression fails, use original
		compressedPath = tempFile
	} else {
		if compressedPath != tempFile {
			defer os.Remove(compressedPath) // Clean up compressed file
		}
	}

	// Upload to Cloudinary
	imageURL, err := c.UploadImage(compressedPath)
	if err != nil {
		return "", err
	}

	return imageURL, nil
}

// ProcessMultipleFiles processes multiple files from memory
func (c *CloudinaryClient) ProcessMultipleFiles(files []FileData) ([]string, error) {
	var imageURLs []string

	for _, fileData := range files {
		imageURL, err := c.ProcessFileFromMemory(fileData.Data, fileData.Filename)
		if err != nil {
			// Log error but continue processing other files
			fmt.Printf("Error processing file %s: %v\n", fileData.Filename, err)
			continue
		}
		imageURLs = append(imageURLs, imageURL)
	}

	if len(imageURLs) == 0 {
		return nil, fmt.Errorf("no images were successfully processed")
	}

	return imageURLs, nil
}

// UploadVideo uploads a video file to Cloudinary with compression transformations
// Cloudinary handles the compression server-side via eager transformations.
func (c *CloudinaryClient) UploadVideo(filePath string) (string, error) {
	ctx := context.Background()

	result, err := c.cld.Upload.Upload(ctx, filePath, uploader.UploadParams{
		Folder:       c.cfg.CloudinaryFolder + "/videos",
		ResourceType: "video",
	})

	if err != nil {
		return "", fmt.Errorf("error uploading video to cloudinary: %w", err)
	}

	// Inject transformation into URL for optimized delivery:
	// compress, convert to mp4/h264, limit resolution to 720p, auto quality
	url := result.SecureURL
	url = strings.Replace(url, "/upload/", "/upload/c_limit,w_1280,h_720,q_auto,f_mp4,vc_h264/", 1)
	return url, nil
}

// ProcessVideoFromMemory processes a video file from memory (saves to tmp, uploads to Cloudinary)
func (c *CloudinaryClient) ProcessVideoFromMemory(fileData []byte, filename string) (string, error) {
	// Ensure tmp directory exists
	tmpDir, err := ensureTmpDir()
	if err != nil {
		return "", err
	}

	// Create temporary file in tmp directory
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".mp4"
	}
	tempFile := filepath.Join(tmpDir, uuid.New().String()+ext)

	err = os.WriteFile(tempFile, fileData, 0644)
	if err != nil {
		return "", fmt.Errorf("error writing temp video file: %w", err)
	}
	defer os.Remove(tempFile) // Clean up temp file

	// Upload to Cloudinary (compression handled by Cloudinary transformations)
	videoURL, err := c.UploadVideo(tempFile)
	if err != nil {
		return "", err
	}

	return videoURL, nil
}

// ProcessMultipleVideos processes multiple video files from memory
func (c *CloudinaryClient) ProcessMultipleVideos(files []FileData) ([]string, error) {
	var videoURLs []string

	for _, fileData := range files {
		videoURL, err := c.ProcessVideoFromMemory(fileData.Data, fileData.Filename)
		if err != nil {
			fmt.Printf("Error processing video file %s: %v\n", fileData.Filename, err)
			continue
		}
		videoURLs = append(videoURLs, videoURL)
	}

	if len(videoURLs) == 0 {
		return nil, fmt.Errorf("no videos were successfully processed")
	}

	return videoURLs, nil
}

// FileData represents file data in memory
type FileData struct {
	Data     []byte
	Filename string
	MimeType string
}

// ReadFileFromReader reads file data from an io.Reader
func ReadFileFromReader(reader io.Reader, filename string) (*FileData, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	// Detect MIME type from extension
	mimeType := "image/jpeg"
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".png":
		mimeType = "image/png"
	case ".webp":
		mimeType = "image/webp"
	case ".gif":
		mimeType = "image/gif"
	case ".mp4":
		mimeType = "video/mp4"
	case ".mov":
		mimeType = "video/quicktime"
	case ".avi":
		mimeType = "video/x-msvideo"
	case ".webm":
		mimeType = "video/webm"
	case ".mkv":
		mimeType = "video/x-matroska"
	case ".3gp":
		mimeType = "video/3gpp"
	}

	return &FileData{
		Data:     data,
		Filename: filename,
		MimeType: mimeType,
	}, nil
}

// IsVideoFile checks if a filename represents a video file based on extension
func IsVideoFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".mp4", ".mov", ".avi", ".webm", ".mkv", ".3gp":
		return true
	}
	return false
}

// GetFileExt returns the lowercase file extension including the dot (e.g. ".mp4")
func GetFileExt(filename string) string {
	return strings.ToLower(filepath.Ext(filename))
}
