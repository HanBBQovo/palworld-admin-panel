package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const expectedGameParameterKeys = `
DIFFICULTY
RANDOMIZER_TYPE
RANDOMIZER_SEED
IS_RANDOMIZER_PAL_LEVEL_RANDOM
DAYTIME_SPEEDRATE
NIGHTTIME_SPEEDRATE
PAL_DAMAGE_RATE_ATTACK
PAL_DAMAGE_RATE_DEFENSE
PLAYER_DAMAGE_RATE_ATTACK
PLAYER_DAMAGE_RATE_DEFENSE
PLAYER_STOMACH_DECREASE_RATE
PLAYER_STAMINA_DECREASE_RATE
PLAYER_AUTO_HP_REGEN_RATE
PLAYER_AUTO_HP_REGEN_RATE_IN_SLEEP
PAL_STOMACH_DECREASE_RATE
PAL_STAMINA_DECREASE_RATE
PAL_AUTO_HP_REGEN_RATE
PAL_AUTO_HP_REGEN_RATE_IN_SLEEP
BUILD_OBJECT_HP_RATE
BUILD_OBJECT_DAMAGE_RATE
BUILD_OBJECT_DETERIORATION_DAMAGE_RATE
COLLECTION_OBJECT_HP_RATE
COLLECTION_OBJECT_RESPAWN_SPEED_RATE
ENABLE_PLAYER_TO_PLAYER_DAMAGE
ENABLE_FRIENDLY_FIRE
ENABLE_INVADER_ENEMY
ACTIVE_UNKO
ENABLE_AIM_ASSIST_PAD
ENABLE_AIM_ASSIST_KEYBOARD
DROP_ITEM_MAX_NUM
DROP_ITEM_MAX_NUM_UNKO
BASE_CAMP_MAX_NUM
DROP_ITEM_ALIVE_MAX_HOURS
AUTO_RESET_GUILD_NO_ONLINE_PLAYERS
AUTO_RESET_GUILD_TIME_NO_ONLINE_PLAYERS
WORK_SPEED_RATE
IS_MULTIPLAY
IS_PVP
HARDCORE
CHARACTER_RECREATE_IN_HARDCORE
PAL_LOST
CAN_PICKUP_OTHER_GUILD_DEATH_PENALTY_DROP
ENABLE_NON_LOGIN_PENALTY
ENABLE_FAST_TRAVEL
IS_START_LOCATION_SELECT_BY_MAP
EXIST_PLAYER_AFTER_LOGOUT
ENABLE_DEFENSE_OTHER_GUILD_PLAYER
INVISIBLE_OTHER_GUILD_BASE_CAMP_AREA_FX
BUILD_AREA_LIMIT
ITEM_WEIGHT_RATE
COOP_PLAYER_MAX_NUM
REGION
USEAUTH
BAN_LIST_URL
SHOW_PLAYER_LIST
CHAT_POST_LIMIT_PER_MINUTE
USE_BACKUP_SAVE_DATA
SUPPLY_DROP_SPAN
ENABLE_PREDATOR_BOSS_PAL
MAX_BUILDING_LIMIT_NUM
SERVER_REPLICATE_PAWN_CULL_DISTANCE
ALLOW_GLOBAL_PALBOX_EXPORT
ALLOW_GLOBAL_PALBOX_IMPORT
EQUIPMENT_DURABILITY_DAMAGE_RATE
ITEM_CONTAINER_FORCE_MARK_DIRTY_INTERVAL
ITEM_CORRUPTION_MULTIPLIER
PHYSICS_ACTIVE_DROP_ITEM_MAX_NUM
ALLOW_CLIENT_MOD
PLAYER_DATA_PAL_STORAGE_UPDATE_CHECK_TICK_INTERVAL
LOG_FORMAT_TYPE
IS_SHOW_JOIN_LEFT_MESSAGE
MONSTER_FARM_ACTION_SPEED_RATE
DENY_TECHNOLOGY_LIST
GUILD_REJOIN_COOLDOWN_MINUTES
AUTO_TRANSFER_MASTER_CHECK_INTERVAL_SECONDS
AUTO_TRANSFER_MASTER_THRESHOLD_DAYS
MAX_GUILDS_PER_FRAME
BLOCK_RESPAWN_TIME
RESPAWN_PENALTY_DURATION_THRESHOLD
RESPAWN_PENALTY_TIME_SCALE
DISPLAY_PVP_ITEM_NUM_ON_WORLD_MAP_BASE_CAMP
DISPLAY_PVP_ITEM_NUM_ON_WORLD_MAP_PLAYER
ADDITIONAL_DROP_ITEM_WHEN_PLAYER_KILLING_IN_PVP_MODE
ADDITIONAL_DROP_ITEM_NUM_WHEN_PLAYER_KILLING_IN_PVP_MODE
ADDITIONAL_DROP_ITEM_WHEN_PLAYER_KILLING_IN_PVP_MODE_ENABLED
ENABLE_VOICE_CHAT
VOICE_CHAT_MAX_VOLUME_DISTANCE
VOICE_CHAT_ZERO_VOLUME_DISTANCE
ALLOW_ENHANCE_STAT_HEALTH
ALLOW_ENHANCE_STAT_ATTACK
ALLOW_ENHANCE_STAT_STAMINA
ALLOW_ENHANCE_STAT_WEIGHT
ALLOW_ENHANCE_STAT_WORK_SPEED
ENABLE_BUILDING_PLAYER_UID_DISPLAY
BUILDING_NAME_DISPLAY_CACHE_TTL_SECONDS
`

func TestGameParameterDefaultsMatchContainerTemplate(t *testing.T) {
	expected := strings.Fields(expectedGameParameterKeys)
	if len(expected) != 95 {
		t.Fatalf("test fixture must contain 95 extra game parameters, got %d", len(expected))
	}
	if len(gameParameterDefaults) != len(expected) {
		t.Fatalf("expected %d game parameters, got %d", len(expected), len(gameParameterDefaults))
	}
	for _, key := range expected {
		if _, ok := gameParameterDefaults[key]; !ok {
			t.Errorf("missing game parameter %s", key)
		}
	}
}

func TestSettingsToEnvCoversEditableContainerTemplate(t *testing.T) {
	settings := ServerSettings{
		Players:            32,
		PublicPort:         "8211",
		CrossplayPlatforms: []string{"Steam", "Xbox", "PS5", "Mac"},
		GameParameters:     completeGameParameters(nil, gameParameterDefaults),
	}
	updates := settingsToEnv(settings)
	firstClassKeys := strings.Fields(`
		EXP_RATE PAL_CAPTURE_RATE PAL_SPAWN_NUM_RATE COLLECTION_DROP_RATE
		ENEMY_DROP_ITEM_RATE DEATH_PENALTY BASE_CAMP_WORKER_MAX_NUM
		GUILD_PLAYER_MAX_NUM BASE_CAMP_MAX_NUM_IN_GUILD PAL_EGG_DEFAULT_HATCHING_TIME
		AUTO_SAVE_SPAN SERVER_PLAYER_MAX_NUM SERVER_NAME SERVER_DESCRIPTION
		ADMIN_PASSWORD SERVER_PASSWORD PUBLIC_PORT PUBLIC_IP RCON_ENABLED
		REST_API_ENABLED CROSSPLAY_PLATFORMS
	`)

	for _, key := range append(strings.Fields(expectedGameParameterKeys), firstClassKeys...) {
		if _, ok := updates[key]; !ok {
			t.Errorf("settingsToEnv does not map %s", key)
		}
	}
}

func TestNormalizeGameParametersCanonicalizesValues(t *testing.T) {
	values := completeGameParameters(nil, gameParameterDefaults)
	values["ENABLE_FRIENDLY_FIRE"] = "yes"
	values["DAYTIME_SPEEDRATE"] = "2.500000"

	normalized, err := normalizeGameParameters(values)
	if err != nil {
		t.Fatal(err)
	}
	if normalized["ENABLE_FRIENDLY_FIRE"] != "True" {
		t.Fatalf("unexpected boolean value: %q", normalized["ENABLE_FRIENDLY_FIRE"])
	}
	if normalized["DAYTIME_SPEEDRATE"] != "2.5" {
		t.Fatalf("unexpected numeric value: %q", normalized["DAYTIME_SPEEDRATE"])
	}
}

func TestNormalizeGameParametersRejectsUnknownAndInvalidValues(t *testing.T) {
	values := completeGameParameters(nil, gameParameterDefaults)
	values["UNKNOWN_SETTING"] = "1"
	if _, err := normalizeGameParameters(values); err == nil {
		t.Fatal("expected unknown parameter to be rejected")
	}

	delete(values, "UNKNOWN_SETTING")
	values["ENABLE_FRIENDLY_FIRE"] = "sometimes"
	if _, err := normalizeGameParameters(values); err == nil {
		t.Fatal("expected invalid boolean to be rejected")
	}
}

func TestGameParameterEnvRoundTrip(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), ".env")
	updates := map[string]string{
		"CROSSPLAY_PLATFORMS":  "(Steam,Xbox,PS5,Mac)",
		"DENY_TECHNOLOGY_LIST": "",
		"RANDOMIZER_SEED":      "seed with spaces #1",
	}
	if err := updateEnvValues(envFile, updates); err != nil {
		t.Fatal(err)
	}

	readBack := readEnvFile(envFile)
	for key, want := range updates {
		if got := readBack[key]; got != want {
			t.Errorf("%s round trip: got %q, want %q", key, got, want)
		}
	}
	info, err := os.Stat(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("unexpected env permissions: %o", info.Mode().Perm())
	}
}

func TestSaveSettingsHandlerRejectsUnknownGameParameter(t *testing.T) {
	tempDir := t.TempDir()
	app := &App{cfg: Config{
		SettingsFile: filepath.Join(tempDir, "settings.json"),
		EnvFile:      filepath.Join(tempDir, ".env"),
		WriteEnv:     false,
	}}
	settings := envSettings()
	settings.Players = 32
	settings.GameParameters["UNKNOWN_SETTING"] = "1"
	body, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/palworld/settings", bytes.NewReader(body))
	app.handleSaveSettings(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
}
