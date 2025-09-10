package cmd

import (
	"bufio"
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
	cmd.AddCommand(NewPackageSpecCommand())

	return cmd
}

// NewPackageSpecCommand applies a spec file like:
//
//	<language> <version>
//
// Lines beginning with # or blank lines are ignored.
func NewPackageSpecCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "spec <specfile>",
		Short: "Apply a package spec file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			specPath := args[0]
			baseURL, _ := cmd.Flags().GetString("url")
			verbose, _ := cmd.Flags().GetBool("verbose")

			// Wait for API readiness up to 60s
			client := &http.Client{Timeout: 2 * time.Second}
			ready := false
			for i := 0; i < 60; i++ {
				resp, err := client.Get(baseURL + "/api/v2/runtimes")
				if err == nil && resp.StatusCode == http.StatusOK {
					resp.Body.Close()
					ready = true
					break
				}
				if resp != nil {
					resp.Body.Close()
				}
				time.Sleep(1 * time.Second)
			}
			if !ready {
				return fmt.Errorf("API not ready at %s", baseURL)
			}

			f, err := os.Open(specPath)
			if err != nil {
				return fmt.Errorf("failed to open spec: %w", err)
			}
			defer f.Close()

			scanner := bufio.NewScanner(f)
			scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

			failures := 0
			lineNo := 0
			for scanner.Scan() {
				lineNo++
				line := scanner.Text()
				if i := strings.Index(line, "#"); i >= 0 { // strip comments
					line = line[:i]
				}
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				parts := strings.Fields(line)
				if len(parts) < 2 {
					if verbose {
						fmt.Fprintf(os.Stderr, "skip line %d: %q\n", lineNo, line)
					}
					continue
				}
				lang, ver := parts[0], parts[1]
				if err := installLanguageVersion(baseURL, lang, ver); err != nil {
					failures++
					fmt.Fprintf(os.Stderr, "Failed to install %s %s: %v\n", lang, ver, err)
				} else if verbose {
					fmt.Fprintf(os.Stdout, "Installed %s %s\n", lang, ver)
				}
			}
			if scanErr := scanner.Err(); scanErr != nil {
				return fmt.Errorf("failed to read spec: %w", scanErr)
			}
			if failures > 0 {
				return fmt.Errorf("spec apply completed with %d failure(s)", failures)
			}
			return nil
		},
	}
	return c
}

func installLanguageVersion(baseURL, language, version string) error {
	client := &http.Client{
		Timeout: 9 * time.Minute, // 略小于服务端HTTP路由超时
		Transport: &http.Transport{
			DisableKeepAlives: true, // 禁用连接重用，避免EOF问题
		},
	}
	reqObj := map[string]string{
		"language": language,
		"version":  version,
	}
	reqBody, err := json.Marshal(reqObj)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	resp, err := client.Post(baseURL+"/api/v2/packages", "application/json", strings.NewReader(string(reqBody)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Always read the response body to ensure complete transfer
	b, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf("failed to read response body: %w", readErr)
	}

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		return nil
	}
	return fmt.Errorf("status %d: %s", resp.StatusCode, string(b))
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
	client := &http.Client{Timeout: 3 * time.Minute} // 略大于服务端包列表获取超时

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
	client := &http.Client{Timeout: 9 * time.Minute} // 略小于服务端HTTP路由超时
	for _, pkgSpec := range packages {
		// 支持简单的 name 或 name==version / name=version 形式
		name := pkgSpec
		version := "*"
		if parts := strings.SplitN(pkgSpec, "==", 2); len(parts) == 2 {
			name, version = parts[0], parts[1]
		} else if parts := strings.SplitN(pkgSpec, "=", 2); len(parts) == 2 {
			name, version = parts[0], parts[1]
		}

		// coderunr API 期望 {language, version}
		reqObj := map[string]string{
			"language": language,
			"version":  version,
		}
		reqBody, err := json.Marshal(reqObj)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}

		var resp *http.Response
		if action == "install" {
			resp, err = client.Post(baseURL+"/api/v2/packages", "application/json", strings.NewReader(string(reqBody)))
		} else if action == "uninstall" {
			req, reqErr := http.NewRequest("DELETE", baseURL+"/api/v2/packages", strings.NewReader(string(reqBody)))
			if reqErr != nil {
				return fmt.Errorf("failed to create request: %w", reqErr)
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err = client.Do(req)
		} else {
			return fmt.Errorf("unsupported action: %s", action)
		}
		if err != nil {
			return fmt.Errorf("failed to execute %s: %w", action, err)
		}
		// 处理 201/204/200
		if resp.StatusCode == http.StatusNoContent { // 204
			fmt.Printf("Successfully %sed %s %s\n", action, language, version)
			resp.Body.Close()
			continue
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			fmt.Printf("Failed to %s %s: %s\n", action, name, string(body))
			resp.Body.Close()
			continue
		}
		// 尝试解析 JSON，若为空体或解析失败，仍视为成功
		var response map[string]string
		decErr := json.NewDecoder(resp.Body).Decode(&response)
		resp.Body.Close()
		if decErr == nil {
			lang := response["language"]
			ver := response["version"]
			if lang == "" {
				lang = language
			}
			if ver == "" {
				ver = version
			}
			fmt.Printf("Successfully %sed %s %s\n", action, lang, ver)
		} else {
			// 如 201 但无 body
			fmt.Printf("Successfully %sed %s %s\n", action, language, version)
		}
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
