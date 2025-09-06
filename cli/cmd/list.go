package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

type Runtime struct {
	Language string   `json:"language"`
	Version  string   `json:"version"`
	Aliases  []string `json:"aliases"`
	Runtime  string   `json:"runtime,omitempty"`
}

type RuntimesResponse struct {
	Runtimes []Runtime `json:"runtimes"`
}

func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls", "runtimes"},
		Short:   "List available programming languages and runtimes",
		Long: `List all available programming languages and their versions.

This command fetches the current list of supported runtimes from the CodeRunr server,
including language names, versions, and available aliases.

Examples:
  # List all available runtimes
  coderunr list

  # Show verbose output with additional details
  coderunr list -v`,
		RunE: func(cmd *cobra.Command, args []string) error {
			url, _ := cmd.Flags().GetString("url")
			verbose, _ := cmd.Flags().GetBool("verbose")

			return listRuntimes(url, verbose)
		},
	}

	return cmd
}

func listRuntimes(baseURL string, verbose bool) error {
	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Get(baseURL + "/api/v2/runtimes")
	if err != nil {
		return fmt.Errorf("failed to fetch runtimes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var runtimes []Runtime
	if err := json.NewDecoder(resp.Body).Decode(&runtimes); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return printRuntimeList(runtimes, verbose)
}

func printRuntimeList(runtimes []Runtime, verbose bool) error {
	if len(runtimes) == 0 {
		fmt.Println("No runtimes available")
		return nil
	}

	// Group runtimes by language
	runtimesByLang := make(map[string][]Runtime)
	for _, runtime := range runtimes {
		runtimesByLang[runtime.Language] = append(runtimesByLang[runtime.Language], runtime)
	}

	// Sort languages
	var languages []string
	for lang := range runtimesByLang {
		languages = append(languages, lang)
	}
	sort.Strings(languages)

	bold := color.New(color.Bold)
	cyan := color.New(color.FgCyan)

	if verbose {
		// Detailed tabular output
		fmt.Printf("Available runtimes (%d languages, %d total runtimes):\n\n",
			len(languages), len(runtimes))

		for _, lang := range languages {
			bold.Printf("%s:\n", strings.ToUpper(lang))

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "  VERSION\tALIASES\tRUNTIME")
			fmt.Fprintln(w, "  -------\t-------\t-------")

			langRuntimes := runtimesByLang[lang]
			sort.Slice(langRuntimes, func(i, j int) bool {
				return langRuntimes[i].Version < langRuntimes[j].Version
			})

			for _, runtime := range langRuntimes {
				aliases := strings.Join(runtime.Aliases, ", ")
				if aliases == "" {
					aliases = "-"
				}
				runtimeName := runtime.Runtime
				if runtimeName == "" {
					runtimeName = "-"
				}
				fmt.Fprintf(w, "  %s\t%s\t%s\n", runtime.Version, aliases, runtimeName)
			}

			w.Flush()
			fmt.Println()
		}
	} else {
		// Compact output showing just language and versions
		fmt.Printf("Available languages (%d):\n\n", len(languages))

		for _, lang := range languages {
			langRuntimes := runtimesByLang[lang]

			// Get all versions for this language
			var versions []string
			for _, runtime := range langRuntimes {
				versions = append(versions, runtime.Version)
			}

			bold.Printf("%-15s", lang+":")
			cyan.Printf(" %s\n", strings.Join(versions, ", "))
		}

		fmt.Printf("\nTotal: %d languages, %d runtimes\n", len(languages), len(runtimes))
		fmt.Println("\nUse --verbose flag for detailed information including aliases and runtime details.")
	}

	return nil
}
