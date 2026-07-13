package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type AdvancedLayer struct {
	ID              string `json:"id"`
	Label           string `json:"label"`
	State           string `json:"state"`
	Installed       bool   `json:"installed"`
	Reachable       bool   `json:"reachable"`
	ReadOnly        bool   `json:"readOnly"`
	RequiresRestart bool   `json:"requiresRestart"`
	Source          string `json:"source"`
	Message         string `json:"message"`
}

type AdvancedSafety struct {
	GameRunning       bool `json:"gameRunning"`
	PlayersOnline     int  `json:"playersOnline"`
	SnapshotAvailable bool `json:"snapshotAvailable"`
	CanEditSnapshot   bool `json:"canEditSnapshot"`
	CanApplyToWorld   bool `json:"canApplyToWorld"`
	ApplyEnabled      bool `json:"applyEnabled"`
}

type AdvancedCapabilities struct {
	Layers     []AdvancedLayer `json:"layers"`
	Safety     AdvancedSafety  `json:"safety"`
	ObservedAt string          `json:"observedAt"`
}

type LiveMetrics struct {
	ServerFPS       int     `json:"serverFps"`
	CurrentPlayers  int     `json:"currentPlayers"`
	MaxPlayers      int     `json:"maxPlayers"`
	ServerFrameTime float64 `json:"serverFrameTime"`
	UptimeSeconds   int     `json:"uptimeSeconds"`
	InGameDays      int     `json:"inGameDays"`
	Source          string  `json:"source"`
	ObservedAt      string  `json:"observedAt"`
}

type RESTPlayer struct {
	Name          string  `json:"name"`
	AccountName   string  `json:"accountName"`
	PlayerID      string  `json:"playerId"`
	UserID        string  `json:"userId"`
	IP            string  `json:"ip"`
	Ping          float64 `json:"ping"`
	LocationX     float64 `json:"location_x"`
	LocationY     float64 `json:"location_y"`
	Level         int     `json:"level"`
	BuildingCount int     `json:"building_count"`
}

type WorldPal struct {
	Level     int      `json:"level"`
	Type      string   `json:"type"`
	Gender    string   `json:"gender"`
	Nickname  string   `json:"nickname"`
	Lucky     bool     `json:"is_lucky"`
	Boss      bool     `json:"is_boss"`
	Workspeed int      `json:"workspeed"`
	Melee     int      `json:"melee"`
	Ranged    int      `json:"ranged"`
	Defense   int      `json:"defense"`
	Skills    []string `json:"skills"`
}

type WorldItem struct {
	SlotIndex  int    `json:"SlotIndex"`
	ItemID     string `json:"ItemId"`
	StackCount int    `json:"StackCount"`
}

type WorldItems struct {
	Inventory []WorldItem `json:"CommonContainerId"`
	Drops     []WorldItem `json:"DropSlotContainerId"`
	Essential []WorldItem `json:"EssentialContainerId"`
	Food      []WorldItem `json:"FoodEquipContainerId"`
	Armor     []WorldItem `json:"PlayerEquipArmorContainerId"`
	Weapons   []WorldItem `json:"WeaponLoadOutContainerId"`
}

type WorldPlayer struct {
	PlayerUID      string      `json:"player_uid"`
	Nickname       string      `json:"nickname"`
	Level          int         `json:"level"`
	Exp            int64       `json:"exp"`
	HP             int64       `json:"hp"`
	MaxHP          int64       `json:"max_hp"`
	ShieldHP       int64       `json:"shield_hp"`
	ShieldMaxHP    int64       `json:"shield_max_hp"`
	FullStomach    float64     `json:"full_stomach"`
	SaveLastOnline string      `json:"save_last_online"`
	LastOnline     string      `json:"last_online"`
	SteamID        string      `json:"steam_id"`
	UserID         string      `json:"user_id"`
	AccountName    string      `json:"account_name"`
	IP             string      `json:"ip"`
	Ping           float64     `json:"ping"`
	LocationX      float64     `json:"location_x"`
	LocationY      float64     `json:"location_y"`
	BuildingCount  int         `json:"building_count"`
	Pals           []WorldPal  `json:"pals,omitempty"`
	Items          *WorldItems `json:"items,omitempty"`
}

type GuildPlayer struct {
	PlayerUID string `json:"player_uid"`
	Nickname  string `json:"nickname"`
}

type BaseCamp struct {
	ID        string  `json:"id"`
	Area      float64 `json:"area"`
	LocationX float64 `json:"location_x"`
	LocationY float64 `json:"location_y"`
}

type WorldGuild struct {
	Name           string        `json:"name"`
	BaseCampLevel  int           `json:"base_camp_level"`
	AdminPlayerUID string        `json:"admin_player_uid"`
	Players        []GuildPlayer `json:"players"`
	BaseCamps      []BaseCamp    `json:"base_camp"`
}

type DataMeta struct {
	Source     string `json:"source"`
	ObservedAt string `json:"observedAt"`
	Stale      bool   `json:"stale"`
	SnapshotID string `json:"snapshotId,omitempty"`
}

type DataEnvelope struct {
	Meta DataMeta `json:"meta"`
	Data any      `json:"data"`
}

type WorldSnapshot struct {
	ID          string `json:"id"`
	BackupID    string `json:"backupId"`
	CreatedAt   string `json:"createdAt"`
	RefreshedAt string `json:"refreshedAt"`
	SourceDir   string `json:"sourceDir"`
}

type WorldStatus struct {
	Snapshot           WorldSnapshot `json:"snapshot"`
	IndexReachable     bool          `json:"indexReachable"`
	EditorInstalled    bool          `json:"editorInstalled"`
	LatestBackupID     string        `json:"latestBackupId"`
	UpToDate           bool          `json:"upToDate"`
	AutoRefreshSeconds int           `json:"autoRefreshSeconds"`
}

func (a *App) registerAdvancedRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/palworld/capabilities", a.authed("GET", a.handleAdvancedCapabilities))
	mux.HandleFunc("/api/palworld/live/metrics", a.authed("GET", a.handleLiveMetrics))
	mux.HandleFunc("/api/palworld/live/players", a.authed("GET", a.handleLivePlayers))
	mux.HandleFunc("/api/palworld/live/map", a.authed("GET", a.handleLiveMap))
	mux.HandleFunc("/api/palworld/announce", a.authed("POST", a.handleAnnouncement))
	mux.HandleFunc("/api/palworld/world/status", a.authed("GET", a.handleWorldStatus))
	mux.HandleFunc("/api/palworld/world/players", a.authed("GET", a.handleWorldPlayers))
	mux.HandleFunc("/api/palworld/world/players/", a.authed("GET", a.handleWorldPlayer))
	mux.HandleFunc("/api/palworld/world/guilds", a.authed("GET", a.handleWorldGuilds))
	mux.HandleFunc("/api/palworld/world/refresh", a.authed("POST", a.handleWorldRefresh))
	mux.HandleFunc("/api/palworld/editor/status", a.authed("GET", a.handleEditorStatus))
	mux.HandleFunc("/api/palworld/editor/session", a.authed("POST", a.handleEditorSession))
	mux.HandleFunc("/editor/", a.handleEditorProxy)
}

const maxAnnouncementRunes = 500

func (a *App) handleAnnouncement(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Message string `json:"message"`
	}
	if err := readJSON(r, &request); err != nil {
		writeError(w, APIError{Status: http.StatusBadRequest, Message: "广播内容不是合法 JSON"})
		return
	}
	message, err := normalizeAnnouncement(request.Message)
	if err != nil {
		writeError(w, err)
		return
	}
	transport := "Palworld REST API"
	if err := a.palREST(http.MethodPost, "/v1/api/announce", map[string]string{"message": message}, nil); err != nil {
		writeError(w, APIError{Status: http.StatusServiceUnavailable, Message: "广播发送失败: " + err.Error()})
		return
	}
	a.audit("info", "server", "已发送服务器广播", "admin", map[string]any{"transport": transport, "characters": utf8.RuneCountInString(message)})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "广播已发送", "transport": transport, "sentAt": formatTime(time.Now())})
}

func normalizeAnnouncement(value string) (string, error) {
	message := strings.TrimSpace(value)
	if message == "" {
		return "", APIError{Status: http.StatusBadRequest, Message: "广播内容不能为空"}
	}
	if !utf8.ValidString(message) {
		return "", APIError{Status: http.StatusBadRequest, Message: "广播内容不是有效 UTF-8 文本"}
	}
	if utf8.RuneCountInString(message) > maxAnnouncementRunes {
		return "", APIError{Status: http.StatusBadRequest, Message: fmt.Sprintf("广播内容不能超过 %d 个字符", maxAnnouncementRunes)}
	}
	return message, nil
}

func (a *App) handleAdvancedCapabilities(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.advancedCapabilities())
}

func (a *App) advancedCapabilities() AdvancedCapabilities {
	inspect := a.dockerInspect(context.Background())
	gameRunning := inspect != nil && inspect.State.Running
	players, _, _ := a.loadPlayersFromServerLogs()
	if !gameRunning {
		players = nil
	}
	runningREST := parseBool(a.containerEnvValue("REST_API_ENABLED", time.Second), false)
	desiredREST := parseBool(envValueAny(readEnvFile(a.cfg.EnvFile), "", "PALWORLD_REST_API_ENABLED", "REST_API_ENABLED"), false)
	restReachable := false
	if runningREST {
		var info map[string]any
		restReachable = a.palREST("GET", "/v1/api/info", nil, &info) == nil
	}
	indexReachable := a.worldIndexReachable()
	snapshotAvailable := a.worldSnapshotAvailable()
	editorInstalled := a.editorInstalled()
	editorReachable := a.editorReachable()
	canEdit := inspect != nil && !gameRunning && snapshotAvailable && editorInstalled && len(players) == 0
	canApply := a.cfg.EditorApply && canEdit

	restState, restMessage := "disabled", "REST API 尚未写入待应用配置"
	if desiredREST && !runningREST {
		restState, restMessage = "pending-restart", "已配置，等待下一次安全重启游戏后启用"
	} else if runningREST && restReachable {
		restState, restMessage = "ready", "实时玩家、地图和性能指标可用"
	} else if runningREST {
		restState, restMessage = "degraded", "游戏已启用 REST，但接口当前不可达"
	}
	indexState, indexMessage := "not-installed", "只读世界索引侧车尚未运行"
	if snapshotAvailable && !indexReachable {
		indexState, indexMessage = "snapshot-ready", "静态世界快照已准备，等待索引侧车"
	} else if indexReachable {
		indexState, indexMessage = "ready", "玩家、帕鲁、背包、公会和基地索引可用"
	}
	editorState, editorMessage := "not-installed", "存档编辑工具尚未安装"
	if editorInstalled {
		editorState, editorMessage = "locked", "编辑器已安装；只允许编辑快照，写回世界需停服维护"
	}
	if editorInstalled && editorReachable {
		editorState, editorMessage = "ready", "快照编辑会话已启动；生产世界写回仍受维护门禁保护"
	}
	return AdvancedCapabilities{
		Layers: []AdvancedLayer{
			{ID: "realtime", Label: "官方实时接口", State: restState, Installed: desiredREST || runningREST, Reachable: restReachable, ReadOnly: false, RequiresRestart: desiredREST && !runningREST, Source: "Palworld REST API", Message: restMessage},
			{ID: "world-index", Label: "世界存档索引", State: indexState, Installed: snapshotAvailable || indexReachable, Reachable: indexReachable, ReadOnly: true, Source: "Palworld Save Pal v0.17.4", Message: indexMessage},
			{ID: "save-editor", Label: "维护存档编辑器", State: editorState, Installed: editorInstalled, Reachable: editorReachable, ReadOnly: !canApply, Source: "Palworld Save Pal v0.17.4", Message: editorMessage},
		},
		Safety:     AdvancedSafety{GameRunning: gameRunning, PlayersOnline: len(players), SnapshotAvailable: snapshotAvailable, CanEditSnapshot: canEdit, CanApplyToWorld: canApply, ApplyEnabled: a.cfg.EditorApply},
		ObservedAt: formatTime(time.Now()),
	}
}

func (a *App) palREST(method, path string, body any, out any) error {
	if !parseBool(a.containerEnvValue("REST_API_ENABLED", time.Second), false) {
		return errors.New("Palworld REST API 尚未在运行中的游戏容器启用")
	}
	password := a.currentRconPassword(time.Second)
	if password == "" {
		return errors.New("管理员密码未配置")
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, a.cfg.RestURL+path, reader)
	if err != nil {
		return err
	}
	req.SetBasicAuth("admin", password)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Palworld REST 返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out == nil || len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func (a *App) restPlayers() ([]Player, error) {
	var payload struct {
		Players []RESTPlayer `json:"players"`
	}
	if err := a.palREST("GET", "/v1/api/players", nil, &payload); err != nil {
		return nil, err
	}
	players := make([]Player, 0, len(payload.Players))
	for _, row := range payload.Players {
		steamID := strings.TrimPrefix(row.UserID, "steam_")
		manageable := isCompleteSteamID(steamID)
		if !manageable {
			steamID = "-"
		}
		platform := "Unknown"
		if strings.HasPrefix(row.UserID, "steam_") {
			platform = "Steam"
		} else if strings.Contains(strings.ToLower(row.UserID), "xbox") || strings.HasPrefix(strings.ToLower(row.UserID), "xuid_") {
			platform = "Xbox"
		}
		id := strings.TrimSpace(row.PlayerID)
		if id == "" {
			id = row.UserID
		}
		players = append(players, Player{ID: id, Name: row.Name, PlayerUID: row.PlayerID, Platform: platform, SteamID: steamID, UserID: row.UserID, AccountName: row.AccountName, IP: row.IP, Ping: row.Ping, LocationX: row.LocationX, LocationY: row.LocationY, Level: row.Level, BuildingCount: row.BuildingCount, Status: "online", Manageable: manageable})
	}
	return players, nil
}

func (a *App) handleLivePlayers(w http.ResponseWriter, _ *http.Request) {
	players, source, err := a.livePlayers()
	if err != nil {
		writeError(w, APIError{Status: http.StatusBadGateway, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, DataEnvelope{Meta: DataMeta{Source: source, ObservedAt: formatTime(time.Now()), Stale: source != "Palworld REST API"}, Data: players})
}

func (a *App) livePlayers() ([]Player, string, error) {
	if players, err := a.restPlayers(); err == nil {
		return players, "Palworld REST API", nil
	}
	if players, reliable, err := a.loadPlayersFromServerLogs(); err == nil && reliable {
		return players, "server connection logs", nil
	}
	result, err := a.executeRcon("ShowPlayers", a.cfg.RconTimeout)
	if err != nil {
		return nil, "", err
	}
	return parsePlayers(result.Output), "Palworld RCON", nil
}

func (a *App) handleLiveMetrics(w http.ResponseWriter, _ *http.Request) {
	var payload struct {
		ServerFPS       int     `json:"serverfps"`
		CurrentPlayers  int     `json:"currentplayernum"`
		ServerFrameTime float64 `json:"serverframetime"`
		MaxPlayers      int     `json:"maxplayernum"`
		Uptime          int     `json:"uptime"`
		Days            int     `json:"days"`
	}
	if err := a.palREST("GET", "/v1/api/metrics", nil, &payload); err != nil {
		writeError(w, APIError{Status: http.StatusServiceUnavailable, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, LiveMetrics{ServerFPS: payload.ServerFPS, CurrentPlayers: payload.CurrentPlayers, MaxPlayers: payload.MaxPlayers, ServerFrameTime: payload.ServerFrameTime, UptimeSeconds: payload.Uptime, InGameDays: payload.Days, Source: "Palworld REST API", ObservedAt: formatTime(time.Now())})
}

func (a *App) handleLiveMap(w http.ResponseWriter, _ *http.Request) {
	players, source, err := a.livePlayers()
	if err != nil {
		writeError(w, APIError{Status: http.StatusBadGateway, Message: err.Error()})
		return
	}
	guilds := []WorldGuild{}
	_ = a.worldIndexGet("/api/guild", &guilds)
	writeJSON(w, http.StatusOK, DataEnvelope{Meta: DataMeta{Source: source + " + world snapshot", ObservedAt: formatTime(time.Now()), Stale: source != "Palworld REST API", SnapshotID: a.snapshotID()}, Data: map[string]any{"players": players, "guilds": guilds}})
}

func (a *App) worldIndexReachable() bool {
	var status struct {
		Ready     bool   `json:"ready"`
		Syncing   bool   `json:"syncing"`
		LastError string `json:"last_error"`
	}
	return a.worldIndexGet("/api/status", &status) == nil && status.Ready
}

func (a *App) worldIndexToken() (string, error) {
	raw, _ := json.Marshal(map[string]string{"password": a.cfg.WorldIndexPass})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.WorldIndexURL+"/api/login", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var payload struct {
		Token string `json:"token"`
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("世界索引登录失败: %s", strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	return payload.Token, nil
}

func (a *App) worldIndexRequest(method, path string, body any, out any) error {
	token, err := a.worldIndexToken()
	if err != nil {
		return err
	}
	var reader io.Reader
	if body != nil {
		raw, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			return marshalErr
		}
		reader = bytes.NewReader(raw)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, a.cfg.WorldIndexURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("世界索引返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out == nil || len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func (a *App) worldIndexGet(path string, out any) error {
	return a.worldIndexRequest(http.MethodGet, path, nil, out)
}

func (a *App) worldEnvelope(data any) DataEnvelope {
	return DataEnvelope{Meta: DataMeta{Source: "Palworld Save Pal v0.17.4 read-only index", ObservedAt: formatTime(time.Now()), SnapshotID: a.snapshotID()}, Data: data}
}

func (a *App) handleWorldStatus(w http.ResponseWriter, _ *http.Request) {
	snapshot, _ := a.readWorldSnapshot()
	latestBackupID, _ := a.latestStableBackupID()
	writeJSON(w, http.StatusOK, WorldStatus{
		Snapshot:           snapshot,
		IndexReachable:     a.worldIndexReachable(),
		EditorInstalled:    a.editorInstalled(),
		LatestBackupID:     latestBackupID,
		UpToDate:           latestBackupID != "" && snapshot.BackupID == latestBackupID && a.worldSnapshotAvailable(),
		AutoRefreshSeconds: int(a.cfg.WorldRefresh / time.Second),
	})
}

func (a *App) handleWorldPlayers(w http.ResponseWriter, _ *http.Request) {
	players := []WorldPlayer{}
	if err := a.worldIndexGet("/api/player?order_by=last_online&desc=true", &players); err != nil {
		writeError(w, APIError{Status: http.StatusServiceUnavailable, Message: "世界索引不可用: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, a.worldEnvelope(players))
}

func (a *App) handleWorldPlayer(w http.ResponseWriter, r *http.Request) {
	uid := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/palworld/world/players/"))
	if uid == "" || strings.Contains(uid, "/") {
		writeError(w, APIError{Status: http.StatusBadRequest, Message: "玩家 UID 无效"})
		return
	}
	var player WorldPlayer
	if err := a.worldIndexGet("/api/player/"+url.PathEscape(uid), &player); err != nil {
		writeError(w, APIError{Status: http.StatusBadGateway, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, a.worldEnvelope(player))
}

func (a *App) handleWorldGuilds(w http.ResponseWriter, _ *http.Request) {
	guilds := []WorldGuild{}
	if err := a.worldIndexGet("/api/guild", &guilds); err != nil {
		writeError(w, APIError{Status: http.StatusServiceUnavailable, Message: "世界索引不可用: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, a.worldEnvelope(guilds))
}

func (a *App) handleWorldRefresh(w http.ResponseWriter, _ *http.Request) {
	if !a.snapshotMu.TryLock() {
		writeError(w, APIError{Status: http.StatusConflict, Message: "世界快照刷新正在进行，请稍后重试"})
		return
	}
	defer a.snapshotMu.Unlock()
	snapshot, err := a.refreshWorldSnapshot()
	if err != nil {
		writeError(w, err)
		return
	}
	syncErr := a.worldIndexRequest(http.MethodPost, "/api/sync?from=sav", nil, nil)
	message := "已从完成的备份准备世界快照"
	if syncErr == nil {
		message += "，索引任务已启动"
	} else {
		message += "；侧车尚未可用，快照将在侧车启动后解析"
	}
	a.audit("info", "server", message, "admin", map[string]any{"snapshotId": snapshot.ID, "backupId": snapshot.BackupID})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": message, "snapshot": snapshot})
}

func (a *App) refreshWorldSnapshot() (WorldSnapshot, error) {
	backupPath, info, err := latestStableCompressedBackup(a.cfg.BackupsDir, time.Now(), 30*time.Second)
	if err != nil {
		return WorldSnapshot{}, APIError{Status: http.StatusNotFound, Message: err.Error()}
	}
	snapshotParent := filepath.Dir(a.cfg.WorldSnapshot)
	if err := os.MkdirAll(snapshotParent, 0o750); err != nil {
		return WorldSnapshot{}, fmt.Errorf("创建世界快照目录失败: %w", err)
	}
	stage, err := os.MkdirTemp(snapshotParent, ".world-index-stage-")
	if err != nil {
		return WorldSnapshot{}, err
	}
	defer os.RemoveAll(stage)
	if err := extractTarGz(backupPath, stage); err != nil {
		return WorldSnapshot{}, fmt.Errorf("解压静态备份失败: %w", err)
	}
	levelPath, err := findWorldLevelSave(stage)
	if err != nil {
		return WorldSnapshot{}, err
	}
	prepared := filepath.Join(stage, "prepared")
	if err := copyDir(filepath.Dir(levelPath), prepared); err != nil {
		return WorldSnapshot{}, err
	}
	previous := a.cfg.WorldSnapshot + ".previous"
	_ = os.RemoveAll(previous)
	if pathExists(a.cfg.WorldSnapshot) {
		if err := os.Rename(a.cfg.WorldSnapshot, previous); err != nil {
			return WorldSnapshot{}, err
		}
	}
	if err := os.Rename(prepared, a.cfg.WorldSnapshot); err != nil {
		if pathExists(previous) {
			_ = os.Rename(previous, a.cfg.WorldSnapshot)
		}
		return WorldSnapshot{}, err
	}
	_ = os.RemoveAll(previous)
	snapshot := WorldSnapshot{ID: randomID(), BackupID: filepath.Base(backupPath), CreatedAt: formatTime(info.ModTime()), RefreshedAt: formatTime(time.Now()), SourceDir: a.cfg.WorldSnapshot}
	raw, _ := json.MarshalIndent(snapshot, "", "  ")
	if err := os.WriteFile(a.worldSnapshotFile(), append(raw, '\n'), 0o600); err != nil {
		return WorldSnapshot{}, err
	}
	return snapshot, nil
}

func (a *App) startWorldSnapshotWatcher() {
	if a.cfg.WorldRefresh <= 0 {
		return
	}
	go func() {
		a.refreshWorldSnapshotInBackground()
		ticker := time.NewTicker(a.cfg.WorldRefresh)
		defer ticker.Stop()
		for range ticker.C {
			a.refreshWorldSnapshotInBackground()
		}
	}()
}

func (a *App) refreshWorldSnapshotInBackground() {
	if !a.snapshotMu.TryLock() {
		return
	}
	defer a.snapshotMu.Unlock()

	needsRefresh, latestBackupID, err := a.worldSnapshotNeedsRefresh()
	if err != nil {
		log.Printf("world snapshot watcher: %v", err)
		return
	}
	if !needsRefresh {
		return
	}
	snapshot, err := a.refreshWorldSnapshot()
	if err != nil {
		log.Printf("world snapshot watcher refresh %s: %v", latestBackupID, err)
		return
	}
	if err := a.worldIndexRequest(http.MethodPost, "/api/sync?from=sav", nil, nil); err != nil {
		log.Printf("world snapshot watcher index %s: %v", snapshot.BackupID, err)
		return
	}
	log.Printf("world snapshot watcher indexed %s", snapshot.BackupID)
}

func (a *App) worldSnapshotNeedsRefresh() (bool, string, error) {
	latestBackupID, err := a.latestStableBackupID()
	if err != nil {
		return false, "", err
	}
	snapshot, err := a.readWorldSnapshot()
	if err != nil {
		return true, latestBackupID, nil
	}
	return snapshot.BackupID != latestBackupID || !a.worldSnapshotAvailable(), latestBackupID, nil
}

func (a *App) latestStableBackupID() (string, error) {
	path, _, err := latestStableCompressedBackup(a.cfg.BackupsDir, time.Now(), 30*time.Second)
	if err != nil {
		return "", err
	}
	return filepath.Base(path), nil
}

func (a *App) readWorldSnapshot() (WorldSnapshot, error) {
	raw, err := os.ReadFile(a.worldSnapshotFile())
	if err != nil {
		return WorldSnapshot{}, err
	}
	var snapshot WorldSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return WorldSnapshot{}, err
	}
	return snapshot, nil
}

func latestCompressedBackup(dir string) (string, os.FileInfo, error) {
	return latestStableCompressedBackup(dir, time.Now(), 0)
}

func latestStableCompressedBackup(dir string, now time.Time, minimumAge time.Duration) (string, os.FileInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil, fmt.Errorf("无法读取备份目录: %w", err)
	}
	type candidate struct {
		path string
		info os.FileInfo
	}
	rows := make([]candidate, 0)
	for _, entry := range entries {
		if entry.IsDir() || backupFormatFor(entry.Name(), false) != "tar.gz" {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr == nil && now.Sub(info.ModTime()) >= minimumAge {
			rows = append(rows, candidate{path: filepath.Join(dir, entry.Name()), info: info})
		}
	}
	if len(rows) == 0 {
		return "", nil, errors.New("没有可用于世界索引的稳定压缩备份")
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].info.ModTime().After(rows[j].info.ModTime()) })
	return rows[0].path, rows[0].info, nil
}

func findWorldLevelSave(root string) (string, error) {
	var matches []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && strings.EqualFold(entry.Name(), "backup") {
			return filepath.SkipDir
		}
		if !entry.IsDir() && entry.Name() == "Level.sav" {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("备份中应只有一个活动 Level.sav，实际找到 %d 个", len(matches))
	}
	return matches[0], nil
}

func (a *App) handleEditorStatus(w http.ResponseWriter, _ *http.Request) {
	capabilities := a.advancedCapabilities()
	writeJSON(w, http.StatusOK, map[string]any{"installed": a.editorInstalled(), "reachable": a.editorReachable(), "url": a.cfg.EditorURL, "applyEnabled": a.cfg.EditorApply, "safety": capabilities.Safety, "supportedActions": []string{"player.stats", "player.inventory", "player.map", "pal.edit", "guild.edit", "player.transfer", "save.repair"}})
}

func (a *App) editorInstalled() bool {
	if pathExists(a.cfg.EditorDir) {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	_, err := runCmd(ctx, "", "docker", "image", "inspect", a.cfg.EditorImage)
	return err == nil
}

func (a *App) handleEditorSession(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Action string `json:"action"`
	}
	if err := readJSON(r, &request); err != nil {
		writeError(w, APIError{Status: http.StatusBadRequest, Message: "维护会话参数无效"})
		return
	}
	switch request.Action {
	case "start":
		result, err := a.startEditorSession()
		if err != nil {
			writeError(w, err)
			return
		}
		if err := a.issueEditorSession(w, r, result); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	case "open":
		inspect := a.dockerInspect(r.Context())
		if inspect == nil {
			writeError(w, APIError{Status: http.StatusServiceUnavailable, Message: "无法确认游戏服务器已停止，维护会话保持锁定"})
			return
		}
		if inspect.State.Running {
			writeError(w, APIError{Status: http.StatusConflict, Message: "游戏服务器仍在运行，不能打开存档编辑会话"})
			return
		}
		if !a.editorReachable() {
			writeError(w, APIError{Status: http.StatusServiceUnavailable, Message: "维护编辑器未运行"})
			return
		}
		result := map[string]any{"ok": true, "message": "维护编辑会话入口已生成"}
		if err := a.issueEditorSession(w, r, result); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	case "stop":
		if err := a.stopEditorSession(); err != nil {
			writeError(w, err)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "palworld_editor_session", Value: "", Path: "/editor/", MaxAge: -1, HttpOnly: true, Secure: r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"), SameSite: http.SameSiteStrictMode})
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "维护编辑会话已停止"})
	default:
		writeError(w, APIError{Status: http.StatusBadRequest, Message: "未知维护会话动作"})
	}
}

func (a *App) startEditorSession() (map[string]any, error) {
	inspect := a.dockerInspect(context.Background())
	if inspect == nil {
		return nil, APIError{Status: http.StatusServiceUnavailable, Message: "无法确认游戏服务器已停止，维护会话保持锁定"}
	}
	if inspect.State.Running {
		return nil, APIError{Status: http.StatusConflict, Message: "游戏服务器仍在运行，不能启动存档编辑会话"}
	}
	if a.editorReachable() {
		return map[string]any{"ok": true, "message": "维护编辑会话已在运行"}, nil
	}
	if !a.editorInstalled() {
		return nil, APIError{Status: http.StatusServiceUnavailable, Message: "存档编辑器尚未安装"}
	}
	if !a.worldSnapshotAvailable() {
		return nil, APIError{Status: http.StatusConflict, Message: "尚未准备静态世界快照"}
	}
	if err := a.prepareEditorWorkspace(); err != nil {
		return nil, fmt.Errorf("准备编辑工作区失败: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if _, err := a.runAdvancedCompose(ctx, "--profile", "maintenance-editor", "up", "-d", "save-editor"); err != nil {
		return nil, fmt.Errorf("启动存档编辑器失败: %w", err)
	}
	deadline := time.Now().Add(25 * time.Second)
	for !a.editorReachable() && time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
	}
	if !a.editorReachable() {
		_ = a.stopEditorSession()
		return nil, APIError{Status: http.StatusServiceUnavailable, Message: "存档编辑器启动后未通过健康检查"}
	}
	a.audit("warn", "server", "已从静态快照启动维护编辑会话", "admin", map[string]any{"workspace": a.cfg.EditorWorkspace})
	return map[string]any{"ok": true, "message": "维护编辑会话已启动"}, nil
}

func (a *App) issueEditorSession(w http.ResponseWriter, r *http.Request, result map[string]any) error {
	token, err := a.signTokenTTL("editor", 15*time.Minute)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: "palworld_editor_session", Value: token, Path: "/editor/", MaxAge: 15 * 60, HttpOnly: true, Secure: r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"), SameSite: http.SameSiteStrictMode})
	result["url"] = "/editor/"
	return nil
}

func (a *App) stopEditorSession() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := a.runAdvancedCompose(ctx, "--profile", "maintenance-editor", "stop", "save-editor"); err != nil {
		return fmt.Errorf("停止存档编辑器失败: %w", err)
	}
	a.audit("info", "server", "已停止维护编辑会话", "admin", nil)
	return nil
}

func (a *App) runAdvancedCompose(ctx context.Context, args ...string) ([]byte, error) {
	base := []string{"compose", "--env-file", a.cfg.EnvFile, "-p", "palworld-advanced", "-f", a.cfg.AdvancedCompose}
	return runCmd(ctx, a.cfg.ComposeDir, "docker", append(base, args...)...)
}

func (a *App) prepareEditorWorkspace() error {
	parent := filepath.Dir(a.cfg.EditorWorkspace)
	if err := os.MkdirAll(parent, 0o750); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, ".editor-workspace-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	prepared := filepath.Join(stage, "world")
	if err := copyDir(a.cfg.WorldSnapshot, prepared); err != nil {
		return err
	}
	if err := filepath.WalkDir(prepared, func(path string, _ os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		return os.Chown(path, 10002, 10002)
	}); err != nil {
		return err
	}
	previous := a.cfg.EditorWorkspace + ".previous"
	_ = os.RemoveAll(previous)
	if pathExists(a.cfg.EditorWorkspace) {
		if err := os.Rename(a.cfg.EditorWorkspace, previous); err != nil {
			return err
		}
	}
	if err := os.Rename(prepared, a.cfg.EditorWorkspace); err != nil {
		if pathExists(previous) {
			_ = os.Rename(previous, a.cfg.EditorWorkspace)
		}
		return err
	}
	_ = os.RemoveAll(previous)
	return nil
}

func (a *App) handleEditorProxy(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("palworld_editor_session")
	if err != nil {
		writeError(w, APIError{Status: http.StatusUnauthorized, Message: "请从存档编辑页面打开维护编辑器"})
		return
	}
	username, ok := a.verifyToken(cookie.Value)
	if !ok || username != "editor" {
		writeError(w, APIError{Status: http.StatusUnauthorized, Message: "维护会话已过期"})
		return
	}
	if !a.editorReachable() {
		writeError(w, APIError{Status: http.StatusServiceUnavailable, Message: "维护编辑器未运行"})
		return
	}
	target, err := url.Parse(a.cfg.EditorURL)
	if err != nil {
		writeError(w, err)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	director := proxy.Director
	proxy.Director = func(request *http.Request) {
		director(request)
		request.URL.Path = strings.TrimPrefix(request.URL.Path, "/editor")
		if request.URL.Path == "" {
			request.URL.Path = "/"
		}
		request.Host = target.Host
	}
	proxy.ModifyResponse = func(response *http.Response) error {
		if location := response.Header.Get("Location"); strings.HasPrefix(location, "/") && !strings.HasPrefix(location, "/editor/") {
			response.Header.Set("Location", "/editor"+location)
		}
		return nil
	}
	proxy.ErrorHandler = func(writer http.ResponseWriter, _ *http.Request, proxyErr error) {
		writeError(writer, APIError{Status: http.StatusBadGateway, Message: "维护编辑器代理失败: " + proxyErr.Error()})
	}
	proxy.ServeHTTP(w, r)
}

func (a *App) editorReachable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.cfg.EditorURL+"/", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusInternalServerError
}

func (a *App) worldSnapshotFile() string {
	return filepath.Join(a.cfg.StateDir, "world-index-snapshot.json")
}

func (a *App) worldSnapshotAvailable() bool {
	return pathExists(filepath.Join(a.cfg.WorldSnapshot, "Level.sav"))
}

func (a *App) snapshotID() string {
	snapshot, err := a.readWorldSnapshot()
	if err != nil {
		return ""
	}
	return snapshot.ID
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
