package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestEnvironmentWithoutKeysLetsComposeEnvFileWin(t *testing.T) {
	environment := []string{
		"PATH=/usr/bin",
		"PAL_STAMINA_DECREASE_RATE=1",
		"EXP_RATE=1",
		"PANEL_API_PORT=16825",
	}
	fileValues := map[string]string{
		"PAL_STAMINA_DECREASE_RATE": "0.1",
		"EXP_RATE":                  "2",
	}

	got := environmentWithoutKeys(environment, fileValues)
	want := []string{"PATH=/usr/bin", "PANEL_API_PORT=16825"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected Compose process environment: got %v, want %v", got, want)
	}
}

func TestRunComposeUsesEnvFileWithoutInheritedSettingOverrides(t *testing.T) {
	tempDir := t.TempDir()
	dockerPath := filepath.Join(tempDir, "docker")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" \"${PAL_STAMINA_DECREASE_RATE-unset}\"\n"
	if err := os.WriteFile(dockerPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	envFile := filepath.Join(tempDir, ".env")
	if err := os.WriteFile(envFile, []byte("PAL_STAMINA_DECREASE_RATE=0.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tempDir)
	t.Setenv("PAL_STAMINA_DECREASE_RATE", "1")

	app := &App{cfg: Config{ComposeDir: tempDir, ComposeProject: "test-project", EnvFile: envFile}}
	output, err := app.runCompose(context.Background(), "config")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) != 2 {
		t.Fatalf("unexpected fake Docker output: %q", output)
	}
	wantArgs := "compose --env-file " + envFile + " -p test-project config"
	if lines[0] != wantArgs {
		t.Fatalf("unexpected Compose arguments: got %q, want %q", lines[0], wantArgs)
	}
	if lines[1] != "unset" {
		t.Fatalf("inherited setting still overrides the env file: %q", lines[1])
	}
}
