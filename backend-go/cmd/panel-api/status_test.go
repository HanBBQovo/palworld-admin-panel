package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseGameVersion(t *testing.T) {
	got := parseGameVersion("Welcome to Pal Server[v0.7.3.90464] Hanhan Palworld 116")
	if got != "v0.7.3.90464" {
		t.Fatalf("expected game version, got %q", got)
	}
}

func TestParsePlayersKeepsOnlyRconFields(t *testing.T) {
	players := parsePlayers("name,playeruid,steamid\nAlice,uid-1,76561198000000001")
	if len(players) != 1 {
		t.Fatalf("expected one player, got %d", len(players))
	}
	player := players[0]
	if player.Name != "Alice" || player.PlayerUID != "uid-1" || player.SteamID != "76561198000000001" {
		t.Fatalf("unexpected player: %#v", player)
	}
	if !player.Manageable {
		t.Fatalf("expected player with complete Steam ID to be manageable")
	}
}

func TestParsePlayersHandlesPalworldNullPaddedSteamID(t *testing.T) {
	players := parsePlayers("name,playeruid,steamid\n奶思兔米鱿,F532F39D000000000000000000000000,76561199\x00\x00\x00\x00\n")
	if len(players) != 1 {
		t.Fatalf("expected one player, got %d", len(players))
	}
	player := players[0]
	if player.Name != "奶思兔米鱿" || player.PlayerUID != "F532F39D000000000000000000000000" {
		t.Fatalf("unexpected player: %#v", player)
	}
	if player.SteamID != "-" || player.Manageable {
		t.Fatalf("expected truncated Steam ID to be unavailable: %#v", player)
	}
}

func TestParseOnlinePlayersFromServerLogsTracksLifecycle(t *testing.T) {
	logs := "Running Palworld dedicated server on :8211\n" +
		"[LOG] Alice joined the server. (User id: steam_76561198000000001, Player id: AAAA)\n" +
		"[LOG] 略略略 joined the server. (User id: steam_76561198826677646, Player id: BBBB)\n" +
		"[LOG] Alice left the server. (User id: steam_76561198000000001)\n"
	players, reliable := parseOnlinePlayersFromServerLogs(logs)
	if !reliable {
		t.Fatal("expected lifecycle logs to be reliable")
	}
	if len(players) != 1 || players[0].Name != "略略略" {
		t.Fatalf("unexpected online players: %#v", players)
	}
	if players[0].SteamID != "76561198826677646" || !players[0].Manageable {
		t.Fatalf("expected complete Steam identity: %#v", players[0])
	}
}

func TestParseOnlinePlayersFromServerLogsResetsAfterGameRestart(t *testing.T) {
	logs := "[LOG] Alice joined the server. (User id: steam_76561198000000001, Player id: AAAA)\n" +
		"Running Palworld dedicated server on :8211\n"
	players, reliable := parseOnlinePlayersFromServerLogs(logs)
	if reliable || len(players) != 0 {
		t.Fatalf("expected pre-restart lifecycle to be discarded: reliable=%v players=%#v", reliable, players)
	}
}

func TestParseOnlinePlayersFromServerLogsKeepsEmptyOnlineState(t *testing.T) {
	logs := "Running Palworld dedicated server on :8211\n" +
		"[LOG] Alice joined the server. (User id: steam_76561198000000001, Player id: AAAA)\n" +
		"[LOG] Alice left the server. (User id: steam_76561198000000001)\n"
	players, reliable := parseOnlinePlayersFromServerLogs(logs)
	if !reliable || len(players) != 0 {
		t.Fatalf("expected a reliable empty online state: reliable=%v players=%#v", reliable, players)
	}
}

func TestClonePlayersKeepsEmptySliceAsJSONArray(t *testing.T) {
	raw, err := json.Marshal(clonePlayers(nil))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "[]" {
		t.Fatalf("expected empty JSON array, got %s", raw)
	}
}

func TestCleanLogMessageStripsAnsiEscapes(t *testing.T) {
	got := cleanLogMessage("\x1b[36mWHAT\x1b[0m: test\x00")
	if got != "WHAT: test" {
		t.Fatalf("unexpected cleaned log message: %q", got)
	}
}

func TestParseDiskUsageWithWrappedFilesystemName(t *testing.T) {
	output := "Filesystem 1024-blocks Used Available Capacity Mounted on\n" +
		"/dev/mapper/ubuntu--vg-ubuntu--lv\n" +
		"1843285304 555553232 1211716984 31% /stack\n"
	got := parseDiskUsage(output)
	if got.TotalGB != 1757.9 || got.UsedGB != 529.8 {
		t.Fatalf("unexpected disk usage: %#v", got)
	}
}

func TestConfiguredTimezoneFormatsUTCInput(t *testing.T) {
	previous := displayLocation
	t.Cleanup(func() { displayLocation = previous })
	setDisplayTimezone("Asia/Shanghai")
	value := formatTime(time.Date(2026, 7, 9, 1, 3, 31, 0, time.UTC))
	if value != "2026-07-09 09:03:31" {
		t.Fatalf("unexpected formatted time: %s", value)
	}
}
