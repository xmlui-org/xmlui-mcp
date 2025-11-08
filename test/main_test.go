// Package test provides  integration tests for XMLUI Local Server.
//
// This package contains end-to-end tests that validate server functionality.
package test

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Setup test environment
	if err := setup(); err != nil {
		stdErrf("Setup failed: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Cleanup test environment
	teardown()

	os.Exit(code)
}

// setup prepares the test environment before running tests.
func setup() (err error) {
	// TODO Setup goes here
	return err
}

// teardown cleans up the test environment after all tests have run.
func teardown() {
	// TODO Teardown goes here
}

func stdErrf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format, args...)
}
