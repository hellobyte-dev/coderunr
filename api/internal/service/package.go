package service

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/sirupsen/logrus"

	"github.com/coderunr/api/internal/config"
	"github.com/coderunr/api/internal/runtime"
	"github.com/coderunr/api/internal/types"
)

// PackageService handles package management operations
type PackageService struct {
	cfg            *config.Config
	logger         *logrus.Logger
	runtimeManager *runtime.Manager
}

// NewPackageService creates a new package service
func NewPackageService(cfg *config.Config, logger *logrus.Logger, runtimeManager *runtime.Manager) *PackageService {
	return &PackageService{
		cfg:            cfg,
		logger:         logger,
		runtimeManager: runtimeManager,
	}
}

// GetPackageList retrieves the list of available packages from the repository
func (ps *PackageService) GetPackageList() ([]*types.Package, error) {
	ps.logger.Debug("Fetching package list from repository")

	resp, err := http.Get(ps.cfg.RepoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch package list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("repository returned status: %d", resp.StatusCode)
	}

	var packages []*types.Package
	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Split(line, ",")
		if len(parts) != 4 {
			ps.logger.Warnf("Invalid package line format: %s", line)
			continue
		}

		version, err := semver.NewVersion(parts[1])
		if err != nil {
			ps.logger.Warnf("Invalid version %s for package %s: %v", parts[1], parts[0], err)
			continue
		}

		pkg := &types.Package{
			Language: parts[0],
			Version:  version,
			Checksum: parts[2],
			Download: parts[3],
		}

		packages = append(packages, pkg)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading package list: %w", err)
	}

	ps.logger.Debugf("Found %d packages in repository", len(packages))
	return packages, nil
}

// GetPackage finds a specific package by language and version constraint
func (ps *PackageService) GetPackage(language, versionConstraint string) (*types.Package, error) {
	packages, err := ps.GetPackageList()
	if err != nil {
		return nil, err
	}

	constraint, err := semver.NewConstraint(versionConstraint)
	if err != nil {
		return nil, fmt.Errorf("invalid version constraint: %w", err)
	}

	var candidates []*types.Package
	for _, pkg := range packages {
		if pkg.Language == language && constraint.Check(pkg.Version) {
			candidates = append(candidates, pkg)
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no package found for %s-%s", language, versionConstraint)
	}

	// Sort by version (highest first) and return the best match
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate.Version.GreaterThan(best.Version) {
			best = candidate
		}
	}

	return best, nil
}

// IsInstalled checks if a package is installed
func (ps *PackageService) IsInstalled(pkg *types.Package) bool {
	installedFile := filepath.Join(ps.getInstallPath(pkg), ".ppman-installed")
	_, err := os.Stat(installedFile)
	return err == nil
}

// InstallPackage installs a package
func (ps *PackageService) InstallPackage(pkg *types.Package) error {
	installPath := ps.getInstallPath(pkg)

	if ps.IsInstalled(pkg) {
		return fmt.Errorf("package %s-%s is already installed", pkg.Language, pkg.Version.String())
	}

	ps.logger.Infof("Installing %s-%s", pkg.Language, pkg.Version.String())

	// Remove any existing directory
	if _, err := os.Stat(installPath); err == nil {
		ps.logger.Warnf("%s-%s has residual files. Removing them.", pkg.Language, pkg.Version.String())
		if err := os.RemoveAll(installPath); err != nil {
			return fmt.Errorf("failed to remove existing directory: %w", err)
		}
	}

	// Create install directory
	if err := os.MkdirAll(installPath, 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	// Download package
	pkgPath := filepath.Join(installPath, "pkg.tar.gz")
	if err := ps.downloadPackage(pkg.Download, pkgPath); err != nil {
		return fmt.Errorf("failed to download package: %w", err)
	}

	// Verify checksum
	if err := ps.verifyChecksum(pkgPath, pkg.Checksum); err != nil {
		return fmt.Errorf("checksum verification failed: %w", err)
	}

	// Extract package
	if err := ps.extractPackage(pkgPath, installPath); err != nil {
		return fmt.Errorf("failed to extract package: %w", err)
	}

	// Cache environment
	if err := ps.cacheEnvironment(installPath); err != nil {
		ps.logger.Warnf("Failed to cache environment for %s-%s: %v", pkg.Language, pkg.Version.String(), err)
	}

	// Mark as installed
	installedFile := filepath.Join(installPath, ".ppman-installed")
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	if err := os.WriteFile(installedFile, []byte(timestamp), 0644); err != nil {
		return fmt.Errorf("failed to mark package as installed: %w", err)
	}

	// Load the package into runtime manager immediately
	ps.logger.Debug("Loading package into runtime manager")
	if err := ps.runtimeManager.LoadPackage(installPath); err != nil {
		ps.logger.WithError(err).Warnf("Failed to load package into runtime manager: %s", installPath)
		// Don't fail installation if runtime loading fails
	}

	ps.logger.Infof("Successfully installed %s-%s", pkg.Language, pkg.Version.String())
	return nil
}

// UninstallPackage uninstalls a package
func (ps *PackageService) UninstallPackage(pkg *types.Package) error {
	installPath := ps.getInstallPath(pkg)

	if !ps.IsInstalled(pkg) {
		return fmt.Errorf("package %s-%s is not installed", pkg.Language, pkg.Version.String())
	}

	ps.logger.Infof("Uninstalling %s-%s", pkg.Language, pkg.Version.String())

	// Remove package directory
	if err := os.RemoveAll(installPath); err != nil {
		return fmt.Errorf("failed to remove package directory: %w", err)
	}

	ps.logger.Infof("Successfully uninstalled %s-%s", pkg.Language, pkg.Version.String())
	return nil
}

// getInstallPath returns the installation path for a package
func (ps *PackageService) getInstallPath(pkg *types.Package) string {
	return filepath.Join(
		ps.cfg.DataDirectory,
		"packages",
		pkg.Language,
		pkg.Version.String(),
	)
}

// downloadPackage downloads a package from the given URL
func (ps *PackageService) downloadPackage(url, destPath string) error {
	ps.logger.Debugf("Downloading package from %s to %s", url, destPath)

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	file, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}

// verifyChecksum verifies the SHA256 checksum of a file
func (ps *PackageService) verifyChecksum(filePath, expectedChecksum string) error {
	ps.logger.Debug("Validating checksums")

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return err
	}

	actualChecksum := hex.EncodeToString(hasher.Sum(nil))
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	return nil
}

// extractPackage extracts a tar.gz package
func (ps *PackageService) extractPackage(pkgPath, installPath string) error {
	ps.logger.Debugf("Extracting package from %s to %s", pkgPath, installPath)

	cmd := exec.Command("tar", "xzf", pkgPath, "-C", installPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	return nil
}

// cacheEnvironment caches the package environment variables
func (ps *PackageService) cacheEnvironment(installPath string) error {
	ps.logger.Debug("Caching environment")

	envScript := filepath.Join(installPath, "environment")
	if _, err := os.Stat(envScript); os.IsNotExist(err) {
		ps.logger.Debug("No environment script found, skipping")
		return nil
	}

	cmd := exec.Command("bash", "-c", fmt.Sprintf("cd %s && source environment && env", installPath))
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	// Filter out common environment variables
	filtered := []string{}
	lines := strings.Split(string(output), "\n")
	excludeVars := map[string]bool{
		"PWD": true, "OLDPWD": true, "_": true, "SHLVL": true,
	}

	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && !excludeVars[parts[0]] {
			filtered = append(filtered, line)
		}
	}

	envFile := filepath.Join(installPath, ".env")
	return os.WriteFile(envFile, []byte(strings.Join(filtered, "\n")), 0644)
}
