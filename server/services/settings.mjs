import fs from 'node:fs/promises'
import path from 'node:path'
import crypto from 'node:crypto'

import { parseBool } from '../config/env.mjs'

export function createSettingsService({ config, repos }) {
  return {
    async read() {
      const latest = repos.settingsSnapshots.latest()
      return { ...envSettings(), ...(latest || {}) }
    },
    async save(next, actor) {
      await fs.mkdir(path.dirname(config.settingsFile), { recursive: true })
      await fs.writeFile(config.settingsFile, `${JSON.stringify(next, null, 2)}\n`)
      if (config.writeEnv) await updateEnvFile(config.envFile, next)
      repos.settingsSnapshots.insert({
        id: crypto.randomUUID(),
        payload: next,
        actor,
        createdAt: new Date().toISOString(),
      })
      return next
    },
  }
}

function envSettings() {
  return {
    serverName: process.env.PALWORLD_SERVER_NAME || process.env.SERVER_NAME || 'Palworld Dedicated Server',
    description: process.env.PALWORLD_SERVER_DESCRIPTION || process.env.SERVER_DESCRIPTION || 'Managed by Palworld Ops',
    players: Number(process.env.PALWORLD_PLAYERS || process.env.PLAYERS || 32),
    serverPassword: process.env.PALWORLD_SERVER_PASSWORD || process.env.SERVER_PASSWORD || '',
    adminPassword: process.env.PALWORLD_ADMIN_PASSWORD || process.env.ADMIN_PASSWORD || '',
    community: parseBool(process.env.PALWORLD_COMMUNITY || process.env.COMMUNITY, false),
    restApiEnabled: parseBool(process.env.PALWORLD_REST_API_ENABLED || process.env.REST_API_ENABLED, false),
    rconEnabled: parseBool(process.env.PALWORLD_RCON_ENABLED || process.env.RCON_ENABLED, true),
    publicIp: process.env.PALWORLD_PUBLIC_IP || process.env.PUBLIC_IP || '',
    publicPort: process.env.PALWORLD_PUBLIC_PORT || process.env.PUBLIC_PORT || process.env.PALWORLD_PORT || '8211',
    expRate: Number(process.env.PALWORLD_EXP_RATE || process.env.EXP_RATE || 1),
    captureRate: Number(process.env.PALWORLD_CAPTURE_RATE || process.env.CAPTURE_RATE || 1),
    spawnRate: Number(process.env.PALWORLD_SPAWN_RATE || process.env.SPAWN_RATE || 1),
    collectionDropRate: Number(process.env.PALWORLD_COLLECTION_DROP_RATE || process.env.COLLECTION_DROP_RATE || 1),
    enemyDropRate: Number(process.env.PALWORLD_ENEMY_DROP_RATE || process.env.ENEMY_DROP_RATE || 1),
    eggHatchingHours: Number(process.env.PALWORLD_EGG_HATCHING_HOURS || process.env.EGG_HATCHING_HOURS || 72),
    autoSaveSpan: Number(process.env.PALWORLD_AUTO_SAVE_SPAN || process.env.AUTO_SAVE_SPAN || 30),
    deathPenalty: process.env.PALWORLD_DEATH_PENALTY || process.env.DEATH_PENALTY || 'All',
    baseCampWorkerMax: Number(process.env.PALWORLD_BASE_CAMP_WORKER_MAX || process.env.BASE_CAMP_WORKER_MAX || 15),
    guildPlayerMax: Number(process.env.PALWORLD_GUILD_PLAYER_MAX || process.env.GUILD_PLAYER_MAX || 20),
    baseCampMaxInGuild: Number(process.env.PALWORLD_BASE_CAMP_MAX_IN_GUILD || process.env.BASE_CAMP_MAX_IN_GUILD || 4),
    crossplayPlatforms: String(process.env.PALWORLD_CROSSPLAY_PLATFORMS || 'Steam,Xbox,PS5,Mac').split(',').map((item) => item.trim()).filter(Boolean),
    autoPauseEnabled: parseBool(process.env.PALWORLD_AUTO_PAUSE_ENABLED || process.env.AUTO_PAUSE_ENABLED, false),
    playerLoggingEnabled: parseBool(process.env.PALWORLD_PLAYER_LOGGING_ENABLED || process.env.ENABLE_PLAYER_LOGGING, true),
    discordWebhookEnabled: parseBool(process.env.PALWORLD_DISCORD_WEBHOOK_ENABLED || process.env.DISCORD_WEBHOOK_ENABLED, false),
    targetManifestId: process.env.PALWORLD_TARGET_MANIFEST_ID || process.env.TARGET_MANIFEST_ID || '',
  }
}

async function updateEnvFile(envFile, settings) {
  const updates = settingsToEnv(settings)
  let text = ''
  try {
    text = await fs.readFile(envFile, 'utf8')
  } catch {}

  const seen = new Set()
  const lines = text.split(/\r?\n/).map((line) => {
    const match = line.match(/^([A-Z0-9_]+)=/)
    if (!match || !Object.prototype.hasOwnProperty.call(updates, match[1])) return line
    seen.add(match[1])
    return `${match[1]}=${formatEnvValue(updates[match[1]])}`
  })

  for (const [key, value] of Object.entries(updates)) {
    if (!seen.has(key)) lines.push(`${key}=${formatEnvValue(value)}`)
  }

  await fs.mkdir(path.dirname(envFile), { recursive: true })
  await fs.writeFile(envFile, `${lines.filter((line, index, list) => line || index < list.length - 1).join('\n')}\n`)
}

function settingsToEnv(settings) {
  return {
    PALWORLD_SERVER_NAME: settings.serverName,
    PALWORLD_SERVER_DESCRIPTION: settings.description,
    PALWORLD_PLAYERS: settings.players,
    PALWORLD_SERVER_PASSWORD: settings.serverPassword,
    PALWORLD_ADMIN_PASSWORD: settings.adminPassword,
    PALWORLD_COMMUNITY: settings.community,
    PALWORLD_RCON_ENABLED: settings.rconEnabled,
    PALWORLD_REST_API_ENABLED: settings.restApiEnabled,
    PALWORLD_PUBLIC_IP: settings.publicIp,
    PALWORLD_PUBLIC_PORT: settings.publicPort,
    PALWORLD_EXP_RATE: settings.expRate,
    PALWORLD_CAPTURE_RATE: settings.captureRate,
    PALWORLD_SPAWN_RATE: settings.spawnRate,
    PALWORLD_COLLECTION_DROP_RATE: settings.collectionDropRate,
    PALWORLD_ENEMY_DROP_RATE: settings.enemyDropRate,
    PALWORLD_EGG_HATCHING_HOURS: settings.eggHatchingHours,
    PALWORLD_AUTO_SAVE_SPAN: settings.autoSaveSpan,
    PALWORLD_DEATH_PENALTY: settings.deathPenalty,
    PALWORLD_BASE_CAMP_WORKER_MAX: settings.baseCampWorkerMax,
    PALWORLD_GUILD_PLAYER_MAX: settings.guildPlayerMax,
    PALWORLD_BASE_CAMP_MAX_IN_GUILD: settings.baseCampMaxInGuild,
    PALWORLD_CROSSPLAY_PLATFORMS: settings.crossplayPlatforms.join(','),
    PALWORLD_AUTO_PAUSE_ENABLED: settings.autoPauseEnabled,
    PALWORLD_PLAYER_LOGGING_ENABLED: settings.playerLoggingEnabled,
    PALWORLD_DISCORD_WEBHOOK_ENABLED: settings.discordWebhookEnabled,
    PALWORLD_TARGET_MANIFEST_ID: settings.targetManifestId,
  }
}

function formatEnvValue(value) {
  const text = Array.isArray(value) ? value.join(',') : String(value ?? '')
  if (!text || /[\s#"']/u.test(text)) return JSON.stringify(text)
  return text
}
