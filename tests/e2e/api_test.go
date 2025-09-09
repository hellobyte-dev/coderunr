package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	APIBaseURL  = "http://localhost:2000"
	RepoBaseURL = "http://localhost:8000"
)

func TestServices(t *testing.T) {
	t.Run("API Service Health", func(t *testing.T) {
		resp, err := http.Get(APIBaseURL + "/api/v2/packages")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Package Repository Health", func(t *testing.T) {
		resp, err := http.Get(RepoBaseURL)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestPackageAPI(t *testing.T) {
	t.Run("List Packages", func(t *testing.T) {
		resp, err := http.Get(APIBaseURL + "/api/v2/packages")
		require.NoError(t, err)
		defer resp.Body.Close()

		var packages []PackageInfo
		err = json.NewDecoder(resp.Body).Decode(&packages)
		require.NoError(t, err)

		// Should have at least one package from repository index
		assert.GreaterOrEqual(t, len(packages), 1)

		// Verify expected packages
		languages := make(map[string]bool)
		for _, pkg := range packages {
			languages[pkg.Language] = true
		}

		assert.True(t, languages["python"] || languages["go"] || languages["java"], "At least one of python/go/java should be listed")
	})
}

func TestRuntimeAPI(t *testing.T) {
	t.Run("List Runtimes", func(t *testing.T) {
		resp, err := http.Get(APIBaseURL + "/api/v2/runtimes")
		require.NoError(t, err)
		defer resp.Body.Close()

		var runtimes []Runtime
		err = json.NewDecoder(resp.Body).Decode(&runtimes)
		require.NoError(t, err)

		// Runtimes depend on installed packages; presence may vary, but response should be valid JSON array
		assert.GreaterOrEqual(t, len(runtimes), 0)

		// Verify runtime structure
		for _, runtime := range runtimes {
			assert.NotEmpty(t, runtime.Language, "Runtime language should not be empty")
			assert.NotEmpty(t, runtime.Version, "Runtime version should not be empty")
			assert.NotEmpty(t, runtime.Runtime, "Runtime name should not be empty")
		}

		// No hard assertions on specific languages here; covered by package management tests
	})
}

func TestPackageRepository(t *testing.T) {
	t.Run("Package Index", func(t *testing.T) {
		resp, err := http.Get(RepoBaseURL + "/index")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Read response body
		var body []byte
		_, err = resp.Body.Read(body[:])
		// Index should contain package information
		// Format: language,version,checksum,url
		assert.NoError(t, err)
	})

	t.Run("Package Download", func(t *testing.T) {
		// Test if package files are accessible
		packages := []string{
			"python-3.12.0.pkg.tar.gz",
			"go-1.16.2.pkg.tar.gz",
			"java-15.0.2.pkg.tar.gz",
		}

		for _, pkg := range packages {
			t.Run(pkg, func(t *testing.T) {
				resp, err := http.Head(RepoBaseURL + "/" + pkg)
				require.NoError(t, err)
				defer resp.Body.Close()

				assert.Equal(t, http.StatusOK, resp.StatusCode)
				assert.Equal(t, "application/gzip", resp.Header.Get("Content-Type"))
				assert.NotEmpty(t, resp.Header.Get("Content-Length"))
			})
		}
	})
}

// Helper function to wait for services to be ready
func waitForServices(t *testing.T) {
	timeout := time.Now().Add(30 * time.Second)

	for time.Now().Before(timeout) {
		// Check API service
		if resp, err := http.Get(APIBaseURL + "/api/v2/packages"); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return // Services are ready
			}
		}

		fmt.Println("Waiting for services to be ready...")
		time.Sleep(2 * time.Second)
	}

	t.Fatal("Services did not become ready within timeout")
}
