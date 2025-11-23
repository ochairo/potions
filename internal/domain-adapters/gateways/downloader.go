package gateways

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ochairo/potions/internal/domain/entities"
)

// Security validation functions

// validatePathWithinBase ensures path doesn't escape base directory (prevents Zip Slip)
func validatePathWithinBase(path, base string) error {
	// Clean paths to resolve ../ segments
	cleanPath := filepath.Clean(path)
	cleanBase := filepath.Clean(base)

	// Convert to absolute
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}
	absBase, err := filepath.Abs(cleanBase)
	if err != nil {
		return fmt.Errorf("failed to resolve base: %w", err)
	}

	// Ensure path is within base directory
	if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) && absPath != absBase {
		return fmt.Errorf("path traversal detected: %s escapes %s", path, base)
	}
	return nil
}

// validateGitURL validates git repository URLs to prevent command injection
func validateGitURL(urlStr string) error {
	// Only allow https:// or git@ URLs
	if !strings.HasPrefix(urlStr, "https://") && !strings.HasPrefix(urlStr, "git@") {
		return fmt.Errorf("only https:// and git@ URLs allowed, got: %s", urlStr)
	}

	// Block shell metacharacters that could enable command injection
	if strings.ContainsAny(urlStr, "|&;`$(){}[]<>\n\r") {
		return fmt.Errorf("invalid characters in URL")
	}

	// Additional validation for https URLs
	if strings.HasPrefix(urlStr, "https://") {
		if _, err := url.Parse(urlStr); err != nil {
			return fmt.Errorf("invalid URL format: %w", err)
		}
	}

	return nil
}

// validateGitTag validates git tag/branch names to prevent command injection
func validateGitTag(tag string) error {
	if tag == "" {
		return fmt.Errorf("tag cannot be empty")
	}

	// Only allow safe characters: alphanumeric, dots, hyphens, underscores, slashes (for refs)
	matched, err := regexp.MatchString(`^[a-zA-Z0-9._/-]+$`, tag)
	if err != nil {
		return fmt.Errorf("regex error: %w", err)
	}
	if !matched {
		return fmt.Errorf("tag contains invalid characters: %s", tag)
	}

	// Block obviously dangerous patterns
	if strings.Contains(tag, "..") {
		return fmt.Errorf("tag contains path traversal")
	}

	return nil
}

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

	// Create output directory
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	var finalPath string
	var downloadedFilePath string

	// Check if this is a git-based download
	if def.Download.Method == "git" && def.Download.GitURL != "" {
		// Clone from git
		gitTag := def.Download.GitTagPrefix + version
		cloneDir := filepath.Join(outputDir, def.Name+"-"+version)

		// Convert to absolute path for security validation
		absCloneDir, err := filepath.Abs(cloneDir)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
		}

		if err := d.cloneGitRepo(def.Download.GitURL, gitTag, absCloneDir); err != nil {
			return nil, fmt.Errorf("git clone failed: %w", err)
		}
		finalPath = absCloneDir
		// For git downloads, there's no separate download file
		downloadedFilePath = ""
	} else {
		// HTTP download (existing behavior)
		url := d.BuildDownloadURL(def.Download.DownloadURL, version, &platformConfig)

		// Determine filename from URL, sanitizing to remove query params and invalid chars
		filename := sanitizeFilename(url)
		outputPath := filepath.Join(outputDir, filename)

		// Download file
		if err := d.downloadFile(url, outputPath); err != nil {
			return nil, fmt.Errorf("download failed: %w", err)
		}

		// Keep track of the original downloaded file path
		downloadedFilePath = outputPath

		// Extract if tarball
		if strings.HasSuffix(filename, ".tar.gz") || strings.HasSuffix(filename, ".tgz") {
			// Create unique extraction directory using filename without extension
			baseName := strings.TrimSuffix(strings.TrimSuffix(filename, ".tar.gz"), ".tgz")
			extractDir := filepath.Join(outputDir, baseName+"-extracted")
			if err := d.extractTarGz(outputPath, extractDir); err != nil {
				return nil, fmt.Errorf("extraction failed: %w", err)
			}

			// Find the actual extracted directory
			entries, err := os.ReadDir(extractDir)
			if err != nil {
				return nil, fmt.Errorf("failed to read extracted directory: %w", err)
			}

			// If there's exactly one directory, use it as the working directory
			if len(entries) == 1 && entries[0].IsDir() {
				finalPath = filepath.Join(extractDir, entries[0].Name())
			} else {
				finalPath = extractDir
			}
		} else {
			finalPath = outputPath
		}
	}

	// Create artifact entity with both paths
	artifact := &entities.Artifact{
		Name:         def.Name,
		Version:      version,
		Platform:     platform,
		Path:         finalPath,
		DownloadPath: downloadedFilePath,
		Type:         "binary",
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

		// Replace custom fields (e.g., {target}, {triple}, etc.)
		for key, value := range platformConfig.Custom {
			placeholder := "{" + key + "}"
			url = strings.ReplaceAll(url, placeholder, value)
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
		return fmt.Errorf("HTTP %d: %s (URL: %s)", resp.StatusCode, resp.Status, url)
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
		//nolint:gosec // G305: Path traversal validated by checks below
		target := filepath.Join(destDir, header.Name)

		// SECURITY: Prevent Zip Slip vulnerability
		// 1. Check for absolute paths in tar entries
		if filepath.IsAbs(header.Name) {
			return fmt.Errorf("security: tar entry contains absolute path: %s", header.Name)
		}

		// 2. Check for path traversal attempts (../ as path component, not substring in filename)
		// Split the path into components and check each one
		pathComponents := strings.Split(filepath.ToSlash(header.Name), "/")
		for _, component := range pathComponents {
			if component == ".." {
				return fmt.Errorf("security: tar entry contains path traversal: %s", header.Name)
			}
		}

		// 3. Validate resolved path is within destination directory
		if err := validatePathWithinBase(target, destDir); err != nil {
			return fmt.Errorf("security: path traversal attempt: %w", err)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory with writable permissions (ensure we can delete it later)
			if err := os.MkdirAll(target, 0750); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}

		case tar.TypeReg:
			// Create parent directory with writable permissions
			if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			// SECURITY: Set explicit file permissions
			// Use restrictive permissions: 0640 for regular files, 0750 for executables
			var mode os.FileMode
			if header.Mode >= 0 && header.Mode <= 0777 {
				// Check if original file was executable
				if os.FileMode(header.Mode)&0111 != 0 {
					mode = 0750 // rwxr-x--- for executables
				} else {
					mode = 0640 // rw-r----- for regular files
				}
			} else {
				// Invalid mode, use safe default
				mode = 0640
			}

			//nolint:gosec // G304: target path validated by validatePathWithinBase
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			} // Copy file contents with size limit (1GB max to prevent decompression bombs)
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
		// Create parent directory for symlink with writable permissions
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

// sanitizeFilename removes invalid characters and query parameters from a filename
func sanitizeFilename(rawURL string) string {
	// Parse URL to remove query parameters and fragments
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		// If URL parsing fails, try to extract filename from the raw string
		parsedURL = &url.URL{Path: rawURL}
	}

	// Get the base filename from the path (without query params)
	filename := filepath.Base(parsedURL.Path)

	// If filename is empty or just "/", generate a default name
	if filename == "" || filename == "/" || filename == "." {
		filename = "download"
	}

	// Remove invalid filesystem characters: " : < > | * ? \r \n
	// These are not allowed in Windows NTFS and cause issues in GitHub Actions
	invalidChars := regexp.MustCompile(`[":<>|*?\r\n]`)
	filename = invalidChars.ReplaceAllString(filename, "_")

	return filename
}

// cloneGitRepo clones a git repository to the destination directory
func (d *Downloader) cloneGitRepo(gitURL, tag, destDir string) error {
	// Security: Validate destDir is an absolute path and is clean
	if !filepath.IsAbs(destDir) {
		return fmt.Errorf("destination directory must be absolute path")
	}
	cleanDest := filepath.Clean(destDir)
	if cleanDest != destDir {
		return fmt.Errorf("destination directory contains path traversal elements")
	}

	// Security: Prevent command injection via git URL or tag
	if err := validateGitURL(gitURL); err != nil {
		return err
	}
	if err := validateGitTag(tag); err != nil {
		return err
	}

	//nolint:gosec // G204: gitURL and tag validated by validateGitURL and validateGitTag
	cmd := exec.Command("git", "clone", "--depth=1", "--branch="+tag, gitURL, destDir)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Cloned %s (tag: %s) to %s\n", gitURL, tag, destDir)
	return nil
}
