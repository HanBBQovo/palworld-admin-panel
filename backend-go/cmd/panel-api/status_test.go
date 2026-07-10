package main

import (
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
