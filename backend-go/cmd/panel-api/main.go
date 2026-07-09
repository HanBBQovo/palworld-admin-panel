package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Bind         string
	Port         int
	AuthPassword string
	TokenSecret  string
	TokenTTL     time.Duration
	CorsOrigin   string
	WebRoot      string
	StateDir     string
	SettingsFile string
	AuditFile    string
	OpsFile      string
	DataDir      string
	SavesDir     string
	BackupsDir   string
	ComposeDir   string
	EnvFile      string
	Container    string
	RconHost     string
	RconPort     int
	RconPassword string
	RconTimeout  time.Duration
	AllowRawRcon bool
	WriteEnv     bool
	DisplayHost  string
	PublicDomain string
}

type App struct {
	cfg           Config
	playersMu     sync.RWMutex
	lastPlayers   []Player
	lastPlayersAt time.Time
	auditMu       sync.Mutex
	opsMu         sync.Mutex
}

type APIError struct {
	Status  int
	Message string
}

func (e APIError) Error() string { return e.Message }

type ServerSettings struct {
	ServerName            string   `json:"serverName"`
	Description           string   `json:"description"`
	Players               int      `json:"players"`
	ServerPassword        string   `json:"serverPassword"`
	AdminPassword         string   `json:"adminPassword"`
	Community             bool     `json:"community"`
	RestAPIEnabled        bool     `json:"restApiEnabled"`
	RconEnabled           bool     `json:"rconEnabled"`
	PublicDomain          string   `json:"publicDomain"`
	PublicIP              string   `json:"publicIp"`
	PublicPort            string   `json:"publicPort"`
	ExpRate               float64  `json:"expRate"`
	CaptureRate           float64  `json:"captureRate"`
	SpawnRate             float64  `json:"spawnRate"`
	CollectionDropRate    float64  `json:"collectionDropRate"`
	EnemyDropRate         float64  `json:"enemyDropRate"`
	EggHatchingHours      float64  `json:"eggHatchingHours"`
	AutoSaveSpan          int      `json:"autoSaveSpan"`
	DeathPenalty          string   `json:"deathPenalty"`
	BaseCampWorkerMax     int      `json:"baseCampWorkerMax"`
	GuildPlayerMax        int      `json:"guildPlayerMax"`
	BaseCampMaxInGuild    int      `json:"baseCampMaxInGuild"`
	CrossplayPlatforms    []string `json:"crossplayPlatforms"`
	AutoPauseEnabled      bool     `json:"autoPauseEnabled"`
	PlayerLoggingEnabled  bool     `json:"playerLoggingEnabled"`
	DiscordWebhookEnabled bool     `json:"discordWebhookEnabled"`
	TargetManifestID      string   `json:"targetManifestId"`
}

type MaintenancePolicy struct {
	UpdateOnBoot    bool   `json:"updateOnBoot"`
	AutoUpdate      bool   `json:"autoUpdate"`
	AutoUpdateCron  string `json:"autoUpdateCron"`
	AutoReboot      bool   `json:"autoReboot"`
	AutoRebootCron  string `json:"autoRebootCron"`
	BackupEnabled   bool   `json:"backupEnabled"`
	BackupCron      string `json:"backupCron"`
	BackupRetention int    `json:"backupRetention"`
}

type PortBinding struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Exposure string `json:"exposure"`
	Purpose  string `json:"purpose"`
	Safe     bool   `json:"safe"`
}

type ServerStatus struct {
	Name          string            `json:"name"`
	Host          string            `json:"host"`
	Address       string            `json:"address"`
	Version       string            `json:"version"`
	Timezone      string            `json:"timezone"`
	Container     string            `json:"container"`
	Image         string            `json:"image"`
	Health        string            `json:"health"`
	StartedAt     string            `json:"startedAt"`
	Uptime        string            `json:"uptime"`
	PlayersOnline int               `json:"playersOnline"`
	PlayersMax    int               `json:"playersMax"`
	CPU           float64           `json:"cpu"`
	MemoryUsedGB  float64           `json:"memoryUsedGb"`
	MemoryLimitGB float64           `json:"memoryLimitGb"`
	DiskUsedGB    float64           `json:"diskUsedGb"`
	DiskTotalGB   float64           `json:"diskTotalGb"`
	WorldSizeGB   float64           `json:"worldSizeGb"`
	LastSaveAt    string            `json:"lastSaveAt"`
	NextBackupAt  string            `json:"nextBackupAt"`
	NextRestartAt string            `json:"nextRestartAt"`
	Ports         []PortBinding     `json:"ports"`
	Maintenance   MaintenancePolicy `json:"maintenance"`
}

type Player struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Platform  string `json:"platform"`
	SteamID   string `json:"steamId"`
	Level     int    `json:"level"`
	Guild     string `json:"guild"`
	Location  string `json:"location"`
	OnlineFor string `json:"onlineFor"`
	Ping      int    `json:"ping"`
	Status    string `json:"status"`
}

type LogEntry struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Source    string `json:"source"`
	Message   string `json:"message"`
}

type Backup struct {
	ID        string `json:"id"`
	CreatedAt string `json:"createdAt"`
	Size      string `json:"size"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	Note      string `json:"note"`
}

type RconCommandDefinition struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Command     string `json:"command"`
	Description string `json:"description"`
	Risk        string `json:"risk"`
	Category    string `json:"category"`
}

type RconCommandResult struct {
	Command    string `json:"command"`
	Output     string `json:"output"`
	ExecutedAt string `json:"executedAt"`
}

type OperationRecord struct {
	ID          string         `json:"id"`
	Action      string         `json:"action"`
	Status      string         `json:"status"`
	Message     string         `json:"message,omitempty"`
	Actor       string         `json:"actor,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   string         `json:"createdAt"`
	CompletedAt string         `json:"completedAt,omitempty"`
}

var commandDefinitions = []RconCommandDefinition{
	{ID: "info", Label: "查看服务器信息", Command: "Info", Description: "显示服务器基础信息，用来确认 RCON 已连通。", Risk: "low", Category: "info"},
	{ID: "players", Label: "查看在线玩家", Command: "ShowPlayers", Description: "列出当前在线玩家、玩家 ID 和 SteamID。", Risk: "low", Category: "player"},
	{ID: "save", Label: "立即保存世界", Command: "Save", Description: "手动保存当前世界状态，备份或维护前建议先执行。", Risk: "low", Category: "world"},
	{ID: "broadcast", Label: "广播消息", Command: "Broadcast 服务器将在5分钟后维护", Description: "向所有在线玩家发送一条公告。", Risk: "low", Category: "broadcast"},
	{ID: "kick", Label: "踢出玩家", Command: "KickPlayer <SteamID>", Description: "把指定玩家踢下线，需要把 <SteamID> 替换成真实值。", Risk: "medium", Category: "player"},
	{ID: "ban", Label: "封禁玩家", Command: "BanPlayer <SteamID>", Description: "封禁指定玩家，需要谨慎执行并记录原因。", Risk: "high", Category: "player"},
	{ID: "shutdown", Label: "延迟关服", Command: "Shutdown 300 服务器将在5分钟后关闭", Description: "倒计时关服并给玩家提示，适合维护前使用。", Risk: "high", Category: "shutdown"},
}

var allowedRconPrefixes = []string{"Info", "ShowPlayers", "Save", "Broadcast", "KickPlayer", "BanPlayer", "Shutdown"}

func main() {
	loadDotEnv(".env")
	cfg := loadConfig()
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		log.Fatalf("create state dir: %v", err)
	}

	app := &App{cfg: cfg}
	addr := fmt.Sprintf("%s:%d", cfg.Bind, cfg.Port)
	log.Printf("palworld panel api listening on %s", addr)
	if err := http.ListenAndServe(addr, app.routes()); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func loadConfig() Config {
	stateDir := getenv("PANEL_STATE_DIR", ".panel-state")
	timeoutMs := getenvInt("PANEL_RCON_TIMEOUT_MS", 1800)
	return Config{
		Bind:         getenvAny("0.0.0.0", "PANEL_API_BIND", "HOST"),
		Port:         getenvIntAny(16824, "PANEL_API_PORT", "APP_API_PORT", "PORT"),
		AuthPassword: getenv("PANEL_AUTH_PASSWORD", "change-panel-password"),
		TokenSecret:  getenvAny("", "PANEL_TOKEN_SECRET", "PANEL_JWT_SECRET", "PANEL_AUTH_PASSWORD"),
		TokenTTL:     time.Duration(getenvInt("PANEL_TOKEN_TTL_SECONDS", 60*60*24*7)) * time.Second,
		CorsOrigin:   getenv("PANEL_CORS_ORIGIN", "*"),
		WebRoot:      getenv("PANEL_WEB_ROOT", "dist"),
		StateDir:     stateDir,
		SettingsFile: getenv("PANEL_SETTINGS_FILE", filepath.Join(stateDir, "settings.json")),
		AuditFile:    getenv("PANEL_AUDIT_FILE", filepath.Join(stateDir, "audit.jsonl")),
		OpsFile:      getenv("PANEL_OPS_FILE", filepath.Join(stateDir, "operations.jsonl")),
		DataDir:      getenv("PALWORLD_DATA_DIR", "/palworld"),
		SavesDir:     getenvAny("/palworld/Pal/Saved/SaveGames", "PALWORLD_SAVES_DIR", "PALWORLD_SAVE_DIR"),
		BackupsDir:   getenv("PALWORLD_BACKUP_DIR", "/palworld/backups"),
		ComposeDir:   getenv("PALWORLD_COMPOSE_DIR", "."),
		EnvFile:      getenv("PANEL_ENV_FILE", filepath.Join(getenv("PALWORLD_COMPOSE_DIR", "."), ".env")),
		Container:    getenv("PALWORLD_CONTAINER", "palworld-server"),
		RconHost:     getenv("PALWORLD_RCON_HOST", "127.0.0.1"),
		RconPort:     getenvIntAny(25575, "PALWORLD_RCON_PORT", "RCON_PORT"),
		RconPassword: getenvAny("", "PALWORLD_ADMIN_PASSWORD", "ADMIN_PASSWORD"),
		RconTimeout:  time.Duration(timeoutMs) * time.Millisecond,
		AllowRawRcon: parseBool(getenv("PANEL_ALLOW_RAW_RCON", ""), false),
		WriteEnv:     parseBool(getenv("PANEL_WRITE_ENV", ""), true),
		DisplayHost:  getenv("PANEL_DISPLAY_HOST", ""),
		PublicDomain: getenvAny("", "PALWORLD_PUBLIC_DOMAIN", "PUBLIC_DOMAIN"),
	}
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", a.method("GET", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}))
	mux.HandleFunc("/api/auth/login", a.method("POST", a.handleLogin))
	mux.HandleFunc("/api/auth/logout", a.method("POST", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}))
	mux.HandleFunc("/api/auth/status", a.method("GET", a.handleAuthStatus))

	mux.HandleFunc("/api/palworld/status", a.authed("GET", a.handleStatus))
	mux.HandleFunc("/api/palworld/players", a.authed("GET", a.handlePlayers))
	mux.HandleFunc("/api/palworld/logs", a.authed("GET", a.handleLogs))
	mux.HandleFunc("/api/palworld/backups", a.authedAny(map[string]http.HandlerFunc{"GET": a.handleBackups, "POST": a.handleCreateBackup}))
	mux.HandleFunc("/api/palworld/backups/", a.authed("POST", a.handleBackupSubroute))
	mux.HandleFunc("/api/palworld/settings", a.authedAny(map[string]http.HandlerFunc{"GET": a.handleGetSettings, "PUT": a.handleSaveSettings}))
	mux.HandleFunc("/api/palworld/maintenance-policy", a.authed("PUT", a.handleSaveMaintenancePolicy))
	mux.HandleFunc("/api/palworld/rcon-commands", a.authed("GET", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, commandDefinitions)
	}))
	mux.HandleFunc("/api/palworld/rcon", a.authed("POST", a.handleRcon))
	mux.HandleFunc("/api/palworld/maintenance", a.authed("POST", a.handleMaintenance))
	mux.HandleFunc("/", a.serveSPA)
	return withCORS(a.cfg.CorsOrigin, withRecover(mux))
}

func (a *App) method(method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			writeError(w, APIError{Status: http.StatusMethodNotAllowed, Message: "Method not allowed"})
			return
		}
		next(w, r)
	}
}

func (a *App) authed(method string, next http.HandlerFunc) http.HandlerFunc {
	return a.method(method, func(w http.ResponseWriter, r *http.Request) {
		if _, ok := a.verifyRequestToken(r); !ok {
			writeError(w, APIError{Status: http.StatusUnauthorized, Message: "未登录或登录已过期"})
			return
		}
		next(w, r)
	})
}

func (a *App) authedAny(routes map[string]http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		next, ok := routes[r.Method]
		if !ok {
			writeError(w, APIError{Status: http.StatusMethodNotAllowed, Message: "Method not allowed"})
			return
		}
		if _, ok := a.verifyRequestToken(r); !ok {
			writeError(w, APIError{Status: http.StatusUnauthorized, Message: "未登录或登录已过期"})
			return
		}
		next(w, r)
	}
}

func withCORS(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept,Authorization,Content-Type,X-Request-Id")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withRecover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				writeError(w, APIError{Status: http.StatusInternalServerError, Message: fmt.Sprint(err)})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, APIError{Status: http.StatusBadRequest, Message: "请求体不是合法 JSON"})
		return
	}
	if subtle.ConstantTimeCompare([]byte(body.Password), []byte(a.cfg.AuthPassword)) != 1 {
		writeError(w, APIError{Status: http.StatusUnauthorized, Message: "面板密码错误"})
		return
	}
	token, err := a.signToken("admin")
	if err != nil {
		writeError(w, err)
		return
	}
	a.audit("info", "server", "面板登录成功", "admin", nil)
	writeJSON(w, http.StatusOK, map[string]any{"token": token})
}

func (a *App) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	_, ok := a.verifyRequestToken(r)
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": ok})
}

func (a *App) signToken(username string) (string, error) {
	payload := map[string]any{"u": username, "exp": time.Now().Add(a.cfg.TokenTTL).Unix()}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, []byte(a.cfg.TokenSecret))
	_, _ = mac.Write([]byte(body))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return body + "." + sig, nil
}

func (a *App) verifyRequestToken(r *http.Request) (string, bool) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return "", false
	}
	parts := strings.Split(strings.TrimSpace(header[7:]), ".")
	if len(parts) != 2 {
		return "", false
	}
	mac := hmac.New(sha256.New, []byte(a.cfg.TokenSecret))
	_, _ = mac.Write([]byte(parts[0]))
	want := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(got, want) {
		return "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", false
	}
	var payload struct {
		Username string `json:"u"`
		Expires  int64  `json:"exp"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil || payload.Expires <= time.Now().Unix() {
		return "", false
	}
	return payload.Username, true
}

func (a *App) handleStatus(w http.ResponseWriter, r *http.Request) {
	settings := a.readSettings()
	ctx := r.Context()
	inspect := a.dockerInspect(ctx)
	stats := a.dockerStats(ctx)
	disk := diskUsage(ctx, a.cfg.DataDir)
	worldSize := pathSizeGB(ctx, a.cfg.SavesDir)
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
		Version: a.detectVersion(inspect), Timezone: serverTimezone(), Container: a.cfg.Container, Image: image, Health: health,
		StartedAt: formatTimeString(startedAt), Uptime: formatUptime(startedAt),
		PlayersOnline: a.cachedPlayerCount(), PlayersMax: settings.Players, CPU: parseHostCPU(stats.CPUPerc),
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
	result, err := a.executeRcon("ShowPlayers", a.cfg.RconTimeout)
	if err != nil {
		a.audit("warn", "rcon", "ShowPlayers 超时或失败，玩家列表已降级为空列表: "+err.Error(), "admin", nil)
		writeJSON(w, http.StatusOK, []Player{})
		return
	}
	players := parsePlayers(result.Output)
	a.playersMu.Lock()
	a.lastPlayers = players
	a.lastPlayersAt = time.Now()
	a.playersMu.Unlock()
	writeJSON(w, http.StatusOK, players)
}

func (a *App) handleLogs(w http.ResponseWriter, r *http.Request) {
	rows := a.auditRows(40)
	ctx, cancel := context.WithTimeout(r.Context(), 2500*time.Millisecond)
	defer cancel()
	if output, err := runCmd(ctx, "", "docker", "logs", "--tail", "80", a.cfg.Container); err == nil {
		now := formatTime(time.Now())
		for i, line := range strings.Split(string(output), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			lower := strings.ToLower(line)
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
			rows = append(rows, LogEntry{ID: fmt.Sprintf("docker-%d", i), Timestamp: now, Level: level, Source: source, Message: line})
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
		writeError(w, APIError{Status: http.StatusBadGateway, Message: err.Error()})
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
		var saved ServerSettings
		if err := json.Unmarshal(raw, &saved); err == nil {
			settings = saved
		}
	}
	return settings
}

func envSettings() ServerSettings {
	return ServerSettings{
		ServerName:            getenvAny("Palworld Dedicated Server", "PALWORLD_SERVER_NAME", "SERVER_NAME"),
		Description:           getenvAny("Managed by Palworld Ops", "PALWORLD_SERVER_DESCRIPTION", "SERVER_DESCRIPTION"),
		Players:               getenvIntAny(32, "PALWORLD_PLAYERS", "PLAYERS"),
		ServerPassword:        getenvAny("", "PALWORLD_SERVER_PASSWORD", "SERVER_PASSWORD"),
		AdminPassword:         getenvAny("", "PALWORLD_ADMIN_PASSWORD", "ADMIN_PASSWORD"),
		Community:             parseBool(getenvAny("", "PALWORLD_COMMUNITY", "COMMUNITY"), false),
		RestAPIEnabled:        parseBool(getenvAny("", "PALWORLD_REST_API_ENABLED", "REST_API_ENABLED"), false),
		RconEnabled:           parseBool(getenvAny("", "PALWORLD_RCON_ENABLED", "RCON_ENABLED"), true),
		PublicDomain:          getenvAny("", "PALWORLD_PUBLIC_DOMAIN", "PUBLIC_DOMAIN"),
		PublicIP:              getenvAny("", "PALWORLD_PUBLIC_IP", "PUBLIC_IP"),
		PublicPort:            getenvAny("8211", "PALWORLD_PUBLIC_PORT", "PUBLIC_PORT", "PALWORLD_PORT", "PORT"),
		ExpRate:               getenvFloatAny(1, "PALWORLD_EXP_RATE", "EXP_RATE"),
		CaptureRate:           getenvFloatAny(1, "PALWORLD_CAPTURE_RATE", "CAPTURE_RATE"),
		SpawnRate:             getenvFloatAny(1, "PALWORLD_SPAWN_RATE", "SPAWN_RATE"),
		CollectionDropRate:    getenvFloatAny(1, "PALWORLD_COLLECTION_DROP_RATE", "COLLECTION_DROP_RATE"),
		EnemyDropRate:         getenvFloatAny(1, "PALWORLD_ENEMY_DROP_RATE", "ENEMY_DROP_RATE"),
		EggHatchingHours:      getenvFloatAny(72, "PALWORLD_EGG_HATCHING_HOURS", "EGG_HATCHING_HOURS"),
		AutoSaveSpan:          getenvIntAny(30, "PALWORLD_AUTO_SAVE_SPAN", "AUTO_SAVE_SPAN"),
		DeathPenalty:          getenvAny("All", "PALWORLD_DEATH_PENALTY", "DEATH_PENALTY"),
		BaseCampWorkerMax:     getenvIntAny(15, "PALWORLD_BASE_CAMP_WORKER_MAX", "BASE_CAMP_WORKER_MAX"),
		GuildPlayerMax:        getenvIntAny(20, "PALWORLD_GUILD_PLAYER_MAX", "GUILD_PLAYER_MAX"),
		BaseCampMaxInGuild:    getenvIntAny(4, "PALWORLD_BASE_CAMP_MAX_IN_GUILD", "BASE_CAMP_MAX_IN_GUILD"),
		CrossplayPlatforms:    splitList(getenv("PALWORLD_CROSSPLAY_PLATFORMS", "Steam,Xbox,PS5,Mac")),
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
	if err := os.WriteFile(a.cfg.SettingsFile, append(raw, '\n'), 0o644); err != nil {
		return err
	}
	if a.cfg.WriteEnv {
		if err := updateEnvValues(a.cfg.EnvFile, settingsToEnv(settings)); err != nil {
			return err
		}
	}
	a.audit("info", "server", "已保存服务器参数到配置文件", actor, nil)
	return nil
}

func settingsToEnv(settings ServerSettings) map[string]string {
	return map[string]string{
		"SERVER_NAME": settings.ServerName, "SERVER_DESCRIPTION": settings.Description, "PLAYERS": strconv.Itoa(settings.Players),
		"SERVER_PASSWORD": settings.ServerPassword, "ADMIN_PASSWORD": settings.AdminPassword, "COMMUNITY": formatBool(settings.Community),
		"RCON_ENABLED": formatBool(settings.RconEnabled), "REST_API_ENABLED": formatBool(settings.RestAPIEnabled),
		"PALWORLD_SERVER_NAME": settings.ServerName, "PALWORLD_SERVER_DESCRIPTION": settings.Description, "PALWORLD_PLAYERS": strconv.Itoa(settings.Players),
		"PALWORLD_SERVER_PASSWORD": settings.ServerPassword, "PALWORLD_ADMIN_PASSWORD": settings.AdminPassword, "PALWORLD_COMMUNITY": formatBool(settings.Community),
		"PALWORLD_RCON_ENABLED": formatBool(settings.RconEnabled), "PALWORLD_REST_API_ENABLED": formatBool(settings.RestAPIEnabled),
		"PALWORLD_PUBLIC_DOMAIN": settings.PublicDomain, "PALWORLD_PUBLIC_IP": settings.PublicIP, "PALWORLD_PUBLIC_PORT": settings.PublicPort, "PALWORLD_EXP_RATE": trimFloat(settings.ExpRate),
		"PALWORLD_CAPTURE_RATE": trimFloat(settings.CaptureRate), "PALWORLD_SPAWN_RATE": trimFloat(settings.SpawnRate),
		"PALWORLD_COLLECTION_DROP_RATE": trimFloat(settings.CollectionDropRate), "PALWORLD_ENEMY_DROP_RATE": trimFloat(settings.EnemyDropRate),
		"PALWORLD_EGG_HATCHING_HOURS": trimFloat(settings.EggHatchingHours), "PALWORLD_AUTO_SAVE_SPAN": strconv.Itoa(settings.AutoSaveSpan),
		"PALWORLD_DEATH_PENALTY": settings.DeathPenalty, "PALWORLD_BASE_CAMP_WORKER_MAX": strconv.Itoa(settings.BaseCampWorkerMax),
		"PALWORLD_GUILD_PLAYER_MAX": strconv.Itoa(settings.GuildPlayerMax), "PALWORLD_BASE_CAMP_MAX_IN_GUILD": strconv.Itoa(settings.BaseCampMaxInGuild),
		"PALWORLD_CROSSPLAY_PLATFORMS": strings.Join(settings.CrossplayPlatforms, ","), "PALWORLD_AUTO_PAUSE_ENABLED": formatBool(settings.AutoPauseEnabled),
		"PALWORLD_PLAYER_LOGGING_ENABLED": formatBool(settings.PlayerLoggingEnabled), "PALWORLD_DISCORD_WEBHOOK_ENABLED": formatBool(settings.DiscordWebhookEnabled),
		"PALWORLD_TARGET_MANIFEST_ID": settings.TargetManifestID,
	}
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
	return os.WriteFile(envFile, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
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

func (a *App) detectVersion(inspect *dockerInspectResult) string {
	if value := getenv("PALWORLD_VERSION", ""); value != "" {
		return value
	}
	if value := steamBuildID(filepath.Join(a.cfg.DataDir, "steamapps", "appmanifest_2394010.acf")); value != "" {
		return "Build " + value
	}
	if inspect != nil {
		for _, key := range []string{"org.opencontainers.image.version", "version", "build_version"} {
			if value := strings.TrimSpace(inspect.Config.Labels[key]); value != "" {
				return value
			}
		}
		if parts := strings.Split(inspect.Config.Image, ":"); len(parts) > 1 && parts[len(parts)-1] != "latest" {
			return parts[len(parts)-1]
		}
	}
	return "unknown"
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

func (a *App) executeRcon(command string, timeout time.Duration) (RconCommandResult, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return RconCommandResult{}, APIError{Status: http.StatusBadRequest, Message: "RCON 命令不能为空"}
	}
	if !a.cfg.AllowRawRcon && !isAllowedRcon(command) {
		return RconCommandResult{}, APIError{Status: http.StatusBadRequest, Message: "该 RCON 命令不在白名单内；如需开放任意命令，请设置 PANEL_ALLOW_RAW_RCON=true"}
	}
	if a.cfg.RconPassword == "" {
		return RconCommandResult{}, errors.New("PALWORLD_ADMIN_PASSWORD 未配置，无法连接 RCON")
	}
	output, err := runRcon(a.cfg.RconHost, a.cfg.RconPort, a.cfg.RconPassword, command, timeout)
	if err != nil {
		return RconCommandResult{}, err
	}
	return RconCommandResult{Command: command, Output: output, ExecutedAt: formatTime(time.Now())}, nil
}

func isAllowedRcon(command string) bool {
	lower := strings.ToLower(command)
	for _, prefix := range allowedRconPrefixes {
		p := strings.ToLower(prefix)
		if lower == p || strings.HasPrefix(lower, p+" ") {
			return true
		}
	}
	return false
}

func runRcon(host string, port int, password, command string, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = 1800 * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	conn, err := (&net.Dialer{Timeout: minDuration(800*time.Millisecond, timeout)}).DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return "", fmt.Errorf("RCON 连接失败: %w", err)
	}
	defer conn.Close()
	deadline := time.Now().Add(timeout)
	_ = conn.SetDeadline(deadline)
	if err := writeRconPacket(conn, 1, 3, password); err != nil {
		return "", err
	}
	authed := false
	for time.Now().Before(deadline) {
		id, _, _, err := readRconPacket(conn)
		if err != nil {
			return "", fmt.Errorf("RCON 鉴权超时")
		}
		if id == -1 {
			return "", fmt.Errorf("RCON 鉴权失败，请检查管理员密码")
		}
		if id == 1 {
			authed = true
			break
		}
	}
	if !authed {
		return "", fmt.Errorf("RCON 鉴权超时")
	}
	if err := writeRconPacket(conn, 2, 2, command); err != nil {
		return "", err
	}
	_ = writeRconPacket(conn, 3, 2, "")
	var output strings.Builder
	gotBody := false
	for time.Now().Before(deadline) {
		if gotBody {
			_ = conn.SetReadDeadline(time.Now().Add(180 * time.Millisecond))
		} else {
			_ = conn.SetReadDeadline(deadline)
		}
		id, _, body, err := readRconPacket(conn)
		if err != nil {
			if gotBody && isTimeout(err) {
				break
			}
			return "", fmt.Errorf("RCON 连接超时")
		}
		if body != "" {
			output.WriteString(body)
			gotBody = true
		}
		if id == 3 && gotBody {
			break
		}
	}
	text := strings.TrimSpace(output.String())
	if text == "" {
		return "OK", nil
	}
	return text, nil
}

func writeRconPacket(w io.Writer, id, typ int32, body string) error {
	bodyBytes := []byte(body)
	size := int32(4 + 4 + len(bodyBytes) + 2)
	buf := make([]byte, 4+size)
	binary.LittleEndian.PutUint32(buf[0:4], uint32(size))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(id))
	binary.LittleEndian.PutUint32(buf[8:12], uint32(typ))
	copy(buf[12:], bodyBytes)
	_, err := w.Write(buf)
	return err
}

func readRconPacket(r io.Reader) (int32, int32, string, error) {
	var sizeBuf [4]byte
	if _, err := io.ReadFull(r, sizeBuf[:]); err != nil {
		return 0, 0, "", err
	}
	size := int(binary.LittleEndian.Uint32(sizeBuf[:]))
	if size < 10 || size > 1024*1024 {
		return 0, 0, "", fmt.Errorf("非法 RCON 包长度: %d", size)
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return 0, 0, "", err
	}
	id := int32(binary.LittleEndian.Uint32(buf[0:4]))
	typ := int32(binary.LittleEndian.Uint32(buf[4:8]))
	bodyLen := size - 10
	body := ""
	if bodyLen > 0 {
		body = strings.TrimRight(string(buf[8:8+bodyLen]), "\x00")
	}
	return id, typ, body, nil
}

func parsePlayers(output string) []Player {
	players := make([]Player, 0)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(strings.ToLower(line), "name,") {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 3 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		playerID := strings.TrimSpace(parts[1])
		steamID := strings.TrimSpace(parts[2])
		if name == "" || strings.EqualFold(name, "no online players") {
			continue
		}
		id := playerID
		if id == "" {
			id = steamID
		}
		if id == "" {
			id = fmt.Sprintf("player-%d", len(players)+1)
		}
		players = append(players, Player{ID: id, Name: name, Platform: "Steam", SteamID: orDefault(steamID, "-"), Level: 0, Guild: "-", Location: "-", OnlineFor: "-", Ping: 0, Status: "online"})
	}
	return players
}

func (a *App) audit(level, source, message, actor string, metadata map[string]any) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	row := LogEntry{ID: randomID(), Timestamp: time.Now().Format(time.RFC3339), Level: level, Source: source, Message: message}
	a.auditMu.Lock()
	defer a.auditMu.Unlock()
	_ = os.MkdirAll(filepath.Dir(a.cfg.AuditFile), 0o755)
	file, err := os.OpenFile(a.cfg.AuditFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	raw, _ := json.Marshal(row)
	_, _ = file.Write(append(raw, '\n'))
}

func (a *App) auditRows(limit int) []LogEntry {
	file, err := os.Open(a.cfg.AuditFile)
	if err != nil {
		return []LogEntry{}
	}
	defer file.Close()
	rows := make([]LogEntry, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var row LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &row); err == nil {
			row.Timestamp = formatTimeString(row.Timestamp)
			rows = append(rows, row)
		}
	}
	if len(rows) > limit {
		rows = rows[len(rows)-limit:]
	}
	return rows
}

func (a *App) startOperation(action, actor string, metadata map[string]any) string {
	id := randomID()
	a.writeOperation(OperationRecord{ID: id, Action: action, Status: "running", Actor: actor, Metadata: metadata, CreatedAt: time.Now().Format(time.RFC3339)})
	return id
}

func (a *App) finishOperation(id, status, message string) {
	a.writeOperation(OperationRecord{ID: id, Status: status, Message: message, CompletedAt: time.Now().Format(time.RFC3339)})
}

func (a *App) writeOperation(row OperationRecord) {
	a.opsMu.Lock()
	defer a.opsMu.Unlock()
	_ = os.MkdirAll(filepath.Dir(a.cfg.OpsFile), 0o755)
	file, err := os.OpenFile(a.cfg.OpsFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	raw, _ := json.Marshal(row)
	_, _ = file.Write(append(raw, '\n'))
}

func (a *App) backupRows(ctx context.Context, limit int) []Backup {
	entries, err := os.ReadDir(a.cfg.BackupsDir)
	if err != nil {
		return []Backup{}
	}
	rows := make([]Backup, 0)
	for _, entry := range entries {
		fullPath := filepath.Join(a.cfg.BackupsDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		size := info.Size()
		if entry.IsDir() {
			size = pathSizeBytes(ctx, fullPath)
		}
		backupType := "automatic"
		if strings.Contains(strings.ToLower(entry.Name()), "manual") {
			backupType = "manual"
		}
		note := "文件备份，恢复前需要手动解包。"
		if entry.IsDir() {
			note = "目录备份，可直接恢复。"
		}
		rows = append(rows, Backup{ID: entry.Name(), CreatedAt: formatTime(info.ModTime()), Size: formatBytes(size), Type: backupType, Status: "ready", Note: note})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].CreatedAt > rows[j].CreatedAt })
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}

func (a *App) createBackup(actor string) (map[string]any, error) {
	op := a.startOperation("backup:create", actor, nil)
	id := "manual-" + time.Now().Format("20060102-150405")
	target := filepath.Join(a.cfg.BackupsDir, id)
	if err := os.MkdirAll(a.cfg.BackupsDir, 0o755); err != nil {
		a.finishOperation(op, "failed", err.Error())
		return nil, err
	}
	_, _ = a.executeRcon("Save", a.cfg.RconTimeout)
	if err := copyDir(a.cfg.SavesDir, target); err != nil {
		a.finishOperation(op, "failed", err.Error())
		return nil, err
	}
	a.finishOperation(op, "success", "已创建手动备份 "+id)
	a.audit("info", "backup", "已创建手动备份 "+id, actor, map[string]any{"backupId": id})
	return map[string]any{"ok": true, "message": "已创建手动备份 " + id}, nil
}

func (a *App) restoreBackup(id, actor string) (map[string]any, error) {
	if !safeBackupID(id) {
		return nil, APIError{Status: http.StatusBadRequest, Message: "非法备份 ID"}
	}
	op := a.startOperation("backup:restore", actor, map[string]any{"backupId": id})
	source := filepath.Join(a.cfg.BackupsDir, id)
	info, err := os.Stat(source)
	if err != nil {
		a.finishOperation(op, "failed", err.Error())
		return nil, err
	}
	if !info.IsDir() {
		err := errors.New("当前只支持恢复目录备份")
		a.finishOperation(op, "failed", err.Error())
		return nil, err
	}
	preRestore := filepath.Join(a.cfg.BackupsDir, "pre-restore-"+time.Now().Format("20060102-150405"))
	_ = copyDir(a.cfg.SavesDir, preRestore)
	if err := os.RemoveAll(a.cfg.SavesDir); err != nil {
		a.finishOperation(op, "failed", err.Error())
		return nil, err
	}
	if err := copyDir(source, a.cfg.SavesDir); err != nil {
		a.finishOperation(op, "failed", err.Error())
		return nil, err
	}
	a.finishOperation(op, "success", "已恢复 "+id)
	a.audit("warn", "backup", "已恢复备份 "+id+"；重启游戏容器后生效。", actor, map[string]any{"backupId": id})
	return map[string]any{"ok": true, "message": "已恢复 " + id + "，建议立即重启游戏容器"}, nil
}

func (a *App) maintenance(action, actor string) (map[string]any, error) {
	if action == "" {
		return nil, APIError{Status: http.StatusBadRequest, Message: "维护动作不能为空"}
	}
	op := a.startOperation(action, actor, nil)
	fail := func(err error) (map[string]any, error) {
		a.finishOperation(op, "failed", err.Error())
		return nil, err
	}
	success := func(message string) (map[string]any, error) {
		a.finishOperation(op, "success", message)
		return map[string]any{"ok": true, "message": message}, nil
	}
	switch {
	case action == "rcon:save":
		result, err := a.executeRcon("Save", a.cfg.RconTimeout)
		if err != nil {
			return fail(err)
		}
		a.audit("info", "rcon", "已执行 Save", actor, nil)
		return success(result.Output)
	case action == "server:shutdown":
		result, err := a.executeRcon("Shutdown 300 服务器将在5分钟后关闭", a.cfg.RconTimeout)
		if err != nil {
			return fail(err)
		}
		a.audit("warn", "rcon", "已提交延迟关服命令", actor, nil)
		return success(result.Output)
	case action == "server:restart":
		ctx, cancel := context.WithTimeout(context.Background(), 70*time.Second)
		defer cancel()
		if _, err := runCmd(ctx, "", "docker", "restart", a.cfg.Container); err != nil {
			return fail(err)
		}
		a.audit("warn", "server", "已重启容器 "+a.cfg.Container, actor, nil)
		return success("容器重启命令已执行")
	case action == "server:update":
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
		defer cancel()
		if _, err := runCmd(ctx, a.cfg.ComposeDir, "docker", "compose", "pull", "palworld"); err != nil {
			return fail(err)
		}
		if _, err := runCmd(ctx, a.cfg.ComposeDir, "docker", "compose", "up", "-d", "palworld"); err != nil {
			return fail(err)
		}
		a.audit("warn", "update", "已执行服务端更新流程", actor, nil)
		return success("更新流程已执行")
	case action == "backup:create":
		a.finishOperation(op, "delegated", "转入备份创建流程")
		return a.createBackup(actor)
	case strings.HasPrefix(action, "backup:restore:"):
		a.finishOperation(op, "delegated", "转入备份恢复流程")
		return a.restoreBackup(strings.TrimPrefix(action, "backup:restore:"), actor)
	default:
		return fail(APIError{Status: http.StatusBadRequest, Message: "未知维护动作: " + action})
	}
}

func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s 不是目录", src)
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}

func (a *App) serveSPA(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, APIError{Status: http.StatusNotFound, Message: "接口不存在"})
		return
	}
	clean := pathpkg.Clean("/" + r.URL.Path)
	if hasDotPath(clean) {
		http.NotFound(w, r)
		return
	}
	if a.cfg.WebRoot == "" {
		http.NotFound(w, r)
		return
	}
	rel := strings.TrimPrefix(clean, "/")
	target := filepath.Join(a.cfg.WebRoot, filepath.FromSlash(rel))
	rootAbs, _ := filepath.Abs(a.cfg.WebRoot)
	targetAbs, _ := filepath.Abs(target)
	if !strings.HasPrefix(targetAbs, rootAbs) {
		http.NotFound(w, r)
		return
	}
	if info, err := os.Stat(targetAbs); err == nil && !info.IsDir() {
		http.ServeFile(w, r, targetAbs)
		return
	}
	if strings.HasPrefix(clean, "/assets/") {
		http.NotFound(w, r)
		return
	}
	index := filepath.Join(a.cfg.WebRoot, "index.html")
	if _, err := os.Stat(index); err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, index)
}

func hasDotPath(path string) bool {
	for _, part := range strings.Split(path, "/") {
		if strings.HasPrefix(part, ".") && part != "." {
			return true
		}
	}
	return false
}

func readJSON(r *http.Request, out any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(out)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	message := "Server error"
	var apiErr APIError
	if errors.As(err, &apiErr) {
		status = apiErr.Status
		message = apiErr.Message
	} else if err != nil {
		message = err.Error()
	}
	writeJSON(w, status, map[string]any{"error": message})
}

func runCmd(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return output, fmt.Errorf("%w: %s", err, msg)
		}
		return output, err
	}
	return output, nil
}

type diskInfo struct{ UsedGB, TotalGB float64 }

func diskUsage(ctx context.Context, target string) diskInfo {
	cctx, cancel := context.WithTimeout(ctx, 1800*time.Millisecond)
	defer cancel()
	output, err := runCmd(cctx, "", "df", "-k", target)
	if err != nil {
		return diskInfo{}
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 2 {
		return diskInfo{}
	}
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) < 3 {
		return diskInfo{}
	}
	total, _ := strconv.ParseFloat(fields[1], 64)
	used, _ := strconv.ParseFloat(fields[2], 64)
	return diskInfo{UsedGB: round1(used / 1024 / 1024), TotalGB: round1(total / 1024 / 1024)}
}

func pathSizeGB(ctx context.Context, target string) float64 {
	return round2(float64(pathSizeBytes(ctx, target)) / 1024 / 1024 / 1024)
}

func pathSizeBytes(ctx context.Context, target string) int64 {
	cctx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
	defer cancel()
	output, err := runCmd(cctx, "", "du", "-sk", target)
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(output))
	if len(fields) == 0 {
		return 0
	}
	kb, _ := strconv.ParseInt(fields[0], 10, 64)
	return kb * 1024
}

func parseHostCPU(value string) float64 {
	clean := strings.TrimSpace(strings.TrimSuffix(value, "%"))
	raw, _ := strconv.ParseFloat(clean, 64)
	if raw <= 0 {
		return 0
	}
	cores := runtime.NumCPU()
	if cores <= 0 {
		cores = 1
	}
	return round1(raw / float64(cores))
}

func parseMemoryUsage(value string) (float64, float64) {
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return 0, 0
	}
	return round1(memoryToGB(parts[0])), round1(memoryToGB(parts[1]))
}

func memoryToGB(value string) float64 {
	amount, unit := splitNumberUnit(value)
	if amount == 0 {
		return 0
	}
	switch {
	case strings.HasPrefix(unit, "ki"), unit == "kb":
		return amount / 1024 / 1024
	case strings.HasPrefix(unit, "mi"), unit == "mb":
		return amount / 1024
	case strings.HasPrefix(unit, "gi"), unit == "gb":
		return amount
	case strings.HasPrefix(unit, "ti"), unit == "tb":
		return amount * 1024
	default:
		return amount / 1024 / 1024 / 1024
	}
}

func splitNumberUnit(value string) (float64, string) {
	text := strings.TrimSpace(value)
	if text == "" {
		return 0, ""
	}
	end := 0
	for end < len(text) {
		c := text[end]
		if (c >= '0' && c <= '9') || c == '.' {
			end++
			continue
		}
		break
	}
	if end == 0 {
		return 0, strings.ToLower(strings.TrimSpace(text))
	}
	amount, err := strconv.ParseFloat(text[:end], 64)
	if err != nil {
		return 0, strings.ToLower(strings.TrimSpace(text[end:]))
	}
	return amount, strings.ToLower(strings.TrimSpace(text[end:]))
}

func steamBuildID(manifestPath string) string {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return ""
	}
	for _, key := range []string{"buildid", "TargetBuildID"} {
		for _, line := range strings.Split(string(raw), "\n") {
			fields := strings.Fields(strings.ReplaceAll(line, "\"", ""))
			if len(fields) >= 2 && fields[0] == key {
				return fields[1]
			}
		}
	}
	return ""
}

func totalMemoryBytes() uint64 {
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	output, err := runCmd(ctx, "", "sh", "-lc", "awk '/MemTotal/ {print $2*1024}' /proc/meminfo 2>/dev/null")
	if err == nil {
		value, _ := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64)
		if value > 0 {
			return value
		}
	}
	return 0
}

func lastModified(target string) string {
	info, err := os.Stat(target)
	if err != nil {
		return "-"
	}
	return formatTime(info.ModTime())
}

func formatTimeString(value string) string {
	if strings.TrimSpace(value) == "" || strings.HasPrefix(value, "0001-") {
		return "-"
	}
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return value
	}
	return formatTime(t)
}

func serverTimezone() string {
	if value := strings.TrimSpace(getenv("TZ", "")); value != "" {
		return value
	}
	location := time.Now().Location().String()
	if location != "" && location != "Local" {
		return location
	}
	_, offset := time.Now().Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	return fmt.Sprintf("UTC%s%02d:%02d", sign, offset/3600, (offset%3600)/60)
}

func formatTime(t time.Time) string { return t.Local().Format("2006-01-02 15:04:05") }

func formatUptime(startedAt string) string {
	if startedAt == "" || strings.HasPrefix(startedAt, "0001-") {
		return "-"
	}
	start, err := time.Parse(time.RFC3339Nano, startedAt)
	if err != nil {
		return "-"
	}
	seconds := int64(time.Since(start).Seconds())
	if seconds < 0 {
		seconds = 0
	}
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	if days > 0 {
		return fmt.Sprintf("%d 天 %d 小时 %d 分", days, hours, minutes)
	}
	return fmt.Sprintf("%d 小时 %d 分", hours, minutes)
}

func formatBytes(size int64) string {
	if size <= 0 {
		return "-"
	}
	mb := float64(size) / 1024 / 1024
	if mb < 1024 {
		return fmt.Sprintf("%.0f MB", math.Max(1, mb))
	}
	return fmt.Sprintf("%.1f GB", mb/1024)
}

func formatEnvValue(value string) string {
	if value == "" || strings.ContainsAny(value, " \t#\"'") {
		raw, _ := json.Marshal(value)
		return string(raw)
	}
	return value
}

func formatBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func trimFloat(value float64) string { return strconv.FormatFloat(value, 'f', -1, 64) }

func splitList(value string) []string {
	var result []string
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	if len(result) == 0 {
		return []string{"Steam", "Xbox", "PS5", "Mac"}
	}
	return result
}

func readEnvFile(filePath string) map[string]string {
	result := map[string]string{}
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return result
	}
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || !strings.Contains(trimmed, "=") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		key := strings.TrimSpace(parts[0])
		if key == "" {
			continue
		}
		result[key] = parseEnvValue(strings.TrimSpace(parts[1]))
	}
	return result
}

func loadDotEnv(filePath string) {
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || !strings.Contains(trimmed, "=") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		key := strings.TrimSpace(parts[0])
		value := parseEnvValue(strings.TrimSpace(parts[1]))
		if key != "" && os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
}

func parseEnvValue(value string) string {
	if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
		unquoted, err := strconv.Unquote(value)
		if err == nil {
			return unquoted
		}
		return value[1 : len(value)-1]
	}
	return value
}

func envValueAny(fileEnv map[string]string, fallback string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(fileEnv[key]); value != "" {
			return value
		}
	}
	return getenvAny(fallback, keys...)
}

func getenv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func getenvAny(fallback string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return fallback
}

func envIntAny(fileEnv map[string]string, fallback int, keys ...string) int {
	for _, key := range keys {
		if value, err := strconv.Atoi(strings.TrimSpace(fileEnv[key])); err == nil {
			return value
		}
	}
	return getenvIntAny(fallback, keys...)
}

func getenvInt(key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil {
		return fallback
	}
	return value
}

func getenvIntAny(fallback int, keys ...string) int {
	for _, key := range keys {
		if value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key))); err == nil {
			return value
		}
	}
	return fallback
}

func getenvFloatAny(fallback float64, keys ...string) float64 {
	for _, key := range keys {
		if value, err := strconv.ParseFloat(strings.TrimSpace(os.Getenv(key)), 64); err == nil {
			return value
		}
	}
	return fallback
}

func parseBool(value string, fallback bool) bool {
	if value == "" {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func randomID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:])
}

func safeBackupID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func round1(value float64) float64 { return math.Round(value*10) / 10 }
func round2(value float64) float64 { return math.Round(value*100) / 100 }

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func orDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
