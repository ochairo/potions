package gateways

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ochairo/potions/internal/domain/entities"
)

// Downloader handles downloading artifacts from URLs
type Downloader struct {
	httpClient *http.Client
}

// NewDownloader creates a new downloader
func NewDownloader() *Downloader {
	return &Downloader{
		httpClient: &http.Client{
			Timeout: 5 * time.Minute, // Long timeout for large downloads
		},
	}
}

// DownloadArtifact downloads an artifact based on recipe and platform
func (d *Downloader) DownloadArtifact(def *entities.Recipe, version, platform, outputDir string) (*entities.Artifact, error) {
	// Get platform config
	platformConfig, exists := def.Download.Platforms[platform]
	if !exists {
		return nil, fmt.Errorf("platform %s not supported", platform)
	}

	// Build download URL with template substitution
	url := d.BuildDownloadURL(def.Download.DownloadURL, version, &platformConfig)

	// Create output directory
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Determine filename from URL
	filename := filepath.Base(url)
	outputPath := filepath.Join(outputDir, filename)

	// Download file
	if err := d.downloadFile(url, outputPath); err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	// Extract if tarball
	var finalPath string
	if strings.HasSuffix(filename, ".tar.gz") || strings.HasSuffix(filename, ".tgz") {
		// Create unique extraction directory using filename without extension
		baseName := strings.TrimSuffix(strings.TrimSuffix(filename, ".tar.gz"), ".tgz")
		extractDir := filepath.Join(outputDir, baseName+"-extracted")
		if err := d.extractTarGz(outputPath, extractDir); err != nil {
			return nil, fmt.Errorf("extraction failed: %w", err)
		}

		// Find the actual extracted directory
		// Many tarballs extract to a subdirectory (e.g., go/, ansible-2.20.0/)
		entries, err := os.ReadDir(extractDir)
		if err != nil {
			return nil, fmt.Errorf("failed to read extracted directory: %w", err)
		}

		// If there's exactly one directory, use it as the working directory
		if len(entries) == 1 && entries[0].IsDir() {
			finalPath = filepath.Join(extractDir, entries[0].Name())
		} else {
			// Multiple files/dirs or single file - use extract dir
			finalPath = extractDir
		}
	} else {
		finalPath = outputPath
	}

	// Create artifact entity
	artifact := &entities.Artifact{
		Name:     def.Name,
		Version:  version,
		Platform: platform,
		Path:     finalPath,
		Type:     "binary",
	}

	return artifact, nil
}

// BuildDownloadURL performs template substitution (exported for testing)
func (d *Downloader) BuildDownloadURL(template, version string, platformConfig *entities.PlatformConfig) string {
	url := template
	url = strings.ReplaceAll(url, "{version}", version)

	// Use platform-specific values if provided
	os := "linux"
	arch := "amd64"
	suffix := ""

	if platformConfig != nil {
		if platformConfig.OS != "" {
			os = platformConfig.OS
		}
		if platformConfig.Arch != "" {
			arch = platformConfig.Arch
		}
		if platformConfig.Suffix != "" {
			suffix = platformConfig.Suffix
			// Replace {version} in suffix if present
			suffix = strings.ReplaceAll(suffix, "{version}", version)
		}
	}

	url = strings.ReplaceAll(url, "{os}", os)
	url = strings.ReplaceAll(url, "{arch}", arch)
	url = strings.ReplaceAll(url, "{suffix}", suffix)

	return url
}

// downloadFile downloads a file from URL to destination
func (d *Downloader) downloadFile(url, dest string) error {
	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set user agent
	req.Header.Set("User-Agent", "potions/1.0")

	// Execute request
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	//nolint:errcheck // Defer close on HTTP response body
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Create destination file
	//nolint:gosec // G304: File path dest is function parameter for download destination
	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	//nolint:errcheck // Defer close on file being written
	defer out.Close()

	// Copy with progress tracking
	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Log download size
	fmt.Fprintf(os.Stderr, "Downloaded %s (%d bytes)\n", filepath.Base(dest), written)

	return nil
}

// extractTarGz extracts a .tar.gz file to destination directory
func (d *Downloader) extractTarGz(tarPath, destDir string) error {
	// Open tar.gz file
	//nolint:gosec // G304: File path tarPath is function parameter for extraction
	file, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("failed to open tar.gz: %w", err)
	}
	//nolint:errcheck // Defer close on read-only file
	defer file.Close()

	// Create gzip reader
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	//nolint:errcheck // Defer close on gzip reader
	defer gzr.Close()

	// Create tar reader
	tr := tar.NewReader(gzr)

	// Create destination directory
	if err := os.MkdirAll(destDir, 0750); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Collect symlinks for second pass (to handle cases where target doesn't exist yet)
	type symlinkInfo struct {
		target   string
		linkname string
	}
	var symlinks []symlinkInfo

	// Extract all files (first pass: files and directories)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return fmt.Errorf("tar read error: %w", err)
		}

		// Build target path
		//nolint:gosec // G305: Path traversal validated by HasPrefix check below
		target := filepath.Join(destDir, header.Name)

		// Ensure target is within destDir (security check)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)) {
			return fmt.Errorf("invalid file path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(target, 0750); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}

		case tar.TypeReg:
			// Create parent directory
			if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			// Create file
			//nolint:gosec // G115: Integer overflow from tar header mode is acceptable
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}

			// Copy file contents with size limit (1GB max to prevent decompression bombs)
			if _, err := io.Copy(outFile, io.LimitReader(tr, 1<<30)); err != nil {
				_ = outFile.Close()
				return fmt.Errorf("failed to write file: %w", err)
			}
			if err := outFile.Close(); err != nil {
				return fmt.Errorf("failed to close file: %w", err)
			}

		case tar.TypeSymlink:
			// Defer symlink creation to second pass
			symlinks = append(symlinks, symlinkInfo{
				target:   target,
				linkname: header.Linkname,
			})

		default:
			fmt.Fprintf(os.Stderr, "Warning: ignoring unsupported file type %c: %s\n", header.Typeflag, header.Name)
		}
	}

	// Second pass: create symlinks after all files exist
	for _, link := range symlinks {
		// Create parent directory for symlink
		if err := os.MkdirAll(filepath.Dir(link.target), 0750); err != nil {
			return fmt.Errorf("failed to create directory for symlink: %w", err)
		}
		// Create symlink (may still fail if target doesn't exist, but that's ok)
		if err := os.Symlink(link.linkname, link.target); err != nil {
			// Warn but don't fail - some tarballs have broken symlinks
			fmt.Fprintf(os.Stderr, "Warning: failed to create symlink %s -> %s: %v\n", link.target, link.linkname, err)
		}
	}

	fmt.Fprintf(os.Stderr, "Extracted to %s\n", destDir)
	return nil
}
