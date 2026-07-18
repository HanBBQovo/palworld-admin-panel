package main

import (
	"sort"
	"testing"
)

func TestDesiredContainerSettingsCoversGameAndFirstClassKeys(t *testing.T) {
	desired := desiredContainerSettings(envSettings())
	want := len(gameParameterDefaults) + len(firstClassContainerSettingKeys)
	if len(desired) != want {
		t.Fatalf("expected %d managed container settings, got %d", want, len(desired))
	}
	if _, ok := desired["PALWORLD_EXP_RATE"]; ok {
		t.Fatal("panel-only alias must not be treated as a container setting")
	}
}

func TestPendingContainerSettingKeysDetectsSavedChanges(t *testing.T) {
	settings := envSettings()
	desired := desiredContainerSettings(settings)
	inspect := &dockerInspectResult{}
	for key, value := range desired {
		inspect.Config.Env = append(inspect.Config.Env, key+"="+value)
	}
	if pending := pendingContainerSettingKeys(settings, inspect); len(pending) != 0 {
		t.Fatalf("expected no pending settings, got %v", pending)
	}

	settings.ExpRate = 2
	settings.CollectionDropRate = 3
	pending := pendingContainerSettingKeys(settings, inspect)
	sort.Strings(pending)
	want := []string{"COLLECTION_DROP_RATE", "EXP_RATE"}
	if len(pending) != len(want) || pending[0] != want[0] || pending[1] != want[1] {
		t.Fatalf("unexpected pending settings: %v", pending)
	}
}

func TestSameContainerSettingNormalizesBooleansAndNumbers(t *testing.T) {
	cases := [][2]string{
		{"true", "True"},
		{"false", "0"},
		{"2", "2.000000"},
		{"(Steam,Xbox)", "(Steam,Xbox)"},
	}
	for _, values := range cases {
		if !sameContainerSetting(values[0], values[1]) {
			t.Errorf("expected %q and %q to match", values[0], values[1])
		}
	}
	if sameContainerSetting("2", "3") {
		t.Fatal("different numeric settings must not match")
	}
}
