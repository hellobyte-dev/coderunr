package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

type Package struct {
	Language        string `json:"language"`
	LanguageVersion string `json:"language_version"`
	Installed       bool   `json:"installed"`
}

type PackageSpec struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Language string `json:"language"`
}

type PackageAction struct {
	Action   string        `json:"action"`
	Packages []PackageSpec `json:"packages"`
}

type PackageResponse struct {
	Packages []Package `json:"packages"`
}

type PackageActionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

func NewPackageCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "package",
		Aliases: []string{"pkg", "ppman"},
		Short:   "Manage runtime packages",
		Long: `Manage packages for different runtime environments.

Available actions:
  list     - List all available packages
  install  - Install packages
  uninstall - Uninstall packages`,
	}

	cmd.AddCommand(NewPackageListCommand())
	cmd.AddCommand(NewPackageInstallCommand())
	cmd.AddCommand(NewPackageUninstallCommand())

	return cmd
}

func NewPackageListCommand() *cobra.Command {
	var language string

	cmd := &cobra.Command{
		Use:   "list [language]",
		Short: "List available packages",
		Long: `List all available packages, optionally filtered by language.

Examples:
  # List all packages
  coderunr package list

  # List Python packages only
  coderunr package list python

  # List packages with specific language filter
  coderunr package list -l python`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				language = args[0]
			}

			url, _ := cmd.Flags().GetString("url")
			verbose, _ := cmd.Flags().GetBool("verbose")

			return listPackages(url, language, verbose)
		},
	}

	cmd.Flags().StringVarP(&language, "language", "l", "", "Filter by language")

	return cmd
}

func NewPackageInstallCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <language> <packages...>",
		Short: "Install packages",
		Long: `Install packages for a specific runtime language.

Examples:
  # Install Python packages
  coderunr package install python numpy pandas

  # Install specific versions
  coderunr package install python numpy==1.21.0 pandas>=1.3.0`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			language := args[0]
			packageNames := args[1:]

			url, _ := cmd.Flags().GetString("url")
			verbose, _ := cmd.Flags().GetBool("verbose")

			return packageAction(url, "install", language, packageNames, verbose)
		},
	}

	return cmd
}

func NewPackageUninstallCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall <language> <packages...>",
		Short: "Uninstall packages",
		Long: `Uninstall packages from a specific runtime language.

Examples:
  # Uninstall Python packages
  coderunr package uninstall python numpy pandas`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			language := args[0]
			packageNames := args[1:]

			url, _ := cmd.Flags().GetString("url")
			verbose, _ := cmd.Flags().GetBool("verbose")

			return packageAction(url, "uninstall", language, packageNames, verbose)
		},
	}

	return cmd
}

func listPackages(baseURL, language string, verbose bool) error {
	client := &http.Client{Timeout: 30 * time.Second}

	// Build URL with optional language filter
	reqURL := baseURL + "/api/v2/packages"
	if language != "" {
		params := url.Values{}
		params.Add("language", language)
		reqURL += "?" + params.Encode()
	}

	resp, err := client.Get(reqURL)
	if err != nil {
		return fmt.Errorf("failed to fetch packages: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var packages []Package
	if err := json.NewDecoder(resp.Body).Decode(&packages); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return printPackageList(packages, verbose)
}

func packageAction(baseURL, action, language string, packages []string, verbose bool) error {
	for _, packageName := range packages {
		request := map[string]string{
			"language": packageName, // The package name is actually the language
			"version":  "*",         // Default to latest version
		}

		reqBody, err := json.Marshal(request)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}

		client := &http.Client{Timeout: 60 * time.Second}

		var resp *http.Response
		if action == "install" {
			resp, err = client.Post(baseURL+"/api/v2/packages", "application/json", strings.NewReader(string(reqBody)))
		} else if action == "uninstall" {
			req, reqErr := http.NewRequest("DELETE", baseURL+"/api/v2/packages", strings.NewReader(string(reqBody)))
			if reqErr != nil {
				return fmt.Errorf("failed to create request: %w", reqErr)
			}
			req.Header.Set("Content-Type", "application/json")
			var doErr error
			resp, doErr = client.Do(req)
			if doErr != nil {
				return fmt.Errorf("failed to execute %s: %w", action, doErr)
			}
		} else {
			return fmt.Errorf("unsupported action: %s", action)
		}

		if err != nil {
			return fmt.Errorf("failed to execute %s: %w", action, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			fmt.Printf("Failed to %s %s: %s\n", action, packageName, string(body))
			continue
		}

		var response map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}

		fmt.Printf("Successfully %sed %s %s\n", action, response["language"], response["version"])
	}

	return nil
}

func parsePackageSpec(pkg, language string) PackageSpec {
	// Simple parsing - can be enhanced to handle version specifiers
	// Examples: "numpy", "numpy==1.21.0", "pandas>=1.3.0"

	// For now, treat the entire string as package name
	// More sophisticated parsing can be added later
	return PackageSpec{
		Name:     pkg,
		Language: language,
		Version:  "", // Default to latest
	}
}

func printPackageList(packages []Package, verbose bool) error {
	if len(packages) == 0 {
		fmt.Println("No packages found")
		return nil
	}

	// Group packages by language
	packagesByLang := make(map[string][]Package)
	for _, pkg := range packages {
		packagesByLang[pkg.Language] = append(packagesByLang[pkg.Language], pkg)
	}

	// Sort languages
	var languages []string
	for lang := range packagesByLang {
		languages = append(languages, lang)
	}
	sort.Strings(languages)

	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)

	if verbose {
		// Detailed tabular output
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

		for _, lang := range languages {
			bold.Printf("\n%s:\n", strings.ToUpper(lang))

			fmt.Fprintln(w, "  VERSION\tINSTALLED")
			fmt.Fprintln(w, "  -------\t---------")

			langPackages := packagesByLang[lang]
			sort.Slice(langPackages, func(i, j int) bool {
				return langPackages[i].LanguageVersion < langPackages[j].LanguageVersion
			})

			for _, pkg := range langPackages {
				status := "No"
				if pkg.Installed {
					status = "Yes"
				}
				fmt.Fprintf(w, "  %s\t%s\n", pkg.LanguageVersion, status)
			}

			w.Flush()
		}
		fmt.Println()
	} else {
		// Compact output
		for _, lang := range languages {
			langPackages := packagesByLang[lang]
			installedCount := 0
			for _, pkg := range langPackages {
				if pkg.Installed {
					installedCount++
				}
			}

			bold.Printf("%-15s", lang+":")
			fmt.Printf(" %d packages (%d installed)\n", len(langPackages), installedCount)

			// Show installed packages
			for _, pkg := range langPackages {
				if pkg.Installed {
					green.Printf("  ● %s (installed)\n", pkg.LanguageVersion)
				} else {
					red.Printf("  ○ %s (available)\n", pkg.LanguageVersion)
				}
			}
		}
	}

	return nil
}

func printPackageActionResult(action string, response PackageActionResponse, verbose bool) error {
	green := color.New(color.FgGreen, color.Bold)
	red := color.New(color.FgRed, color.Bold)

	if response.Success {
		green.Printf("✓ Package %s completed successfully\n", action)
		if response.Message != "" {
			fmt.Printf("Message: %s\n", response.Message)
		}
	} else {
		red.Printf("✗ Package %s failed\n", action)
		if response.Error != "" {
			fmt.Printf("Error: %s\n", response.Error)
		}
		if response.Message != "" {
			fmt.Printf("Message: %s\n", response.Message)
		}
		return fmt.Errorf("package %s failed", action)
	}

	return nil
}
