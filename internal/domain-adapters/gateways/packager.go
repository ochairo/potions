package gateways

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ochairo/potions/internal/domain/entities"
)

// Packager handles packaging built binaries into distributable archives
type Packager struct{}

// NewPackager creates a new packager
func NewPackager() *Packager {
	return &Packager{}
}

// PackageArtifact packages built binaries into a tar.gz archive
// Returns a new artifact pointing to the packaged tar.gz file
func (p *Packager) PackageArtifact(
	_ context.Context,
	def *entities.Recipe,
	artifact *entities.Artifact,
	version, platform, outputDir string,
) (*entities.Artifact, error) {
	// Determine source directory to package
	sourceDir := artifact.Path
	isSingleFile := false

	// Check if artifact.Path is a file or directory
	info, err := os.Stat(artifact.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat artifact path: %w", err)
	}

	// First, check if a bin directory exists in the output directory
	// This handles custom builds that install to $PREFIX/bin
	// Check both outputDir/outputDir/bin and outputDir/bin since the working
	// directory for binary downloads is filepath.Dir(artifact.Path) which is outputDir,
	// so mkdir -p $PREFIX/bin creates outputDir/outputDir/bin
	binDirNested := filepath.Join(outputDir, outputDir, "bin")
	binDirDirect := filepath.Join(outputDir, "bin")

	if binInfo, err := os.Stat(binDirNested); err == nil && binInfo.IsDir() {
		// Use nested bin directory (outputDir/outputDir/bin)
		sourceDir = binDirNested
		isSingleFile = false
	} else if binInfo, err := os.Stat(binDirDirect); err == nil && binInfo.IsDir() {
		// Use direct bin directory (outputDir/bin)
		sourceDir = binDirDirect
		isSingleFile = false
	} else if !info.IsDir() {
		// It's a single file (direct binary download) - we'll package just this file
		isSingleFile = true
	} else if strings.Contains(sourceDir, "extracted") {
		// For extracted source, look for the install directory or binary output
		// Check for bin directory relative to the artifact's parent directory
		buildOutputDir := filepath.Dir(filepath.Dir(sourceDir)) // Go up from extracted/pkgname to output dir
		installDir := filepath.Join(buildOutputDir, "bin")
		if _, err := os.Stat(installDir); err == nil {
			sourceDir = installDir
		}
		// If no bin directory, package the entire extracted directory
	}

	// Create output filename: packagename-version-platform.tar.gz
	// Remove 'v' prefix from version if present
	cleanVersion := strings.TrimPrefix(version, "v")
	tarballName := fmt.Sprintf("%s-%s-%s.tar.gz", def.Name, cleanVersion, platform)

	// Output to the configured output directory
	if outputDir == "" {
		outputDir = "dist"
	}
	tarballPath := filepath.Join(outputDir, tarballName)

	// Create the tarball
	if isSingleFile {
		if err := p.createTarballFromFile(sourceDir, tarballPath, def.Name); err != nil {
			return nil, fmt.Errorf("failed to create tarball: %w", err)
		}
	} else {
		if err := p.createTarball(sourceDir, tarballPath); err != nil {
			return nil, fmt.Errorf("failed to create tarball: %w", err)
		}
	}

	// Create new artifact pointing to the tarball
	packagedArtifact := &entities.Artifact{
		Name:     def.Name,
		Version:  version,
		Platform: platform,
		Path:     tarballPath,
		Type:     "archive",
	}

	return packagedArtifact, nil
}

// createTarball creates a gzipped tar archive from a source directory
func (p *Packager) createTarball(sourceDir, tarballPath string) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(tarballPath), 0750); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create the tar.gz file
	//nolint:gosec // G304: File path tarballPath is constructed for package output
	file, err := os.Create(tarballPath)
	if err != nil {
		return fmt.Errorf("failed to create tarball file: %w", err)
	}
	//nolint:errcheck // Defer close
	defer file.Close()

	// Create gzip writer
	gzipWriter := gzip.NewWriter(file)
	//nolint:errcheck // Defer close
	defer gzipWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzipWriter)
	//nolint:errcheck // Defer close
	defer tarWriter.Close()

	// Walk the source directory and add files to the tarball
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Handle symlinks - read the link target
		var linkTarget string
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err = os.Readlink(path)
			if err != nil {
				// Skip broken/unreadable symlinks to prevent extraction errors
				// This is rare and usually indicates a broken symlink in the source
				fmt.Fprintf(os.Stderr, "Warning: skipping unreadable symlink: %s (%v)\n", path, err)
				return nil
			}
		}

		// Create tar header with symlink support
		header, err := tar.FileInfoHeader(info, linkTarget)
		if err != nil {
			return fmt.Errorf("failed to create tar header: %w", err)
		}

		// Update header name to be relative to source directory
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		header.Name = relPath

		// Write header
		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header: %w", err)
		}

		// If it's a directory or symlink, we're done (symlink target is in header)
		if info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		// If it's a regular file, write its contents
		if info.Mode().IsRegular() {
			//nolint:gosec // G304: File path from filepath.Walk for packaging
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			//nolint:errcheck // Defer close
			defer file.Close()

			if _, err := io.Copy(tarWriter, file); err != nil {
				return fmt.Errorf("failed to write file to tar: %w", err)
			}
		}

		return nil
	})
}

// createTarballFromFile creates a gzipped tar archive from a single file
func (p *Packager) createTarballFromFile(sourceFile, tarballPath, nameInArchive string) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(tarballPath), 0750); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create the tar.gz file
	//nolint:gosec // G304: tarballPath is constructed for package output
	outFile, err := os.Create(tarballPath)
	if err != nil {
		return fmt.Errorf("failed to create tarball file: %w", err)
	}
	//nolint:errcheck // Defer close
	defer outFile.Close()

	// Create gzip writer
	gzipWriter := gzip.NewWriter(outFile)
	//nolint:errcheck // Defer close
	defer gzipWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzipWriter)
	//nolint:errcheck // Defer close
	defer tarWriter.Close()

	// Open source file
	//nolint:gosec // G304: sourceFile is function parameter for packaging
	file, err := os.Open(sourceFile)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	//nolint:errcheck // Defer close
	defer file.Close()

	// Get file info
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	// Create tar header
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return fmt.Errorf("failed to create tar header: %w", err)
	}

	// Use the package name as the file name in the archive
	header.Name = nameInArchive

	// Write header
	if err := tarWriter.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}

	// Write file contents
	if _, err := io.Copy(tarWriter, file); err != nil {
		return fmt.Errorf("failed to write file to tar: %w", err)
	}

	return nil
}
