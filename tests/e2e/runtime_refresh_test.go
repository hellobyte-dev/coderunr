package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Verify runtimes list refreshes after package install/uninstall
func TestRuntimeRefreshOnPackageChanges(t *testing.T) {
	// Skip if services not running
	if !checkServicesRunning() {
		t.Skip("Services not running, skipping runtime refresh test")
	}

	// Helper to fetch runtimes
	getRuntimes := func() []Runtime {
		resp, err := http.Get(APIBaseURL + "/api/v2/runtimes")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var rts []Runtime
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&rts))
		return rts
	}

	// Use a package that exists in repo index
	// Note: repo includes go 1.16.2, python 3.12.0, java 15.0.2
	// We'll operate on go 1.16.2 and restore original state at the end
	pkgLang := "go"
	pkgVer := "1.16.2"
	installReq := map[string]string{"language": pkgLang, "version": pkgVer}
	body, _ := json.Marshal(installReq)

	// Capture initial runtimes and whether our target language exists
	initial := getRuntimes()
	initiallyInstalled := false
	for _, rt := range initial {
		if rt.Language == pkgLang {
			initiallyInstalled = true
			break
		}
	}

	// Helper to uninstall
	uninstall := func() {
		req, _ := http.NewRequest(http.MethodDelete, APIBaseURL+"/api/v2/packages", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusNoContent, resp.StatusCode)
	}
	// Helper to install
	install := func() {
		resp, err := http.Post(APIBaseURL+"/api/v2/packages", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusCreated, resp.StatusCode)
	}

	// Ensure state flips and restore to original
	if initiallyInstalled {
		// Uninstall should remove from runtimes
		uninstall()
		afterUninstall := getRuntimes()
		has := false
		for _, rt := range afterUninstall {
			if rt.Language == pkgLang {
				has = true
				break
			}
		}
		assert.False(t, has, "runtimes should drop language after uninstall")

		// Reinstall and verify comeback
		install()
		afterInstall := getRuntimes()
		has = false
		for _, rt := range afterInstall {
			if rt.Language == pkgLang {
				has = true
				break
			}
		}
		assert.True(t, has, "runtimes should include language after reinstall")
	} else {
		// Not installed initially: install then uninstall to revert
		install()
		afterInstall := getRuntimes()
		has := false
		for _, rt := range afterInstall {
			if rt.Language == pkgLang {
				has = true
				break
			}
		}
		assert.True(t, has, "runtimes should include language after install")

		uninstall()
		afterUninstall := getRuntimes()
		has = false
		for _, rt := range afterUninstall {
			if rt.Language == pkgLang {
				has = true
				break
			}
		}
		assert.False(t, has, "runtimes should drop language after uninstall")
	}
}
