package tests

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	fmt.Println("🧪 Starting CodeRunr E2E Test Suite")

	// Setup
	fmt.Println("Setting up test environment...")

	// Run tests
	code := m.Run()

	// Teardown
	fmt.Println("Cleaning up test environment...")

	os.Exit(code)
}
