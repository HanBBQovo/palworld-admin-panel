package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (a *App) handleStatus(w http.ResponseWriter, r *http.Request) {
	settings := a.readSettings()
	ctx := r.Context()
	inspect := a.dockerInspect(ctx)
	stats := a.dockerStats(ctx)
	disk := diskUsage(ctx, a.cfg.DataDir)
	worldSize := pathSizeGB(ctx, a.cfg.SavesDir)
	players := a.refreshPlayers()
	infoResult, _ := a.executeRcon("Info", a.cfg.RconTimeout)
	version := a.detectVersion(infoResult.Output)
	startedAt := ""
	image := "thijsvanloef/palworld-server-docker:latest"
	running := false
	health := "warning"
	if inspect != nil {
		startedAt = inspect.State.StartedAt
		running = inspect.State.Running
		if inspect.Config.Image != "" {
			image = inspect.Config.Image
		}
		health = mapHealth(inspect)
	}
	if !running && inspect != nil {
		health = "offline"
	}
	memUsed, memLimit := parseMemoryUsage(stats.MemUsage)
	if memLimit == 0 {
		memLimit = round1(float64(totalMemoryBytes()) / 1024 / 1024 / 1024)
	}
	maintenance := a.maintenancePolicy()
	writeJSON(w, http.StatusOK, ServerStatus{
		Name: settings.ServerName, Host: a.displayHost(), Address: a.publicAddress(settings),
		Version: version.GameVersion, GameVersion: version.GameVersion, SteamBuildID: version.SteamBuildID,
		VersionSource: version.Source, Timezone: serverTimezone(), Container: a.cfg.Container, Image: image, Health: health,
		StartedAt: formatTimeString(startedAt), Uptime: formatUptime(startedAt),
		PlayersOnline: len(players), PlayersMax: settings.Players, CPU: parseHostCPU(stats.CPUPerc),
		MemoryUsedGB: memUsed, MemoryLimitGB: memLimit, DiskUsedGB: disk.UsedGB, DiskTotalGB: disk.TotalGB,
		WorldSizeGB: worldSize, LastSaveAt: lastModified(a.cfg.SavesDir),
		NextBackupAt:  enabledSchedule(maintenance.BackupEnabled, maintenance.BackupCron),
		NextRestartAt: enabledSchedule(maintenance.AutoReboot, maintenance.AutoRebootCron),
		Ports: []PortBinding{
			{Port: getenvIntAny(8211, "PALWORLD_PORT", "PORT"), Protocol: "UDP", Exposure: "public", Purpose: "游戏连接端口", Safe: true},
			{Port: getenvIntAny(27015, "PALWORLD_QUERY_PORT", "QUERY_PORT"), Protocol: "UDP", Exposure: "public", Purpose: "Steam 查询端口", Safe: true},
			{Port: a.cfg.RconPort, Protocol: "TCP", Exposure: "local", Purpose: "RCON 管理端口", Safe: true},
			{Port: getenvIntAny(8212, "PALWORLD_REST_PORT", "REST_API_PORT"), Protocol: "TCP", Exposure: "local", Purpose: "REST API 管理端口", Safe: true},
		},
		Maintenance: maintenance,
	})
}

func (a *App) handlePlayers(w http.ResponseWriter, r *http.Request) {
	players, err := a.loadPlayers()
	if err != nil {
		writeError(w, APIError{Status: http.StatusBadGateway, Message: "无法读取玩家列表: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, players)
}

func (a *App) refreshPlayers() []Player {
	players, err := a.loadPlayers()
	if err != nil {
		a.audit("warn", "rcon", "ShowPlayers 超时或失败，状态页玩家数已降级为 0: "+err.Error(), "system", nil)
		return []Player{}
	}
	return players
}

func (a *App) loadPlayers() ([]Player, error) {
	a.playersMu.Lock()
	defer a.playersMu.Unlock()
	if !a.lastPlayersAt.IsZero() && time.Since(a.lastPlayersAt) <= 2*time.Second {
		return clonePlayers(a.lastPlayers), nil
	}

	result, err := a.executeRcon("ShowPlayers", a.cfg.RconTimeout)
	if err != nil {
		return nil, err
	}
	players := parsePlayers(result.Output)
	a.lastPlayers = players
	a.lastPlayersAt = time.Now()
	return clonePlayers(players), nil
}

func (a *App) handleLogs(w http.ResponseWriter, r *http.Request) {
	rows := a.auditRows(40)
	ctx, cancel := context.WithTimeout(r.Context(), 2500*time.Millisecond)
	defer cancel()
	if output, err := runCmd(ctx, "", "docker", "logs", "--timestamps", "--tail", "80", a.cfg.Container); err == nil {
		for i, line := range strings.Split(string(output), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			timestamp, message := parseDockerLogLine(line)
			message = cleanLogMessage(message)
			lower := strings.ToLower(message)
			level := "info"
			if strings.Contains(lower, "error") || strings.Contains(lower, "failed") {
				level = "error"
			} else if strings.Contains(lower, "warn") {
				level = "warn"
			}
			source := "server"
			if strings.Contains(lower, "backup") {
				source = "backup"
			} else if strings.Contains(lower, "steamcmd") || strings.Contains(lower, "update") || strings.Contains(lower, "installed") {
				source = "update"
			}
			rows = append(rows, LogEntry{ID: fmt.Sprintf("docker-%d", i), Timestamp: timestamp, Level: level, Source: source, Message: message})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Timestamp > rows[j].Timestamp })
	if len(rows) > 100 {
		rows = rows[:100]
	}
	writeJSON(w, http.StatusOK, rows)
}

func (a *App) handleBackups(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.backupRows(r.Context(), 200))
}

func (a *App) handleBackupSubroute(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/palworld/backups/" || r.URL.Path == "/api/palworld/backups" {
		a.handleCreateBackup(w, r)
		return
	}
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/palworld/backups/")
	id, ok := strings.CutSuffix(trimmed, "/restore")
	if !ok || id == "" {
		writeError(w, APIError{Status: http.StatusNotFound, Message: "接口不存在"})
		return
	}
	a.handleRestoreBackupID(w, r, id)
}

func (a *App) handleCreateBackup(w http.ResponseWriter, r *http.Request) {
	result, err := a.createBackup("admin")
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleRestoreBackupID(w http.ResponseWriter, r *http.Request, id string) {
	result, err := a.restoreBackup(id, "admin")
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.readSettings())
}

func (a *App) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	var next ServerSettings
	if err := readJSON(r, &next); err != nil {
		writeError(w, APIError{Status: http.StatusBadRequest, Message: "参数不是合法 JSON"})
		return
	}
	if next.Players <= 0 {
		writeError(w, APIError{Status: http.StatusBadRequest, Message: "最大玩家数必须大于 0"})
		return
	}
	if err := a.saveSettings(next, "admin"); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, next)
}

func (a *App) handleSaveMaintenancePolicy(w http.ResponseWriter, r *http.Request) {
	var next MaintenancePolicy
	if err := readJSON(r, &next); err != nil {
		writeError(w, APIError{Status: http.StatusBadRequest, Message: "参数不是合法 JSON"})
		return
	}
	if strings.TrimSpace(next.AutoUpdateCron) == "" || strings.TrimSpace(next.AutoRebootCron) == "" || strings.TrimSpace(next.BackupCron) == "" {
		writeError(w, APIError{Status: http.StatusBadRequest, Message: "Cron 表达式不能为空"})
		return
	}
	if next.BackupRetention < 1 {
		writeError(w, APIError{Status: http.StatusBadRequest, Message: "备份保留数量必须大于 0"})
		return
	}
	if err := a.saveMaintenancePolicy(next, "admin"); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, next)
}

func (a *App) handleRcon(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Command string `json:"command"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, APIError{Status: http.StatusBadRequest, Message: "请求体不是合法 JSON"})
		return
	}
	result, err := a.executeRcon(body.Command, a.cfg.RconTimeout)
	if err != nil {
		writeError(w, err)
		return
	}
	risk := "info"
	command := strings.ToLower(strings.TrimSpace(result.Command))
	if strings.HasPrefix(command, "shutdown") || strings.HasPrefix(command, "banplayer") {
		risk = "warn"
	}
	a.audit(risk, "rcon", "RCON executed: "+result.Command, "admin", nil)
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleMaintenance(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Action string `json:"action"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, APIError{Status: http.StatusBadRequest, Message: "请求体不是合法 JSON"})
		return
	}
	result, err := a.maintenance(strings.TrimSpace(body.Action), "admin")
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) readSettings() ServerSettings {
	settings := envSettings()
	raw, err := os.ReadFile(a.cfg.SettingsFile)
	if err == nil && len(bytes.TrimSpace(raw)) > 0 {
		_ = json.Unmarshal(raw, &settings)
	}
	return settings
}

func envSettings() ServerSettings {
	return ServerSettings{
		ServerName:            getenvAny("Palworld Dedicated Server", "PALWORLD_SERVER_NAME", "SERVER_NAME"),
		Description:           getenvAny("Managed by Palworld Ops", "PALWORLD_SERVER_DESCRIPTION", "SERVER_DESCRIPTION"),
		Players:               getenvIntAny(32, "PALWORLD_PLAYERS", "SERVER_PLAYER_MAX_NUM", "PLAYERS"),
		ServerPassword:        getenvAny("", "PALWORLD_SERVER_PASSWORD", "SERVER_PASSWORD"),
		AdminPassword:         getenvAny("", "PALWORLD_ADMIN_PASSWORD", "ADMIN_PASSWORD"),
		Community:             parseBool(getenvAny("", "PALWORLD_COMMUNITY", "COMMUNITY"), false),
		RestAPIEnabled:        parseBool(getenvAny("", "PALWORLD_REST_API_ENABLED", "REST_API_ENABLED"), false),
		RconEnabled:           parseBool(getenvAny("", "PALWORLD_RCON_ENABLED", "RCON_ENABLED"), true),
		PublicDomain:          getenvAny("", "PALWORLD_PUBLIC_DOMAIN", "PUBLIC_DOMAIN"),
		PublicIP:              getenvAny("", "PALWORLD_PUBLIC_IP", "PUBLIC_IP"),
		PublicPort:            getenvAny("8211", "PALWORLD_PUBLIC_PORT", "PUBLIC_PORT", "PALWORLD_PORT", "PORT"),
		ExpRate:               getenvFloatAny(1, "PALWORLD_EXP_RATE", "EXP_RATE"),
		CaptureRate:           getenvFloatAny(1, "PALWORLD_CAPTURE_RATE", "PAL_CAPTURE_RATE", "CAPTURE_RATE"),
		SpawnRate:             getenvFloatAny(1, "PALWORLD_SPAWN_RATE", "PAL_SPAWN_NUM_RATE", "SPAWN_RATE"),
		CollectionDropRate:    getenvFloatAny(1, "PALWORLD_COLLECTION_DROP_RATE", "COLLECTION_DROP_RATE"),
		EnemyDropRate:         getenvFloatAny(1, "PALWORLD_ENEMY_DROP_RATE", "ENEMY_DROP_ITEM_RATE", "ENEMY_DROP_RATE"),
		EggHatchingHours:      getenvFloatAny(72, "PALWORLD_EGG_HATCHING_HOURS", "PAL_EGG_DEFAULT_HATCHING_TIME", "EGG_HATCHING_HOURS"),
		AutoSaveSpan:          getenvIntAny(30, "PALWORLD_AUTO_SAVE_SPAN", "AUTO_SAVE_SPAN"),
		DeathPenalty:          getenvAny("All", "PALWORLD_DEATH_PENALTY", "DEATH_PENALTY"),
		BaseCampWorkerMax:     getenvIntAny(15, "PALWORLD_BASE_CAMP_WORKER_MAX", "BASE_CAMP_WORKER_MAX_NUM", "BASE_CAMP_WORKER_MAX"),
		GuildPlayerMax:        getenvIntAny(20, "PALWORLD_GUILD_PLAYER_MAX", "GUILD_PLAYER_MAX_NUM", "GUILD_PLAYER_MAX"),
		BaseCampMaxInGuild:    getenvIntAny(4, "PALWORLD_BASE_CAMP_MAX_IN_GUILD", "BASE_CAMP_MAX_NUM_IN_GUILD", "BASE_CAMP_MAX_IN_GUILD"),
		CrossplayPlatforms:    splitList(getenvAny("Steam,Xbox,PS5,Mac", "PALWORLD_CROSSPLAY_PLATFORMS", "CROSSPLAY_PLATFORMS")),
		AutoPauseEnabled:      parseBool(getenvAny("", "PALWORLD_AUTO_PAUSE_ENABLED", "AUTO_PAUSE_ENABLED"), false),
		PlayerLoggingEnabled:  parseBool(getenvAny("", "PALWORLD_PLAYER_LOGGING_ENABLED", "ENABLE_PLAYER_LOGGING"), true),
		DiscordWebhookEnabled: parseBool(getenvAny("", "PALWORLD_DISCORD_WEBHOOK_ENABLED", "DISCORD_WEBHOOK_ENABLED"), false),
		TargetManifestID:      getenvAny("", "PALWORLD_TARGET_MANIFEST_ID", "TARGET_MANIFEST_ID"),
	}
}

func (a *App) saveSettings(settings ServerSettings, actor string) error {
	if err := os.MkdirAll(filepath.Dir(a.cfg.SettingsFile), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(a.cfg.SettingsFile, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	_ = os.Chmod(a.cfg.SettingsFile, 0o600)
	if a.cfg.WriteEnv {
		if err := updateEnvValues(a.cfg.EnvFile, settingsToEnv(settings)); err != nil {
			return err
		}
	}
	a.audit("info", "server", "已保存服务器参数到配置文件", actor, nil)
	return nil
}

func settingsToEnv(settings ServerSettings) map[string]string {
	players := strconv.Itoa(settings.Players)
	expRate := trimFloat(settings.ExpRate)
	captureRate := trimFloat(settings.CaptureRate)
	spawnRate := trimFloat(settings.SpawnRate)
	collectionDropRate := trimFloat(settings.CollectionDropRate)
	enemyDropRate := trimFloat(settings.EnemyDropRate)
	eggHatchingHours := trimFloat(settings.EggHatchingHours)
	autoSaveSpan := strconv.Itoa(settings.AutoSaveSpan)
	baseCampWorkerMax := strconv.Itoa(settings.BaseCampWorkerMax)
	guildPlayerMax := strconv.Itoa(settings.GuildPlayerMax)
	baseCampMaxInGuild := strconv.Itoa(settings.BaseCampMaxInGuild)
	crossplayPlatforms := formatCrossplayPlatforms(settings.CrossplayPlatforms)

	return map[string]string{
		"SERVER_NAME": settings.ServerName, "SERVER_DESCRIPTION": settings.Description, "PLAYERS": players, "SERVER_PLAYER_MAX_NUM": players,
		"SERVER_PASSWORD": settings.ServerPassword, "ADMIN_PASSWORD": settings.AdminPassword, "COMMUNITY": formatBool(settings.Community),
		"RCON_ENABLED": formatBool(settings.RconEnabled), "REST_API_ENABLED": formatBool(settings.RestAPIEnabled),
		"EXP_RATE": expRate, "PAL_CAPTURE_RATE": captureRate, "PAL_SPAWN_NUM_RATE": spawnRate,
		"COLLECTION_DROP_RATE": collectionDropRate, "ENEMY_DROP_ITEM_RATE": enemyDropRate,
		"PAL_EGG_DEFAULT_HATCHING_TIME": eggHatchingHours, "AUTO_SAVE_SPAN": autoSaveSpan,
		"DEATH_PENALTY": settings.DeathPenalty, "BASE_CAMP_WORKER_MAX_NUM": baseCampWorkerMax,
		"GUILD_PLAYER_MAX_NUM": guildPlayerMax, "BASE_CAMP_MAX_NUM_IN_GUILD": baseCampMaxInGuild,
		"CROSSPLAY_PLATFORMS": crossplayPlatforms, "AUTO_PAUSE_ENABLED": formatBool(settings.AutoPauseEnabled),
		"ENABLE_PLAYER_LOGGING": formatBool(settings.PlayerLoggingEnabled), "DISCORD_WEBHOOK_ENABLED": formatBool(settings.DiscordWebhookEnabled),
		"TARGET_MANIFEST_ID":   settings.TargetManifestID,
		"PALWORLD_SERVER_NAME": settings.ServerName, "PALWORLD_SERVER_DESCRIPTION": settings.Description, "PALWORLD_PLAYERS": players,
		"PALWORLD_SERVER_PASSWORD": settings.ServerPassword, "PALWORLD_ADMIN_PASSWORD": settings.AdminPassword, "PALWORLD_COMMUNITY": formatBool(settings.Community),
		"PALWORLD_RCON_ENABLED": formatBool(settings.RconEnabled), "PALWORLD_REST_API_ENABLED": formatBool(settings.RestAPIEnabled),
		"PALWORLD_PUBLIC_DOMAIN": settings.PublicDomain, "PALWORLD_PUBLIC_IP": settings.PublicIP, "PALWORLD_PUBLIC_PORT": settings.PublicPort, "PALWORLD_EXP_RATE": expRate,
		"PALWORLD_CAPTURE_RATE": captureRate, "PALWORLD_SPAWN_RATE": spawnRate,
		"PALWORLD_COLLECTION_DROP_RATE": collectionDropRate, "PALWORLD_ENEMY_DROP_RATE": enemyDropRate,
		"PALWORLD_EGG_HATCHING_HOURS": eggHatchingHours, "PALWORLD_AUTO_SAVE_SPAN": autoSaveSpan,
		"PALWORLD_DEATH_PENALTY": settings.DeathPenalty, "PALWORLD_BASE_CAMP_WORKER_MAX": baseCampWorkerMax,
		"PALWORLD_GUILD_PLAYER_MAX": guildPlayerMax, "PALWORLD_BASE_CAMP_MAX_IN_GUILD": baseCampMaxInGuild,
		"PALWORLD_CROSSPLAY_PLATFORMS": crossplayPlatforms, "PALWORLD_AUTO_PAUSE_ENABLED": formatBool(settings.AutoPauseEnabled),
		"PALWORLD_PLAYER_LOGGING_ENABLED": formatBool(settings.PlayerLoggingEnabled), "PALWORLD_DISCORD_WEBHOOK_ENABLED": formatBool(settings.DiscordWebhookEnabled),
		"PALWORLD_TARGET_MANIFEST_ID": settings.TargetManifestID,
	}
}

func formatCrossplayPlatforms(platforms []string) string {
	values := make([]string, 0, len(platforms))
	for _, platform := range platforms {
		platform = strings.TrimSpace(platform)
		if platform != "" {
			values = append(values, platform)
		}
	}
	if len(values) == 0 {
		values = []string{"Steam"}
	}
	return "(" + strings.Join(values, ",") + ")"
}

func (a *App) saveMaintenancePolicy(policy MaintenancePolicy, actor string) error {
	if !a.cfg.WriteEnv {
		a.audit("warn", "server", "PANEL_WRITE_ENV=false，维护策略未写入 .env", actor, nil)
		return nil
	}
	updates := map[string]string{
		"UPDATE_ON_BOOT": formatBool(policy.UpdateOnBoot), "PALWORLD_UPDATE_ON_BOOT": formatBool(policy.UpdateOnBoot),
		"AUTO_UPDATE_ENABLED": formatBool(policy.AutoUpdate), "PALWORLD_AUTO_UPDATE_ENABLED": formatBool(policy.AutoUpdate),
		"AUTO_UPDATE_CRON_EXPRESSION": policy.AutoUpdateCron, "PALWORLD_AUTO_UPDATE_CRON": policy.AutoUpdateCron,
		"AUTO_REBOOT_ENABLED": formatBool(policy.AutoReboot), "PALWORLD_AUTO_REBOOT_ENABLED": formatBool(policy.AutoReboot),
		"AUTO_REBOOT_CRON_EXPRESSION": policy.AutoRebootCron, "PALWORLD_AUTO_REBOOT_CRON": policy.AutoRebootCron,
		"BACKUP_ENABLED": formatBool(policy.BackupEnabled), "PALWORLD_BACKUP_ENABLED": formatBool(policy.BackupEnabled),
		"BACKUP_CRON_EXPRESSION": policy.BackupCron, "PALWORLD_BACKUP_CRON": policy.BackupCron,
		"BACKUP_RETENTION_AMOUNT_TO_KEEP": strconv.Itoa(policy.BackupRetention), "PALWORLD_BACKUP_RETENTION": strconv.Itoa(policy.BackupRetention),
	}
	if err := updateEnvValues(a.cfg.EnvFile, updates); err != nil {
		return err
	}
	a.audit("info", "server", "已保存自动维护策略到 .env，重启游戏容器后完全生效", actor, nil)
	return nil
}

func updateEnvValues(envFile string, updates map[string]string) error {
	text, _ := os.ReadFile(envFile)
	lines := strings.Split(string(text), "\n")
	seen := map[string]bool{}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || !strings.Contains(trimmed, "=") {
			continue
		}
		key := strings.TrimSpace(strings.SplitN(trimmed, "=", 2)[0])
		if value, ok := updates[key]; ok {
			lines[i] = key + "=" + formatEnvValue(value)
			seen[key] = true
		}
	}
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}
	keys := make([]string, 0, len(updates))
	for key := range updates {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !seen[key] {
			lines = append(lines, key+"="+formatEnvValue(updates[key]))
		}
	}
	if err := os.MkdirAll(filepath.Dir(envFile), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(envFile, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		return err
	}
	return os.Chmod(envFile, 0o600)
}

func (a *App) publicAddress(settings ServerSettings) string {
	port := settings.PublicPort
	if port == "" {
		port = "8211"
	}
	if settings.PublicDomain != "" {
		return settings.PublicDomain + ":" + port
	}
	if a.cfg.PublicDomain != "" {
		return a.cfg.PublicDomain + ":" + port
	}
	if settings.PublicIP != "" {
		return settings.PublicIP + ":" + port
	}
	return "未配置连接域名"
}

func (a *App) displayHost() string {
	if value := strings.TrimSpace(a.cfg.DisplayHost); value != "" {
		return value
	}
	if name, err := os.Hostname(); err == nil && strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return "unknown"
}

func (a *App) maintenancePolicy() MaintenancePolicy {
	fileEnv := readEnvFile(a.cfg.EnvFile)
	return MaintenancePolicy{
		UpdateOnBoot:    parseBool(envValueAny(fileEnv, "", "PALWORLD_UPDATE_ON_BOOT", "UPDATE_ON_BOOT"), true),
		AutoUpdate:      parseBool(envValueAny(fileEnv, "", "PALWORLD_AUTO_UPDATE_ENABLED", "AUTO_UPDATE_ENABLED"), true),
		AutoUpdateCron:  envValueAny(fileEnv, "0 4 * * *", "PALWORLD_AUTO_UPDATE_CRON", "AUTO_UPDATE_CRON_EXPRESSION"),
		AutoReboot:      parseBool(envValueAny(fileEnv, "", "PALWORLD_AUTO_REBOOT_ENABLED", "AUTO_REBOOT_ENABLED"), true),
		AutoRebootCron:  envValueAny(fileEnv, "0 5 * * *", "PALWORLD_AUTO_REBOOT_CRON", "AUTO_REBOOT_CRON_EXPRESSION"),
		BackupEnabled:   parseBool(envValueAny(fileEnv, "", "PALWORLD_BACKUP_ENABLED", "BACKUP_ENABLED"), true),
		BackupCron:      envValueAny(fileEnv, "0 * * * *", "PALWORLD_BACKUP_CRON", "BACKUP_CRON_EXPRESSION"),
		BackupRetention: envIntAny(fileEnv, 72, "PALWORLD_BACKUP_RETENTION", "BACKUP_RETENTION_AMOUNT_TO_KEEP"),
	}
}

func enabledSchedule(enabled bool, schedule string) string {
	if !enabled {
		return "关闭"
	}
	if strings.TrimSpace(schedule) == "" {
		return "按容器配置"
	}
	return schedule
}

type dockerInspectResult struct {
	State struct {
		Running   bool   `json:"Running"`
		StartedAt string `json:"StartedAt"`
		Health    *struct {
			Status string `json:"Status"`
		} `json:"Health"`
	} `json:"State"`
	Config struct {
		Image  string            `json:"Image"`
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
}

type dockerStatsResult struct {
	CPUPerc  string `json:"CPUPerc"`
	MemUsage string `json:"MemUsage"`
}

func (a *App) dockerInspect(ctx context.Context) *dockerInspectResult {
	cctx, cancel := context.WithTimeout(ctx, 2200*time.Millisecond)
	defer cancel()
	output, err := runCmd(cctx, "", "docker", "inspect", a.cfg.Container)
	if err != nil {
		return nil
	}
	var rows []dockerInspectResult
	if err := json.Unmarshal(output, &rows); err != nil || len(rows) == 0 {
		return nil
	}
	return &rows[0]
}

func (a *App) dockerStats(ctx context.Context) dockerStatsResult {
	cctx, cancel := context.WithTimeout(ctx, 2200*time.Millisecond)
	defer cancel()
	output, err := runCmd(cctx, "", "docker", "stats", "--no-stream", "--format", "{{json .}}", a.cfg.Container)
	if err != nil {
		return dockerStatsResult{}
	}
	var row dockerStatsResult
	_ = json.Unmarshal(bytes.TrimSpace(output), &row)
	return row
}

type versionInfo struct {
	GameVersion  string
	SteamBuildID string
	Source       string
}

var gameVersionPattern = regexp.MustCompile(`(?i)\bv\d+(?:\.\d+){2,}[a-z0-9._+-]*`)

func (a *App) detectVersion(infoOutput string) versionInfo {
	buildID := steamBuildID(filepath.Join(a.cfg.DataDir, "steamapps", "appmanifest_2394010.acf"))
	if value := strings.TrimSpace(getenvAny("", "PALWORLD_GAME_VERSION", "PALWORLD_VERSION")); value != "" {
		return versionInfo{GameVersion: value, SteamBuildID: buildID, Source: "environment"}
	}
	if value := parseGameVersion(infoOutput); value != "" {
		return versionInfo{GameVersion: value, SteamBuildID: buildID, Source: "rcon"}
	}
	return versionInfo{GameVersion: "unknown", SteamBuildID: buildID, Source: "unavailable"}
}

func parseGameVersion(output string) string {
	return gameVersionPattern.FindString(strings.TrimSpace(output))
}

func mapHealth(inspect *dockerInspectResult) string {
	if inspect == nil {
		return "warning"
	}
	if !inspect.State.Running {
		return "offline"
	}
	if inspect.State.Health == nil {
		return "healthy"
	}
	switch strings.ToLower(inspect.State.Health.Status) {
	case "healthy":
		return "healthy"
	case "starting":
		return "starting"
	default:
		return "warning"
	}
}

func (a *App) cachedPlayerCount() int {
	a.playersMu.RLock()
	defer a.playersMu.RUnlock()
	if time.Since(a.lastPlayersAt) <= 90*time.Second {
		return len(a.lastPlayers)
	}
	return 0
}
