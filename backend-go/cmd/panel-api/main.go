package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	_ "time/tzdata"
)

type Config struct {
	Bind            string
	Port            int
	AuthPassword    string
	TokenSecret     string
	TokenTTL        time.Duration
	CorsOrigin      string
	WebRoot         string
	StateDir        string
	SettingsFile    string
	AuditFile       string
	OpsFile         string
	DataDir         string
	SavesDir        string
	BackupsDir      string
	ComposeDir      string
	ComposeProject  string
	EnvFile         string
	Container       string
	RconHost        string
	RconPort        int
	RconPassword    string
	RconTimeout     time.Duration
	AllowRawRcon    bool
	WriteEnv        bool
	DisplayHost     string
	PublicDomain    string
	Timezone        string
	RestURL         string
	WorldIndexURL   string
	WorldIndexPass  string
	WorldSnapshot   string
	WorldRefresh    time.Duration
	EditorURL       string
	EditorDir       string
	EditorImage     string
	EditorWorkspace string
	AdvancedCompose string
	EditorApply     bool
}

type App struct {
	cfg           Config
	rconMu        sync.Mutex
	playersMu     sync.RWMutex
	lastPlayers   []Player
	lastPlayersAt time.Time
	auditMu       sync.Mutex
	opsMu         sync.Mutex
	snapshotMu    sync.Mutex
}

type APIError struct {
	Status  int
	Message string
}

func (e APIError) Error() string { return e.Message }

type ServerSettings struct {
	ServerName            string            `json:"serverName"`
	Description           string            `json:"description"`
	Players               int               `json:"players"`
	ServerPassword        string            `json:"serverPassword"`
	AdminPassword         string            `json:"adminPassword"`
	Community             bool              `json:"community"`
	RestAPIEnabled        bool              `json:"restApiEnabled"`
	RconEnabled           bool              `json:"rconEnabled"`
	PublicDomain          string            `json:"publicDomain"`
	PublicIP              string            `json:"publicIp"`
	PublicPort            string            `json:"publicPort"`
	ExpRate               float64           `json:"expRate"`
	CaptureRate           float64           `json:"captureRate"`
	SpawnRate             float64           `json:"spawnRate"`
	CollectionDropRate    float64           `json:"collectionDropRate"`
	EnemyDropRate         float64           `json:"enemyDropRate"`
	EggHatchingHours      float64           `json:"eggHatchingHours"`
	AutoSaveSpan          int               `json:"autoSaveSpan"`
	DeathPenalty          string            `json:"deathPenalty"`
	BaseCampWorkerMax     int               `json:"baseCampWorkerMax"`
	GuildPlayerMax        int               `json:"guildPlayerMax"`
	BaseCampMaxInGuild    int               `json:"baseCampMaxInGuild"`
	CrossplayPlatforms    []string          `json:"crossplayPlatforms"`
	GameParameters        map[string]string `json:"gameParameters"`
	AutoPauseEnabled      bool              `json:"autoPauseEnabled"`
	PlayerLoggingEnabled  bool              `json:"playerLoggingEnabled"`
	DiscordWebhookEnabled bool              `json:"discordWebhookEnabled"`
	TargetManifestID      string            `json:"targetManifestId"`
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
	Name                 string            `json:"name"`
	Host                 string            `json:"host"`
	Address              string            `json:"address"`
	Version              string            `json:"version"`
	GameVersion          string            `json:"gameVersion"`
	SteamBuildID         string            `json:"steamBuildId"`
	VersionSource        string            `json:"versionSource"`
	Timezone             string            `json:"timezone"`
	Container            string            `json:"container"`
	Image                string            `json:"image"`
	Health               string            `json:"health"`
	StartedAt            string            `json:"startedAt"`
	Uptime               string            `json:"uptime"`
	PlayersOnline        int               `json:"playersOnline"`
	PlayersMax           int               `json:"playersMax"`
	CPU                  float64           `json:"cpu"`
	MemoryUsedGB         float64           `json:"memoryUsedGb"`
	MemoryLimitGB        float64           `json:"memoryLimitGb"`
	DiskUsedGB           float64           `json:"diskUsedGb"`
	DiskTotalGB          float64           `json:"diskTotalGb"`
	WorldSizeGB          float64           `json:"worldSizeGb"`
	LastSaveAt           string            `json:"lastSaveAt"`
	NextBackupAt         string            `json:"nextBackupAt"`
	NextRestartAt        string            `json:"nextRestartAt"`
	ConfigSavedAt        string            `json:"configSavedAt"`
	ConfigLoadedAt       string            `json:"configLoadedAt"`
	ConfigPendingRestart bool              `json:"configPendingRestart"`
	ConfigPendingKeys    []string          `json:"configPendingKeys"`
	Ports                []PortBinding     `json:"ports"`
	Maintenance          MaintenancePolicy `json:"maintenance"`
}

type Player struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	PlayerUID     string  `json:"playerUid"`
	Platform      string  `json:"platform"`
	SteamID       string  `json:"steamId"`
	UserID        string  `json:"userId,omitempty"`
	AccountName   string  `json:"accountName,omitempty"`
	IP            string  `json:"ip,omitempty"`
	Ping          float64 `json:"ping,omitempty"`
	LocationX     float64 `json:"locationX,omitempty"`
	LocationY     float64 `json:"locationY,omitempty"`
	Level         int     `json:"level,omitempty"`
	BuildingCount int     `json:"buildingCount,omitempty"`
	Status        string  `json:"status"`
	Manageable    bool    `json:"manageable"`
}

type LogEntry struct {
	ID        string         `json:"id"`
	Timestamp string         `json:"timestamp"`
	Level     string         `json:"level"`
	Source    string         `json:"source"`
	Message   string         `json:"message"`
	Actor     string         `json:"actor,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type Backup struct {
	ID         string `json:"id"`
	CreatedAt  string `json:"createdAt"`
	Size       string `json:"size"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Format     string `json:"format"`
	Restorable bool   `json:"restorable"`
	Note       string `json:"note"`
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
	{ID: "kick", Label: "踢出玩家", Command: "KickPlayer <SteamID>", Description: "把指定玩家踢下线，需要把 <SteamID> 替换成真实值。", Risk: "medium", Category: "player"},
	{ID: "ban", Label: "封禁玩家", Command: "BanPlayer <SteamID>", Description: "封禁指定玩家，需要谨慎执行并记录原因。", Risk: "high", Category: "player"},
	{ID: "shutdown", Label: "延迟关服", Command: "Shutdown 300 Server_shutdown_in_5_minutes", Description: "倒计时关服并给玩家提示；RCON 提示文本只支持 ASCII。", Risk: "high", Category: "shutdown"},
}

var allowedRconPrefixes = []string{"Info", "ShowPlayers", "Save", "KickPlayer", "BanPlayer", "Shutdown"}

func main() {
	loadDotEnv(".env")
	cfg := loadConfig()
	if cfg.AuthPassword == "" || cfg.AuthPassword == "change-panel-password" {
		log.Fatal("PANEL_AUTH_PASSWORD must be configured with a non-default value")
	}
	if cfg.TokenSecret == "" {
		log.Fatal("PANEL_TOKEN_SECRET or PANEL_JWT_SECRET must be configured")
	}
	setDisplayTimezone(cfg.Timezone)
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		log.Fatalf("create state dir: %v", err)
	}

	app := &App{cfg: cfg}
	app.startWorldSnapshotWatcher()
	addr := fmt.Sprintf("%s:%d", cfg.Bind, cfg.Port)
	log.Printf("palworld panel api listening on %s", addr)
	if err := http.ListenAndServe(addr, app.routes()); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func loadConfig() Config {
	stateDir := getenv("PANEL_STATE_DIR", ".panel-state")
	timeoutMs := getenvInt("PANEL_RCON_TIMEOUT_MS", 3000)
	return Config{
		Bind:            getenvAny("0.0.0.0", "PANEL_API_BIND", "HOST"),
		Port:            getenvIntAny(16824, "PANEL_API_PORT", "APP_API_PORT", "PORT"),
		AuthPassword:    getenv("PANEL_AUTH_PASSWORD", "change-panel-password"),
		TokenSecret:     getenvAny("", "PANEL_TOKEN_SECRET", "PANEL_JWT_SECRET", "PANEL_AUTH_PASSWORD"),
		TokenTTL:        time.Duration(getenvInt("PANEL_TOKEN_TTL_SECONDS", 60*60*24*7)) * time.Second,
		CorsOrigin:      getenv("PANEL_CORS_ORIGIN", "*"),
		WebRoot:         getenv("PANEL_WEB_ROOT", "dist"),
		StateDir:        stateDir,
		SettingsFile:    getenv("PANEL_SETTINGS_FILE", filepath.Join(stateDir, "settings.json")),
		AuditFile:       getenv("PANEL_AUDIT_FILE", filepath.Join(stateDir, "audit.jsonl")),
		OpsFile:         getenv("PANEL_OPS_FILE", filepath.Join(stateDir, "operations.jsonl")),
		DataDir:         getenv("PALWORLD_DATA_DIR", "/palworld"),
		SavesDir:        getenvAny("/palworld/Pal/Saved/SaveGames", "PALWORLD_SAVES_DIR", "PALWORLD_SAVE_DIR"),
		BackupsDir:      getenv("PALWORLD_BACKUP_DIR", "/palworld/backups"),
		ComposeDir:      getenv("PALWORLD_COMPOSE_DIR", "."),
		ComposeProject:  getenvAny("palworld-server", "PALWORLD_COMPOSE_PROJECT", "PANEL_COMPOSE_PROJECT", "COMPOSE_PROJECT_NAME"),
		EnvFile:         getenv("PANEL_ENV_FILE", filepath.Join(getenv("PALWORLD_COMPOSE_DIR", "."), ".env")),
		Container:       getenv("PALWORLD_CONTAINER", "palworld-server"),
		RconHost:        getenv("PALWORLD_RCON_HOST", "127.0.0.1"),
		RconPort:        getenvIntAny(25575, "PALWORLD_RCON_PORT", "RCON_PORT"),
		RconPassword:    getenvAny("", "PALWORLD_ADMIN_PASSWORD", "ADMIN_PASSWORD"),
		RconTimeout:     time.Duration(timeoutMs) * time.Millisecond,
		AllowRawRcon:    parseBool(getenv("PANEL_ALLOW_RAW_RCON", ""), false),
		WriteEnv:        parseBool(getenv("PANEL_WRITE_ENV", ""), true),
		DisplayHost:     getenv("PANEL_DISPLAY_HOST", ""),
		PublicDomain:    getenvAny("", "PALWORLD_PUBLIC_DOMAIN", "PUBLIC_DOMAIN"),
		Timezone:        getenv("TZ", "Asia/Shanghai"),
		RestURL:         strings.TrimRight(getenv("PALWORLD_REST_URL", "http://127.0.0.1:8212"), "/"),
		WorldIndexURL:   strings.TrimRight(getenv("PALWORLD_WORLD_INDEX_URL", "http://127.0.0.1:16826"), "/"),
		WorldIndexPass:  getenvAny("", "PALWORLD_WORLD_INDEX_PASSWORD", "PANEL_AUTH_PASSWORD"),
		WorldSnapshot:   getenv("PALWORLD_WORLD_SNAPSHOT_DIR", filepath.Join(stateDir, "world-snapshot")),
		WorldRefresh:    time.Duration(getenvInt("PALWORLD_WORLD_INDEX_REFRESH_SECONDS", 60)) * time.Second,
		EditorURL:       strings.TrimRight(getenv("PALWORLD_SAVE_EDITOR_URL", "http://127.0.0.1:16827"), "/"),
		EditorDir:       getenv("PALWORLD_SAVE_EDITOR_DIR", filepath.Join(getenv("PALWORLD_COMPOSE_DIR", "."), "tools", "palworld-save-pal")),
		EditorImage:     getenv("PALWORLD_SAVE_EDITOR_IMAGE", "palworld-save-editor:v0.17.4"),
		EditorWorkspace: getenv("PALWORLD_SAVE_EDITOR_WORKSPACE", filepath.Join(filepath.Dir(getenv("PALWORLD_WORLD_SNAPSHOT_DIR", filepath.Join(stateDir, "world-snapshot"))), "editor-workspace")),
		AdvancedCompose: getenv("PALWORLD_ADVANCED_COMPOSE_FILE", filepath.Join(getenv("PALWORLD_COMPOSE_DIR", "."), "docker-compose.advanced.yml")),
		EditorApply:     parseBool(getenv("PANEL_EDITOR_APPLY_ENABLED", "false"), false),
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
	a.registerAdvancedRoutes(mux)
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
	return a.signTokenTTL(username, a.cfg.TokenTTL)
}

func (a *App) signTokenTTL(username string, ttl time.Duration) (string, error) {
	payload := map[string]any{"u": username, "exp": time.Now().Add(ttl).Unix()}
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
	return a.verifyToken(strings.TrimSpace(header[7:]))
}

func (a *App) verifyToken(token string) (string, bool) {
	parts := strings.Split(token, ".")
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
