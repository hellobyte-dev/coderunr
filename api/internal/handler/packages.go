package handler

import (
	"encoding/json"
	"errors"
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(types.ErrorResponse{Message: "Failed to get package list"})
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(types.ErrorResponse{Message: "Failed to encode response"})
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

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			_ = json.NewEncoder(w).Encode(types.ErrorResponse{Message: "Request body too large"})
			return
		}
		ph.logger.Errorf("Invalid request body: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(types.ErrorResponse{Message: "Invalid request body"})
		return
	}

	if req.Language == "" || req.Version == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(types.ErrorResponse{Message: "Language and version are required"})
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
		_ = json.NewEncoder(w).Encode(types.ErrorResponse{Message: err.Error()})
		return
	}

	response := map[string]string{
		"language": pkg.Language,
		"version":  pkg.Version.String(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		ph.logger.Errorf("Failed to encode response: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(types.ErrorResponse{Message: "Failed to encode response"})
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

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			_ = json.NewEncoder(w).Encode(types.ErrorResponse{Message: "Request body too large"})
			return
		}
		ph.logger.Errorf("Invalid request body: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(types.ErrorResponse{Message: "Invalid request body"})
		return
	}

	if req.Language == "" || req.Version == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(types.ErrorResponse{Message: "Language and version are required"})
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
		_ = json.NewEncoder(w).Encode(types.ErrorResponse{Message: err.Error()})
		return
	}

	// No Content as per alignment
	w.WriteHeader(http.StatusNoContent)
}
