package main

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

var firstClassContainerSettingKeys = []string{
	"SERVER_NAME",
	"SERVER_DESCRIPTION",
	"PLAYERS",
	"SERVER_PLAYER_MAX_NUM",
	"SERVER_PASSWORD",
	"ADMIN_PASSWORD",
	"COMMUNITY",
	"RCON_ENABLED",
	"REST_API_ENABLED",
	"EXP_RATE",
	"PAL_CAPTURE_RATE",
	"PAL_SPAWN_NUM_RATE",
	"COLLECTION_DROP_RATE",
	"ENEMY_DROP_ITEM_RATE",
	"PAL_EGG_DEFAULT_HATCHING_TIME",
	"AUTO_SAVE_SPAN",
	"DEATH_PENALTY",
	"BASE_CAMP_WORKER_MAX_NUM",
	"GUILD_PLAYER_MAX_NUM",
	"BASE_CAMP_MAX_NUM_IN_GUILD",
	"CROSSPLAY_PLATFORMS",
	"AUTO_PAUSE_ENABLED",
	"ENABLE_PLAYER_LOGGING",
	"DISCORD_WEBHOOK_ENABLED",
	"TARGET_MANIFEST_ID",
}

func (a *App) verifyContainerSettingsApplied(ctx context.Context) (int, error) {
	inspect := a.dockerInspect(ctx)
	if inspect == nil || !inspect.State.Running {
		return 0, fmt.Errorf("游戏容器重建后无法读取运行状态")
	}
	pending := pendingContainerSettingKeys(a.readSettings(), inspect)
	if len(pending) > 0 {
		visible := pending
		if len(visible) > 8 {
			visible = visible[:8]
		}
		message := strings.Join(visible, "、")
		if len(pending) > len(visible) {
			message += fmt.Sprintf(" 等 %d 项", len(pending))
		}
		return 0, fmt.Errorf("游戏容器已重建，但配置仍未加载：%s", message)
	}
	return len(desiredContainerSettings(a.readSettings())), nil
}

func desiredContainerSettings(settings ServerSettings) map[string]string {
	all := settingsToEnv(settings)
	desired := make(map[string]string, len(firstClassContainerSettingKeys)+len(gameParameterDefaults))
	for _, key := range firstClassContainerSettingKeys {
		desired[key] = all[key]
	}
	for key := range gameParameterDefaults {
		desired[key] = all[key]
	}
	return desired
}

func pendingContainerSettingKeys(settings ServerSettings, inspect *dockerInspectResult) []string {
	if inspect == nil {
		return []string{}
	}
	current := parseContainerEnvironment(inspect.Config.Env)
	desired := desiredContainerSettings(settings)
	pending := make([]string, 0)
	for key, value := range desired {
		if !sameContainerSetting(value, current[key]) {
			pending = append(pending, key)
		}
	}
	sort.Strings(pending)
	return pending
}

func parseContainerEnvironment(values []string) map[string]string {
	result := make(map[string]string, len(values))
	for _, value := range values {
		key, item, ok := strings.Cut(value, "=")
		if ok {
			result[key] = item
		}
	}
	return result
}

func sameContainerSetting(expected, actual string) bool {
	expected = strings.TrimSpace(expected)
	actual = strings.TrimSpace(actual)
	if strings.EqualFold(expected, actual) {
		return true
	}
	if expectedBool, ok := canonicalGameBoolean(expected); ok {
		if actualBool, actualOK := canonicalGameBoolean(actual); actualOK {
			return expectedBool == actualBool
		}
	}
	expectedNumber, expectedErr := strconv.ParseFloat(expected, 64)
	actualNumber, actualErr := strconv.ParseFloat(actual, 64)
	return expectedErr == nil && actualErr == nil && expectedNumber == actualNumber
}
