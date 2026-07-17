import { apiRequest } from '@/api/client'

export type ServerHealth = 'healthy' | 'starting' | 'warning' | 'offline'
export type BackupStatus = 'ready' | 'running' | 'failed'
export type OperationRisk = 'low' | 'medium' | 'high'
export type LogLevel = 'info' | 'warn' | 'error'

export interface ServerStatus {
  name: string
  host: string
  address: string
  version: string
  gameVersion: string
  steamBuildId: string
  versionSource: 'environment' | 'rcon' | 'unavailable'
  timezone: string
  container: string
  image: string
  health: ServerHealth
  startedAt: string
  uptime: string
  playersOnline: number
  playersMax: number
  cpu: number
  memoryUsedGb: number
  memoryLimitGb: number
  diskUsedGb: number
  diskTotalGb: number
  worldSizeGb: number
  lastSaveAt: string
  nextBackupAt: string
  nextRestartAt: string
  ports: PortBinding[]
  maintenance: MaintenancePolicy
}

export interface PortBinding {
  port: number
  protocol: 'UDP' | 'TCP'
  exposure: 'public' | 'local'
  purpose: string
  safe: boolean
}

export interface Player {
  id: string
  name: string
  playerUid: string
  platform: 'Steam' | 'Xbox' | 'Unknown'
  steamId: string
  userId?: string
  accountName?: string
  ip?: string
  ping?: number
  locationX?: number
  locationY?: number
  level?: number
  buildingCount?: number
  status: 'online'
  manageable: boolean
}

export interface LogEntry {
  id: string
  timestamp: string
  level: LogLevel
  source: 'server' | 'rcon' | 'backup' | 'update'
  message: string
}

export interface Backup {
  id: string
  createdAt: string
  size: string
  type: 'automatic' | 'manual'
  status: BackupStatus
  format: 'directory' | 'tar.gz' | 'file'
  restorable: boolean
  note: string
}

export interface MaintenancePolicy {
  updateOnBoot: boolean
  autoUpdate: boolean
  autoUpdateCron: string
  autoReboot: boolean
  autoRebootCron: string
  backupEnabled: boolean
  backupCron: string
  backupRetention: number
}

export const DEFAULT_GAME_PARAMETERS = {
  DIFFICULTY: 'None',
  RANDOMIZER_TYPE: 'None',
  RANDOMIZER_SEED: '',
  IS_RANDOMIZER_PAL_LEVEL_RANDOM: 'False',
  DAYTIME_SPEEDRATE: '1.000000',
  NIGHTTIME_SPEEDRATE: '1.000000',
  PAL_DAMAGE_RATE_ATTACK: '1.000000',
  PAL_DAMAGE_RATE_DEFENSE: '1.000000',
  PLAYER_DAMAGE_RATE_ATTACK: '1.000000',
  PLAYER_DAMAGE_RATE_DEFENSE: '1.000000',
  PLAYER_STOMACH_DECREASE_RATE: '1.000000',
  PLAYER_STAMINA_DECREASE_RATE: '1.000000',
  PLAYER_AUTO_HP_REGEN_RATE: '1.000000',
  PLAYER_AUTO_HP_REGEN_RATE_IN_SLEEP: '1.000000',
  PAL_STOMACH_DECREASE_RATE: '1.000000',
  PAL_STAMINA_DECREASE_RATE: '1.000000',
  PAL_AUTO_HP_REGEN_RATE: '1.000000',
  PAL_AUTO_HP_REGEN_RATE_IN_SLEEP: '1.000000',
  BUILD_OBJECT_HP_RATE: '1.000000',
  BUILD_OBJECT_DAMAGE_RATE: '1.000000',
  BUILD_OBJECT_DETERIORATION_DAMAGE_RATE: '1.000000',
  COLLECTION_OBJECT_HP_RATE: '1.000000',
  COLLECTION_OBJECT_RESPAWN_SPEED_RATE: '1.000000',
  ENABLE_PLAYER_TO_PLAYER_DAMAGE: 'False',
  ENABLE_FRIENDLY_FIRE: 'False',
  ENABLE_INVADER_ENEMY: 'True',
  ACTIVE_UNKO: 'False',
  ENABLE_AIM_ASSIST_PAD: 'True',
  ENABLE_AIM_ASSIST_KEYBOARD: 'False',
  DROP_ITEM_MAX_NUM: '3000',
  DROP_ITEM_MAX_NUM_UNKO: '100',
  BASE_CAMP_MAX_NUM: '128',
  DROP_ITEM_ALIVE_MAX_HOURS: '1.000000',
  AUTO_RESET_GUILD_NO_ONLINE_PLAYERS: 'False',
  AUTO_RESET_GUILD_TIME_NO_ONLINE_PLAYERS: '72.000000',
  WORK_SPEED_RATE: '1.000000',
  IS_MULTIPLAY: 'False',
  IS_PVP: 'False',
  HARDCORE: 'False',
  CHARACTER_RECREATE_IN_HARDCORE: 'False',
  PAL_LOST: 'False',
  CAN_PICKUP_OTHER_GUILD_DEATH_PENALTY_DROP: 'False',
  ENABLE_NON_LOGIN_PENALTY: 'True',
  ENABLE_FAST_TRAVEL: 'True',
  IS_START_LOCATION_SELECT_BY_MAP: 'False',
  EXIST_PLAYER_AFTER_LOGOUT: 'False',
  ENABLE_DEFENSE_OTHER_GUILD_PLAYER: 'False',
  INVISIBLE_OTHER_GUILD_BASE_CAMP_AREA_FX: 'False',
  BUILD_AREA_LIMIT: 'False',
  ITEM_WEIGHT_RATE: '1.000000',
  COOP_PLAYER_MAX_NUM: '4',
  REGION: '',
  USEAUTH: 'True',
  BAN_LIST_URL: 'https://b.palworldgame.com/api/banlist.txt',
  SHOW_PLAYER_LIST: 'False',
  CHAT_POST_LIMIT_PER_MINUTE: '30',
  USE_BACKUP_SAVE_DATA: 'True',
  SUPPLY_DROP_SPAN: '180',
  ENABLE_PREDATOR_BOSS_PAL: 'True',
  MAX_BUILDING_LIMIT_NUM: '0',
  SERVER_REPLICATE_PAWN_CULL_DISTANCE: '15000.000000',
  ALLOW_GLOBAL_PALBOX_EXPORT: 'True',
  ALLOW_GLOBAL_PALBOX_IMPORT: 'False',
  EQUIPMENT_DURABILITY_DAMAGE_RATE: '1.000000',
  ITEM_CONTAINER_FORCE_MARK_DIRTY_INTERVAL: '1.000000',
  ITEM_CORRUPTION_MULTIPLIER: '1.000000',
  PHYSICS_ACTIVE_DROP_ITEM_MAX_NUM: '-1',
  ALLOW_CLIENT_MOD: 'True',
  PLAYER_DATA_PAL_STORAGE_UPDATE_CHECK_TICK_INTERVAL: '1.000000',
  LOG_FORMAT_TYPE: 'Text',
  IS_SHOW_JOIN_LEFT_MESSAGE: 'True',
  MONSTER_FARM_ACTION_SPEED_RATE: '1.000000',
  DENY_TECHNOLOGY_LIST: '',
  GUILD_REJOIN_COOLDOWN_MINUTES: '0',
  AUTO_TRANSFER_MASTER_CHECK_INTERVAL_SECONDS: '3600.000000',
  AUTO_TRANSFER_MASTER_THRESHOLD_DAYS: '14',
  MAX_GUILDS_PER_FRAME: '10',
  BLOCK_RESPAWN_TIME: '5.000000',
  RESPAWN_PENALTY_DURATION_THRESHOLD: '0.000000',
  RESPAWN_PENALTY_TIME_SCALE: '2.000000',
  DISPLAY_PVP_ITEM_NUM_ON_WORLD_MAP_BASE_CAMP: 'False',
  DISPLAY_PVP_ITEM_NUM_ON_WORLD_MAP_PLAYER: 'False',
  ADDITIONAL_DROP_ITEM_WHEN_PLAYER_KILLING_IN_PVP_MODE: 'PlayerDropItem',
  ADDITIONAL_DROP_ITEM_NUM_WHEN_PLAYER_KILLING_IN_PVP_MODE: '1',
  ADDITIONAL_DROP_ITEM_WHEN_PLAYER_KILLING_IN_PVP_MODE_ENABLED: 'False',
  ENABLE_VOICE_CHAT: 'False',
  VOICE_CHAT_MAX_VOLUME_DISTANCE: '3000.000000',
  VOICE_CHAT_ZERO_VOLUME_DISTANCE: '15000.000000',
  ALLOW_ENHANCE_STAT_HEALTH: 'True',
  ALLOW_ENHANCE_STAT_ATTACK: 'True',
  ALLOW_ENHANCE_STAT_STAMINA: 'True',
  ALLOW_ENHANCE_STAT_WEIGHT: 'True',
  ALLOW_ENHANCE_STAT_WORK_SPEED: 'True',
  ENABLE_BUILDING_PLAYER_UID_DISPLAY: 'False',
  BUILDING_NAME_DISPLAY_CACHE_TTL_SECONDS: '60',
} as const satisfies Record<string, string>

export type GameParameterName = keyof typeof DEFAULT_GAME_PARAMETERS

export interface ServerSettings {
  serverName: string
  description: string
  players: number
  serverPassword: string
  adminPassword: string
  community: boolean
  restApiEnabled: boolean
  rconEnabled: boolean
  publicDomain: string
  publicIp: string
  publicPort: string
  expRate: number
  captureRate: number
  spawnRate: number
  collectionDropRate: number
  enemyDropRate: number
  eggHatchingHours: number
  autoSaveSpan: number
  deathPenalty: 'None' | 'Item' | 'ItemAndEquipment' | 'All'
  baseCampWorkerMax: number
  guildPlayerMax: number
  baseCampMaxInGuild: number
  gameParameters: Record<GameParameterName, string>
  crossplayPlatforms: string[]
  autoPauseEnabled: boolean
  playerLoggingEnabled: boolean
  discordWebhookEnabled: boolean
  targetManifestId: string
}

export interface RconCommandResult {
  command: string
  output: string
  executedAt: string
}

export interface AnnouncementResult {
  ok: boolean
  message: string
  transport: string
  sentAt: string
}

export interface RconCommandDefinition {
  id: string
  label: string
  command: string
  description: string
  risk: OperationRisk
  category: 'info' | 'player' | 'world' | 'broadcast' | 'shutdown'
}

export type AdvancedLayerState = 'ready' | 'disabled' | 'pending-restart' | 'degraded' | 'not-installed' | 'snapshot-ready' | 'locked'

export interface AdvancedLayer {
  id: 'realtime' | 'world-index' | 'save-editor'
  label: string
  state: AdvancedLayerState
  installed: boolean
  reachable: boolean
  readOnly: boolean
  requiresRestart: boolean
  source: string
  message: string
}

export interface AdvancedCapabilities {
  layers: AdvancedLayer[]
  safety: {
    gameRunning: boolean
    playersOnline: number
    snapshotAvailable: boolean
    canEditSnapshot: boolean
    canApplyToWorld: boolean
    applyEnabled: boolean
  }
  observedAt: string
}

export interface DataMeta {
  source: string
  observedAt: string
  stale: boolean
  snapshotId?: string
}

export interface DataEnvelope<T> {
  meta: DataMeta
  data: T
}

export interface LiveMetrics {
  serverFps: number
  currentPlayers: number
  maxPlayers: number
  serverFrameTime: number
  uptimeSeconds: number
  inGameDays: number
  source: string
  observedAt: string
}

export interface WorldPal {
  level: number
  type: string
  gender: string
  nickname: string
  is_lucky: boolean
  is_boss: boolean
  workspeed: number
  melee: number
  ranged: number
  defense: number
  skills: string[]
}

export interface WorldItem {
  SlotIndex: number
  ItemId: string
  StackCount: number
}

export interface WorldItems {
  CommonContainerId?: WorldItem[]
  DropSlotContainerId?: WorldItem[]
  EssentialContainerId?: WorldItem[]
  FoodEquipContainerId?: WorldItem[]
  PlayerEquipArmorContainerId?: WorldItem[]
  WeaponLoadOutContainerId?: WorldItem[]
}

export interface WorldPlayer {
  player_uid: string
  nickname: string
  level: number
  exp: number
  hp: number
  max_hp: number
  shield_hp: number
  shield_max_hp: number
  full_stomach: number
  save_last_online: string
  last_online: string
  steam_id: string
  user_id: string
  account_name: string
  ip: string
  ping: number
  location_x: number
  location_y: number
  building_count: number
  pals?: WorldPal[]
  items?: WorldItems
}

export interface GuildMember {
  player_uid: string
  nickname: string
}

export interface BaseCamp {
  id: string
  area: number
  location_x: number
  location_y: number
}

export interface WorldGuild {
  name: string
  base_camp_level: number
  admin_player_uid: string
  players: GuildMember[]
  base_camp: BaseCamp[]
}

export interface LiveMapData {
  players: Player[]
  guilds: WorldGuild[]
}

export interface WorldSnapshot {
  id: string
  backupId: string
  createdAt: string
  refreshedAt: string
  sourceDir: string
}

export interface WorldStatus {
  snapshot: WorldSnapshot
  indexReachable: boolean
  editorInstalled: boolean
  latestBackupId: string
  upToDate: boolean
  autoRefreshSeconds: number
}

export interface EditorStatus {
  installed: boolean
  reachable: boolean
  url: string
  applyEnabled: boolean
  safety: AdvancedCapabilities['safety']
  supportedActions: string[]
}

export interface EditorSessionResult {
  ok: boolean
  message: string
  url?: string
}

const USE_MOCK_API = import.meta.env.VITE_USE_MOCK_API === 'true'

const status: ServerStatus = {
  name: 'Palworld Dedicated Server',
  host: 'local-demo',
  address: 'your-domain.example:8211',
  version: 'v0.7.3.90464',
  gameVersion: 'v0.7.3.90464',
  steamBuildId: '22460594',
  versionSource: 'rcon',
  timezone: 'Asia/Shanghai',
  container: 'palworld-server',
  image: 'thijsvanloef/palworld-server-docker:latest',
  health: 'healthy',
  startedAt: '2026-07-09 09:03:31',
  uptime: '2 小时 18 分',
  playersOnline: 0,
  playersMax: 32,
  cpu: 12.8,
  memoryUsedGb: 1.2,
  memoryLimitGb: 62.4,
  diskUsedGb: 3.6,
  diskTotalGb: 1800,
  worldSizeGb: 0.08,
  lastSaveAt: '2026-07-09 09:05:50',
  nextBackupAt: '10:00',
  nextRestartAt: '明天 05:00',
  ports: [
    { port: 8211, protocol: 'UDP', exposure: 'public', purpose: '游戏连接端口', safe: true },
    { port: 27015, protocol: 'UDP', exposure: 'public', purpose: 'Steam 查询端口', safe: true },
    { port: 25575, protocol: 'TCP', exposure: 'local', purpose: 'RCON 管理端口', safe: true },
    { port: 8212, protocol: 'TCP', exposure: 'local', purpose: 'REST API 管理端口', safe: true },
  ],
  maintenance: {
    updateOnBoot: true,
    autoUpdate: true,
    autoUpdateCron: '0 4 * * *',
    autoReboot: true,
    autoRebootCron: '0 5 * * *',
    backupEnabled: true,
    backupCron: '0 * * * *',
    backupRetention: 72,
  },
}

const players: Player[] = [
  {
    id: 'p-001',
    name: 'Demo Player',
    playerUid: 'demo-player-uid',
    platform: 'Steam',
    steamId: '76561198000000001',
    status: 'online',
    manageable: true,
  },
]

const logs: LogEntry[] = [
  {
    id: 'log-001',
    timestamp: '2026-07-09 09:05:50',
    level: 'info',
    source: 'rcon',
    message: 'RCON executed the command. ShowPlayers',
  },
  {
    id: 'log-002',
    timestamp: '2026-07-09 09:05:30',
    level: 'info',
    source: 'server',
    message: 'Running Palworld dedicated server on :8211',
  },
  {
    id: 'log-003',
    timestamp: '2026-07-09 09:05:25',
    level: 'info',
    source: 'backup',
    message: 'Cronjobs started. Automatic backups enabled.',
  },
  {
    id: 'log-004',
    timestamp: '2026-07-09 09:04:58',
    level: 'info',
    source: 'update',
    message: "Success! App '2394010' fully installed.",
  },
  {
    id: 'log-005',
    timestamp: '2026-07-09 09:04:01',
    level: 'warn',
    source: 'server',
    message: 'RCON is deprecated upstream; REST API should be preferred for new management features.',
  },
]

const backups: Backup[] = [
  {
    id: 'auto-20260709-0900',
    createdAt: '2026-07-09 09:00',
    size: '88 MB',
    type: 'automatic',
    status: 'ready',
    format: 'tar.gz',
    restorable: true,
    note: '首轮自动备份策略已启用，保留最近 72 份。',
  },
  {
    id: 'manual-before-tuning',
    createdAt: '建议创建',
    size: '-',
    type: 'manual',
    status: 'running',
    format: 'directory',
    restorable: false,
    note: '调整倍率或恢复存档前建议手动创建。',
  },
]

const settings: ServerSettings = {
  serverName: 'Palworld Dedicated Server',
  description: 'Managed by Palworld Ops',
  players: 32,
  serverPassword: '',
  adminPassword: '',
  community: false,
  restApiEnabled: false,
  rconEnabled: true,
  publicDomain: 'your-domain.example',
  publicIp: '',
  publicPort: '',
  expRate: 1,
  captureRate: 1,
  spawnRate: 1,
  collectionDropRate: 1,
  enemyDropRate: 1,
  eggHatchingHours: 72,
  autoSaveSpan: 30,
  deathPenalty: 'All',
  baseCampWorkerMax: 15,
  guildPlayerMax: 20,
  baseCampMaxInGuild: 4,
  gameParameters: { ...DEFAULT_GAME_PARAMETERS },
  crossplayPlatforms: ['Steam', 'Xbox', 'PS5', 'Mac'],
  autoPauseEnabled: false,
  playerLoggingEnabled: true,
  discordWebhookEnabled: false,
  targetManifestId: '',
}

const commandDefinitions: RconCommandDefinition[] = [
  {
    id: 'info',
    label: '查看服务器信息',
    command: 'Info',
    description: '显示服务器基础信息，用来确认 RCON 已连通。',
    risk: 'low',
    category: 'info',
  },
  {
    id: 'players',
    label: '查看在线玩家',
    command: 'ShowPlayers',
    description: '列出当前在线玩家、玩家 ID 和 SteamID。',
    risk: 'low',
    category: 'player',
  },
  {
    id: 'save',
    label: '立即保存世界',
    command: 'Save',
    description: '手动保存当前世界状态，备份或维护前建议先执行。',
    risk: 'low',
    category: 'world',
  },
  {
    id: 'kick',
    label: '踢出玩家',
    command: 'KickPlayer <SteamID>',
    description: '把指定玩家踢下线，需要把 <SteamID> 替换成真实值。',
    risk: 'medium',
    category: 'player',
  },
  {
    id: 'ban',
    label: '封禁玩家',
    command: 'BanPlayer <SteamID>',
    description: '封禁指定玩家，需要谨慎执行并记录原因。',
    risk: 'high',
    category: 'player',
  },
  {
    id: 'shutdown',
    label: '延迟关服',
    command: 'Shutdown 300 服务器将在5分钟后关闭',
    description: '倒计时关服并给玩家提示，适合维护前使用。',
    risk: 'high',
    category: 'shutdown',
  },
]

function delay<T>(data: T, ms = 260): Promise<T> {
  return new Promise((resolve) => window.setTimeout(() => resolve(data), ms))
}

export function getServerStatus(): Promise<ServerStatus> {
  if (!USE_MOCK_API) return apiRequest<ServerStatus>('/palworld/status')
  return delay(status)
}

export function getPlayers(): Promise<Player[]> {
  if (!USE_MOCK_API) return apiRequest<Player[]>('/palworld/players')
  return delay(players)
}

export function getAdvancedCapabilities(): Promise<AdvancedCapabilities> {
  return apiRequest<AdvancedCapabilities>('/palworld/capabilities')
}

export function getLivePlayers(): Promise<DataEnvelope<Player[]>> {
  return apiRequest<DataEnvelope<Player[]>>('/palworld/live/players')
}

export function getLiveMetrics(): Promise<LiveMetrics> {
  return apiRequest<LiveMetrics>('/palworld/live/metrics')
}

export function getLiveMap(): Promise<DataEnvelope<LiveMapData>> {
  return apiRequest<DataEnvelope<LiveMapData>>('/palworld/live/map')
}

export function getWorldStatus(): Promise<WorldStatus> {
  return apiRequest<WorldStatus>('/palworld/world/status')
}

export function getWorldPlayers(): Promise<DataEnvelope<WorldPlayer[]>> {
  return apiRequest<DataEnvelope<WorldPlayer[]>>('/palworld/world/players')
}

export function getWorldPlayer(uid: string): Promise<DataEnvelope<WorldPlayer>> {
  return apiRequest<DataEnvelope<WorldPlayer>>(`/palworld/world/players/${encodeURIComponent(uid)}`)
}

export function getWorldGuilds(): Promise<DataEnvelope<WorldGuild[]>> {
  return apiRequest<DataEnvelope<WorldGuild[]>>('/palworld/world/guilds')
}

export function refreshWorldSnapshot(): Promise<{ ok: boolean; message: string; snapshot: WorldSnapshot }> {
  return apiRequest('/palworld/world/refresh', { method: 'POST' })
}

export function getEditorStatus(): Promise<EditorStatus> {
  return apiRequest<EditorStatus>('/palworld/editor/status')
}

export function runEditorSession(action: 'start' | 'open' | 'stop'): Promise<EditorSessionResult> {
  return apiRequest<EditorSessionResult>('/palworld/editor/session', {
    method: 'POST',
    body: JSON.stringify({ action }),
  })
}

export function getLogs(): Promise<LogEntry[]> {
  if (!USE_MOCK_API) return apiRequest<LogEntry[]>('/palworld/logs')
  return delay(logs)
}

export function getBackups(): Promise<Backup[]> {
  if (!USE_MOCK_API) return apiRequest<Backup[]>('/palworld/backups')
  return delay(backups)
}

export function getSettings(): Promise<ServerSettings> {
  if (!USE_MOCK_API) return apiRequest<ServerSettings>('/palworld/settings')
  return delay(settings)
}

export function getRconCommands(): Promise<RconCommandDefinition[]> {
  if (!USE_MOCK_API) return apiRequest<RconCommandDefinition[]>('/palworld/rcon-commands')
  return delay(commandDefinitions)
}

export async function runRconCommand(command: string): Promise<RconCommandResult> {
  if (!USE_MOCK_API) {
    return apiRequest<RconCommandResult>('/palworld/rcon', {
      method: 'POST',
      body: JSON.stringify({ command }),
    })
  }

  return delay({
    command,
    executedAt: new Date().toLocaleString('zh-CN', { hour12: false }),
    output: command.toLowerCase().includes('showplayers')
      ? 'name,playeruid,steamid\nNo online players'
      : `模拟执行完成: ${command}`,
  })
}

export function announceMessage(message: string): Promise<AnnouncementResult> {
  if (!USE_MOCK_API) {
    return apiRequest<AnnouncementResult>('/palworld/announce', {
      method: 'POST',
      body: JSON.stringify({ message }),
    })
  }
  return delay({
    ok: true,
    message: '广播已发送',
    transport: 'Palworld REST API',
    sentAt: new Date().toLocaleString('zh-CN', { hour12: false }),
  })
}

export async function runMaintenanceAction(action: string): Promise<{ ok: boolean; message: string }> {
  if (!USE_MOCK_API) {
    return apiRequest<{ ok: boolean; message: string }>('/palworld/maintenance', {
      method: 'POST',
      body: JSON.stringify({ action }),
    })
  }

  return delay({ ok: true, message: `${action} 已加入执行队列` }, 420)
}

export async function saveMaintenancePolicy(policy: MaintenancePolicy): Promise<MaintenancePolicy> {
  if (!USE_MOCK_API) {
    return apiRequest<MaintenancePolicy>('/palworld/maintenance-policy', {
      method: 'PUT',
      body: JSON.stringify(policy),
    })
  }

  Object.assign(status.maintenance, policy)
  return delay({ ...status.maintenance }, 360)
}

export async function saveSettings(nextSettings: ServerSettings): Promise<ServerSettings> {
  if (!USE_MOCK_API) {
    return apiRequest<ServerSettings>('/palworld/settings', {
      method: 'PUT',
      body: JSON.stringify(nextSettings),
    })
  }

  Object.assign(settings, nextSettings)
  status.name = nextSettings.serverName
  status.playersMax = nextSettings.players
  status.address = nextSettings.publicDomain ? `${nextSettings.publicDomain}:${nextSettings.publicPort || '8211'}` : '未配置连接域名'
  return delay({ ...settings }, 360)
}
