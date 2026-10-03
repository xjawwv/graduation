package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLocalEnvironmentWithoutOverwritingProcessEnvironment(t *testing.T) {
	workingDirectory := t.TempDir()
	t.Chdir(workingDirectory)
	path := filepath.Join(workingDirectory, ".env")
	if err := os.WriteFile(path, []byte("GRADUATION_TEST_FILE=from-file\nGRADUATION_TEST_EXISTING=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	const fileKey = "GRADUATION_TEST_FILE"
	const existingKey = "GRADUATION_TEST_EXISTING"
	_ = os.Unsetenv(fileKey)
	t.Cleanup(func() { _ = os.Unsetenv(fileKey) })
	t.Setenv(existingKey, "from-process")

	loadLocalEnvironment()

	if got := os.Getenv(fileKey); got != "from-file" {
		t.Fatalf("loaded value = %q, want from-file", got)
	}
	if got := os.Getenv(existingKey); got != "from-process" {
		t.Fatalf("existing value overwritten: %q", got)
	}
}
