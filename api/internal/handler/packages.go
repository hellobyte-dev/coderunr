package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"

	"github.com/coderunr/api/internal/service"
	"github.com/coderunr/api/internal/types"
)

// PackageHandler handles package management endpoints
type PackageHandler struct {
	packageService *service.PackageService
	logger         *logrus.Logger
}

// NewPackageHandler creates a new package handler
func NewPackageHandler(packageService *service.PackageService, logger *logrus.Logger) *PackageHandler {
	return &PackageHandler{
		packageService: packageService,
		logger:         logger,
	}
}

// RegisterRoutes registers package management routes
func (ph *PackageHandler) RegisterRoutes(r chi.Router) {
	r.Get("/packages", ph.GetPackages)
	r.Post("/packages", ph.InstallPackage)
	r.Delete("/packages", ph.UninstallPackage)
}

// GetPackages returns a list of all available packages
func (ph *PackageHandler) GetPackages(w http.ResponseWriter, r *http.Request) {
	ph.logger.Debug("Request to list packages")

	packages, err := ph.packageService.GetPackageList()
	if err != nil {
		ph.logger.Errorf("Failed to get package list: %v", err)
		http.Error(w, "Failed to get package list", http.StatusInternalServerError)
		return
	}

	// Convert to response format
	var response []types.PackageInfo
	for _, pkg := range packages {
		response = append(response, types.PackageInfo{
			Language:        pkg.Language,
			LanguageVersion: pkg.Version.String(),
			Installed:       ph.packageService.IsInstalled(pkg),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		ph.logger.Errorf("Failed to encode response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// InstallPackage installs a specific package
func (ph *PackageHandler) InstallPackage(w http.ResponseWriter, r *http.Request) {
	ph.logger.Debug("Request to install package")

	var req struct {
		Language string `json:"language"`
		Version  string `json:"version"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ph.logger.Errorf("Invalid request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Language == "" || req.Version == "" {
		http.Error(w, "Language and version are required", http.StatusBadRequest)
		return
	}

	pkg, err := ph.packageService.GetPackage(req.Language, req.Version)
	if err != nil {
		ph.logger.Errorf("Package not found: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"message": err.Error(),
		})
		return
	}

	if err := ph.packageService.InstallPackage(pkg); err != nil {
		ph.logger.Errorf("Error while installing package %s-%s: %v", pkg.Language, pkg.Version.String(), err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"message": err.Error(),
		})
		return
	}

	response := map[string]string{
		"language": pkg.Language,
		"version":  pkg.Version.String(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		ph.logger.Errorf("Failed to encode response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// UninstallPackage uninstalls a specific package
func (ph *PackageHandler) UninstallPackage(w http.ResponseWriter, r *http.Request) {
	ph.logger.Debug("Request to uninstall package")

	var req struct {
		Language string `json:"language"`
		Version  string `json:"version"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ph.logger.Errorf("Invalid request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Language == "" || req.Version == "" {
		http.Error(w, "Language and version are required", http.StatusBadRequest)
		return
	}

	pkg, err := ph.packageService.GetPackage(req.Language, req.Version)
	if err != nil {
		ph.logger.Errorf("Package not found: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"message": err.Error(),
		})
		return
	}

	if err := ph.packageService.UninstallPackage(pkg); err != nil {
		ph.logger.Errorf("Error while uninstalling package %s-%s: %v", pkg.Language, pkg.Version.String(), err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"message": err.Error(),
		})
		return
	}

	response := map[string]string{
		"language": pkg.Language,
		"version":  pkg.Version.String(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		ph.logger.Errorf("Failed to encode response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
