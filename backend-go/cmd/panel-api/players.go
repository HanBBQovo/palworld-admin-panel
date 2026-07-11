package main

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	playerJoinedLogPattern = regexp.MustCompile(`\[LOG\]\s+(.+?) joined the server\. \(User id:\s*([^,\s)]+),\s*Player id:\s*([^)]+)\)`)
	playerLeftLogPattern   = regexp.MustCompile(`\[LOG\]\s+(.+?) left the server\. \(User id:\s*([^)\s]+)\)`)
)

func (a *App) loadPlayersFromServerLogs() ([]Player, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()
	output, err := runCmd(ctx, "", "docker", "logs", "--timestamps", "--since", "48h", "--tail", "50000", a.cfg.Container)
	if err != nil {
		return nil, false, err
	}
	players, reliable := parseOnlinePlayersFromServerLogs(string(output))
	return players, reliable, nil
}

func parseOnlinePlayersFromServerLogs(output string) ([]Player, bool) {
	online := make(map[string]Player)
	observedLifecycle := false
	for _, rawLine := range strings.Split(output, "\n") {
		line := cleanLogMessage(rawLine)
		if strings.Contains(line, "Running Palworld dedicated server on") {
			online = make(map[string]Player)
			observedLifecycle = false
			continue
		}
		if match := playerJoinedLogPattern.FindStringSubmatch(line); len(match) == 4 {
			name := strings.TrimSpace(match[1])
			userID := strings.TrimSpace(match[2])
			playerUID := strings.TrimSpace(match[3])
			online[userID] = playerFromLifecycleLog(name, userID, playerUID)
			observedLifecycle = true
			continue
		}
		if match := playerLeftLogPattern.FindStringSubmatch(line); len(match) == 3 {
			delete(online, strings.TrimSpace(match[2]))
			observedLifecycle = true
		}
	}

	players := make([]Player, 0, len(online))
	for _, player := range online {
		players = append(players, player)
	}
	sort.Slice(players, func(i, j int) bool {
		left := strings.ToLower(players[i].Name) + "\x00" + players[i].ID
		right := strings.ToLower(players[j].Name) + "\x00" + players[j].ID
		return left < right
	})
	return players, observedLifecycle
}

func playerFromLifecycleLog(name, userID, playerUID string) Player {
	platform := "Unknown"
	steamID := "-"
	manageable := false
	if strings.HasPrefix(userID, "steam_") {
		platform = "Steam"
		steamID = strings.TrimPrefix(userID, "steam_")
		manageable = isCompleteSteamID(steamID)
		if !manageable {
			steamID = "-"
		}
	} else if strings.HasPrefix(strings.ToLower(userID), "xuid_") || strings.HasPrefix(strings.ToLower(userID), "xbox_") {
		platform = "Xbox"
	}
	id := playerUID
	if id == "" {
		id = userID
	}
	return Player{
		ID: id, Name: name, PlayerUID: orDefault(playerUID, "-"), Platform: platform,
		SteamID: steamID, Status: "online", Manageable: manageable,
	}
}
