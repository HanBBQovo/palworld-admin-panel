package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// These keys mirror palworld-server-docker 2.6.0's PalWorldSettings.ini
// template. Fields already represented by first-class ServerSettings values
// stay out of this map so there is only one source of truth per env variable.
var gameParameterDefaults = map[string]string{
	"DIFFICULTY":                                                   "None",
	"RANDOMIZER_TYPE":                                              "None",
	"RANDOMIZER_SEED":                                              "",
	"IS_RANDOMIZER_PAL_LEVEL_RANDOM":                               "False",
	"DAYTIME_SPEEDRATE":                                            "1.000000",
	"NIGHTTIME_SPEEDRATE":                                          "1.000000",
	"PAL_DAMAGE_RATE_ATTACK":                                       "1.000000",
	"PAL_DAMAGE_RATE_DEFENSE":                                      "1.000000",
	"PLAYER_DAMAGE_RATE_ATTACK":                                    "1.000000",
	"PLAYER_DAMAGE_RATE_DEFENSE":                                   "1.000000",
	"PLAYER_STOMACH_DECREASE_RATE":                                 "1.000000",
	"PLAYER_STAMINA_DECREASE_RATE":                                 "1.000000",
	"PLAYER_AUTO_HP_REGEN_RATE":                                    "1.000000",
	"PLAYER_AUTO_HP_REGEN_RATE_IN_SLEEP":                           "1.000000",
	"PAL_STOMACH_DECREASE_RATE":                                    "1.000000",
	"PAL_STAMINA_DECREASE_RATE":                                    "1.000000",
	"PAL_AUTO_HP_REGEN_RATE":                                       "1.000000",
	"PAL_AUTO_HP_REGEN_RATE_IN_SLEEP":                              "1.000000",
	"BUILD_OBJECT_HP_RATE":                                         "1.000000",
	"BUILD_OBJECT_DAMAGE_RATE":                                     "1.000000",
	"BUILD_OBJECT_DETERIORATION_DAMAGE_RATE":                       "1.000000",
	"COLLECTION_OBJECT_HP_RATE":                                    "1.000000",
	"COLLECTION_OBJECT_RESPAWN_SPEED_RATE":                         "1.000000",
	"ENABLE_PLAYER_TO_PLAYER_DAMAGE":                               "False",
	"ENABLE_FRIENDLY_FIRE":                                         "False",
	"ENABLE_INVADER_ENEMY":                                         "True",
	"ACTIVE_UNKO":                                                  "False",
	"ENABLE_AIM_ASSIST_PAD":                                        "True",
	"ENABLE_AIM_ASSIST_KEYBOARD":                                   "False",
	"DROP_ITEM_MAX_NUM":                                            "3000",
	"DROP_ITEM_MAX_NUM_UNKO":                                       "100",
	"BASE_CAMP_MAX_NUM":                                            "128",
	"DROP_ITEM_ALIVE_MAX_HOURS":                                    "1.000000",
	"AUTO_RESET_GUILD_NO_ONLINE_PLAYERS":                           "False",
	"AUTO_RESET_GUILD_TIME_NO_ONLINE_PLAYERS":                      "72.000000",
	"WORK_SPEED_RATE":                                              "1.000000",
	"IS_MULTIPLAY":                                                 "False",
	"IS_PVP":                                                       "False",
	"HARDCORE":                                                     "False",
	"CHARACTER_RECREATE_IN_HARDCORE":                               "False",
	"PAL_LOST":                                                     "False",
	"CAN_PICKUP_OTHER_GUILD_DEATH_PENALTY_DROP":                    "False",
	"ENABLE_NON_LOGIN_PENALTY":                                     "True",
	"ENABLE_FAST_TRAVEL":                                           "True",
	"IS_START_LOCATION_SELECT_BY_MAP":                              "False",
	"EXIST_PLAYER_AFTER_LOGOUT":                                    "False",
	"ENABLE_DEFENSE_OTHER_GUILD_PLAYER":                            "False",
	"INVISIBLE_OTHER_GUILD_BASE_CAMP_AREA_FX":                      "False",
	"BUILD_AREA_LIMIT":                                             "False",
	"ITEM_WEIGHT_RATE":                                             "1.000000",
	"COOP_PLAYER_MAX_NUM":                                          "4",
	"REGION":                                                       "",
	"USEAUTH":                                                      "True",
	"BAN_LIST_URL":                                                 "https://b.palworldgame.com/api/banlist.txt",
	"SHOW_PLAYER_LIST":                                             "False",
	"CHAT_POST_LIMIT_PER_MINUTE":                                   "30",
	"USE_BACKUP_SAVE_DATA":                                         "True",
	"SUPPLY_DROP_SPAN":                                             "180",
	"ENABLE_PREDATOR_BOSS_PAL":                                     "True",
	"MAX_BUILDING_LIMIT_NUM":                                       "0",
	"SERVER_REPLICATE_PAWN_CULL_DISTANCE":                          "15000.000000",
	"ALLOW_GLOBAL_PALBOX_EXPORT":                                   "True",
	"ALLOW_GLOBAL_PALBOX_IMPORT":                                   "False",
	"EQUIPMENT_DURABILITY_DAMAGE_RATE":                             "1.000000",
	"ITEM_CONTAINER_FORCE_MARK_DIRTY_INTERVAL":                     "1.000000",
	"ITEM_CORRUPTION_MULTIPLIER":                                   "1.000000",
	"PHYSICS_ACTIVE_DROP_ITEM_MAX_NUM":                             "-1",
	"ALLOW_CLIENT_MOD":                                             "True",
	"PLAYER_DATA_PAL_STORAGE_UPDATE_CHECK_TICK_INTERVAL":           "1.000000",
	"LOG_FORMAT_TYPE":                                              "Text",
	"IS_SHOW_JOIN_LEFT_MESSAGE":                                    "True",
	"MONSTER_FARM_ACTION_SPEED_RATE":                               "1.000000",
	"DENY_TECHNOLOGY_LIST":                                         "",
	"GUILD_REJOIN_COOLDOWN_MINUTES":                                "0",
	"AUTO_TRANSFER_MASTER_CHECK_INTERVAL_SECONDS":                  "3600.000000",
	"AUTO_TRANSFER_MASTER_THRESHOLD_DAYS":                          "14",
	"MAX_GUILDS_PER_FRAME":                                         "10",
	"BLOCK_RESPAWN_TIME":                                           "5.000000",
	"RESPAWN_PENALTY_DURATION_THRESHOLD":                           "0.000000",
	"RESPAWN_PENALTY_TIME_SCALE":                                   "2.000000",
	"DISPLAY_PVP_ITEM_NUM_ON_WORLD_MAP_BASE_CAMP":                  "False",
	"DISPLAY_PVP_ITEM_NUM_ON_WORLD_MAP_PLAYER":                     "False",
	"ADDITIONAL_DROP_ITEM_WHEN_PLAYER_KILLING_IN_PVP_MODE":         "PlayerDropItem",
	"ADDITIONAL_DROP_ITEM_NUM_WHEN_PLAYER_KILLING_IN_PVP_MODE":     "1",
	"ADDITIONAL_DROP_ITEM_WHEN_PLAYER_KILLING_IN_PVP_MODE_ENABLED": "False",
	"ENABLE_VOICE_CHAT":                                            "False",
	"VOICE_CHAT_MAX_VOLUME_DISTANCE":                               "3000.000000",
	"VOICE_CHAT_ZERO_VOLUME_DISTANCE":                              "15000.000000",
	"ALLOW_ENHANCE_STAT_HEALTH":                                    "True",
	"ALLOW_ENHANCE_STAT_ATTACK":                                    "True",
	"ALLOW_ENHANCE_STAT_STAMINA":                                   "True",
	"ALLOW_ENHANCE_STAT_WEIGHT":                                    "True",
	"ALLOW_ENHANCE_STAT_WORK_SPEED":                                "True",
	"ENABLE_BUILDING_PLAYER_UID_DISPLAY":                           "False",
	"BUILDING_NAME_DISPLAY_CACHE_TTL_SECONDS":                      "60",
}

func gameParametersFromEnvironment(fileEnv map[string]string) map[string]string {
	values := make(map[string]string, len(gameParameterDefaults))
	for key, fallback := range gameParameterDefaults {
		values[key] = envValueAny(fileEnv, fallback, key)
	}
	return values
}

func completeGameParameters(values, fallback map[string]string) map[string]string {
	result := make(map[string]string, len(gameParameterDefaults))
	for key, defaultValue := range gameParameterDefaults {
		value := defaultValue
		if current, ok := fallback[key]; ok {
			value = current
		}
		if next, ok := values[key]; ok {
			value = next
		}
		result[key] = value
	}
	return result
}

func normalizeGameParameters(values map[string]string) (map[string]string, error) {
	result := make(map[string]string, len(gameParameterDefaults))
	for key := range values {
		if _, ok := gameParameterDefaults[key]; !ok {
			return nil, fmt.Errorf("不支持的游戏参数：%s", key)
		}
	}

	for key, defaultValue := range gameParameterDefaults {
		value := strings.TrimSpace(values[key])
		if strings.ContainsAny(value, "\r\n") || len(value) > 2048 {
			return nil, fmt.Errorf("游戏参数 %s 包含非法内容", key)
		}

		if isBooleanGameParameter(defaultValue) {
			canonical, ok := canonicalGameBoolean(value)
			if !ok {
				return nil, fmt.Errorf("游戏参数 %s 必须是布尔值", key)
			}
			result[key] = canonical
			continue
		}

		if _, err := strconv.ParseFloat(defaultValue, 64); err == nil {
			number, err := strconv.ParseFloat(value, 64)
			if err != nil || math.IsInf(number, 0) || math.IsNaN(number) {
				return nil, fmt.Errorf("游戏参数 %s 必须是有限数值", key)
			}
			result[key] = strconv.FormatFloat(number, 'f', -1, 64)
			continue
		}

		result[key] = value
	}
	return result, nil
}

func isBooleanGameParameter(defaultValue string) bool {
	return strings.EqualFold(defaultValue, "true") || strings.EqualFold(defaultValue, "false")
}

func canonicalGameBoolean(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return "True", true
	case "0", "false", "no", "off":
		return "False", true
	default:
		return "", false
	}
}
