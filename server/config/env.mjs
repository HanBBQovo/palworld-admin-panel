import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
export const projectRoot = path.resolve(__dirname, '../..')

loadDotEnv(path.join(projectRoot, '.env'))

const stateDir = process.env.PANEL_STATE_DIR || path.join(projectRoot, '.panel-state')

export const config = {
  bind: process.env.PANEL_API_BIND || process.env.HOST || '0.0.0.0',
  port: Number(process.env.PANEL_API_PORT || process.env.APP_API_PORT || process.env.PORT || 16824),
  authPassword: process.env.PANEL_AUTH_PASSWORD || 'change-panel-password',
  jwtSecret: process.env.PANEL_JWT_SECRET || process.env.PANEL_AUTH_PASSWORD || 'dev-secret-change-me',
  tokenTtlSeconds: Number(process.env.PANEL_TOKEN_TTL_SECONDS || 60 * 60 * 24 * 7),
  corsOrigin: process.env.PANEL_CORS_ORIGIN || true,
  webRoot: process.env.PANEL_WEB_ROOT || path.join(projectRoot, 'dist'),
  stateDir,
  dbFile: process.env.PANEL_DB_FILE || path.join(stateDir, 'panel.sqlite'),
  settingsFile: process.env.PANEL_SETTINGS_FILE || path.join(stateDir, 'settings.json'),
  dataDir: process.env.PALWORLD_DATA_DIR || '/palworld',
  savesDir: process.env.PALWORLD_SAVES_DIR || process.env.PALWORLD_SAVE_DIR || '/palworld/Pal/Saved/SaveGames',
  backupsDir: process.env.PALWORLD_BACKUP_DIR || '/palworld/backups',
  composeDir: process.env.PALWORLD_COMPOSE_DIR || projectRoot,
  envFile: process.env.PANEL_ENV_FILE || path.join(process.env.PALWORLD_COMPOSE_DIR || projectRoot, '.env'),
  container: process.env.PALWORLD_CONTAINER || 'palworld-server',
  rconHost: process.env.PALWORLD_RCON_HOST || '127.0.0.1',
  rconPort: Number(process.env.PALWORLD_RCON_PORT || process.env.RCON_PORT || 25575),
  rconPassword: process.env.PALWORLD_ADMIN_PASSWORD || process.env.ADMIN_PASSWORD || '',
  allowRawRcon: process.env.PANEL_ALLOW_RAW_RCON === 'true',
  writeEnv: process.env.PANEL_WRITE_ENV !== 'false',
}

export function loadDotEnv(filePath) {
  try {
    const raw = fs.readFileSync(filePath, 'utf8')
    for (const line of raw.split(/\r?\n/)) {
      const trimmed = line.trim()
      if (!trimmed || trimmed.startsWith('#') || !trimmed.includes('=')) continue
      const index = trimmed.indexOf('=')
      const key = trimmed.slice(0, index).trim()
      const value = parseEnvValue(trimmed.slice(index + 1).trim())
      if (key && process.env[key] === undefined) process.env[key] = value
    }
  } catch {}
}

function parseEnvValue(value) {
  if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) {
    return value.slice(1, -1).replace(/\\n/g, '\n').replace(/\\"/g, '"')
  }
  return value
}

export function parseBool(value, fallback) {
  if (value === undefined || value === '') return fallback
  return ['1', 'true', 'yes', 'on'].includes(String(value).toLowerCase())
}
