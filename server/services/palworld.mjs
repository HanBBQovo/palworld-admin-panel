import crypto from 'node:crypto'
import fs from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'

import { parseBool } from '../config/env.mjs'
import { docker, safeDocker, run } from '../lib/system.mjs'

export function createPalworldService({ config, repos, rcon, settings, audit }) {
  return {
    async status() {
      const currentSettings = await settings.read()
      const inspect = await dockerInspect(config)
      const stats = await dockerStats(config)
      const startedAt = inspect?.State?.StartedAt
      const running = inspect?.State?.Running
      const players = await this.players().catch(() => [])
      const memoryParts = String(stats?.MemUsage || '').split('/')
      const worldSize = await pathSizeGb(config.savesDir)
      const dataSize = await pathSizeGb(config.dataDir)
      const diskTotal = await diskTotalGb(config.dataDir)

      return {
        name: currentSettings.serverName,
        host: os.hostname(),
        address: `${process.env.PALWORLD_PUBLIC_DOMAIN || currentSettings.publicIp || 'your-domain.example'}:${currentSettings.publicPort || 8211}`,
        version: await getVersion(rcon),
        container: config.container,
        image: inspect?.Config?.Image || 'thijsvanloef/palworld-server-docker:latest',
        health: running ? 'healthy' : inspect ? 'offline' : 'warning',
        startedAt: formatDate(startedAt),
        uptime: formatUptime(startedAt),
        playersOnline: players.length,
        playersMax: currentSettings.players,
        cpu: parseCpu(stats?.CPUPerc),
        memoryUsedGb: Number(parseMemoryGb(memoryParts[0]).toFixed(1)),
        memoryLimitGb: Number(parseMemoryGb(memoryParts[1]).toFixed(1)) || Number((os.totalmem() / 1024 / 1024 / 1024).toFixed(1)),
        diskUsedGb: dataSize,
        diskTotalGb: diskTotal,
        worldSizeGb: worldSize,
        lastSaveAt: await lastModified(config.savesDir),
        nextBackupAt: process.env.PALWORLD_BACKUP_CRON || process.env.BACKUP_CRON_EXPRESSION || '按容器配置',
        nextRestartAt: process.env.PALWORLD_AUTO_REBOOT_CRON || process.env.AUTO_REBOOT_CRON_EXPRESSION || '按容器配置',
        ports: [
          { port: Number(process.env.PALWORLD_PORT || 8211), protocol: 'UDP', exposure: 'public', purpose: '游戏连接端口', safe: true },
          { port: Number(process.env.PALWORLD_QUERY_PORT || 27015), protocol: 'UDP', exposure: 'public', purpose: 'Steam 查询端口', safe: true },
          { port: config.rconPort, protocol: 'TCP', exposure: 'local', purpose: 'RCON 管理端口', safe: true },
          { port: Number(process.env.PALWORLD_REST_PORT || 8212), protocol: 'TCP', exposure: 'local', purpose: 'REST API 管理端口', safe: true },
        ],
        maintenance: maintenancePolicy(),
      }
    },
    async players() {
      const result = await rcon.execute('ShowPlayers')
      const lines = result.output.split(/\r?\n/).map((line) => line.trim()).filter(Boolean)
      const rows = lines.filter((line) => !line.toLowerCase().startsWith('name,'))
      return rows.map((line, index) => {
        const [name, playerUid, steamId] = line.split(',').map((item) => item?.trim() || '-')
        return {
          id: playerUid || steamId || `player-${index + 1}`,
          name: name || `Player ${index + 1}`,
          platform: 'Steam',
          steamId: steamId || '-',
          level: 0,
          guild: '-',
          location: '-',
          onlineFor: '-',
          ping: 0,
          status: 'online',
        }
      })
    },
    async logs() {
      const auditRows = audit.list(40)
      const dockerRows = []
      const result = await safeDocker(config, ['logs', '--tail', '80', config.container])
      if (result?.stdout) {
        for (const [index, line] of result.stdout.split('\n').filter(Boolean).entries()) {
          dockerRows.push({
            id: `docker-${index}`,
            timestamp: formatDate(new Date()),
            level: line.toLowerCase().includes('error') ? 'error' : line.toLowerCase().includes('warn') ? 'warn' : 'info',
            source: line.toLowerCase().includes('backup') ? 'backup' : line.toLowerCase().includes('steamcmd') || line.toLowerCase().includes('update') ? 'update' : 'server',
            message: line,
          })
        }
      }
      return [...auditRows, ...dockerRows].slice(-100).reverse()
    },
    async backups() {
      await syncBackups(config, repos)
      return repos.backupRecords.list(200).map((row) => ({
        id: row.id,
        createdAt: formatDate(row.created_at),
        size: `${Math.max(1, Math.round(row.size_bytes / 1024 / 1024))} MB`,
        type: row.type,
        status: row.status,
        note: row.note || '',
      }))
    },
    async createBackup(actor) {
      const operation = startOperation(repos, 'backup:create', actor)
      try {
        await fs.mkdir(config.backupsDir, { recursive: true })
        const id = `manual-${new Date().toISOString().replace(/[-:]/g, '').replace(/\..+/, '').replace('T', '-')}`
        const target = path.join(config.backupsDir, id)
        await rcon.execute('Save').catch(() => null)
        await fs.cp(config.savesDir, target, { recursive: true })
        const stat = await fs.stat(target)
        repos.backupRecords.upsert({
          id,
          path: target,
          type: 'manual',
          status: 'ready',
          sizeBytes: stat.size,
          note: '面板创建的手动备份。',
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        })
        repos.operations.finish(operation.id, 'success', `已创建手动备份 ${id}`)
        audit.write('backup', 'info', `已创建手动备份 ${id}`, { backupId: id }, actor)
        return { ok: true, message: `已创建手动备份 ${id}` }
      } catch (err) {
        repos.operations.finish(operation.id, 'failed', err.message)
        throw err
      }
    },
    async restoreBackup(id, actor) {
      const operation = startOperation(repos, 'backup:restore', actor, { backupId: id })
      try {
        const backupId = safeBackupId(id)
        const source = path.join(config.backupsDir, backupId)
        const stat = await fs.stat(source)
        if (!stat.isDirectory()) throw new Error('当前只支持恢复目录备份')
        await fs.mkdir(config.backupsDir, { recursive: true })
        const currentCopy = path.join(config.backupsDir, `pre-restore-${Date.now()}`)
        await fs.cp(config.savesDir, currentCopy, { recursive: true }).catch(() => null)
        await fs.rm(config.savesDir, { recursive: true, force: true })
        await fs.cp(source, config.savesDir, { recursive: true })
        repos.operations.finish(operation.id, 'success', `已恢复 ${backupId}`)
        audit.write('backup', 'warn', `已恢复备份 ${backupId}；重启游戏容器后生效。`, { backupId }, actor)
        return { ok: true, message: `已恢复 ${backupId}，建议立即重启游戏容器` }
      } catch (err) {
        repos.operations.finish(operation.id, 'failed', err.message)
        throw err
      }
    },
    async maintenance(action, actor) {
      const operation = startOperation(repos, action, actor)
      try {
        let result
        if (action === 'rcon:save') {
          const output = await rcon.execute('Save')
          result = { ok: true, message: output.output }
          audit.write('rcon', 'info', '已执行 Save', {}, actor)
        } else if (action === 'server:shutdown') {
          const output = await rcon.execute('Shutdown 300 服务器将在5分钟后关闭')
          result = { ok: true, message: output.output }
          audit.write('rcon', 'warn', '已提交延迟关服命令', {}, actor)
        } else if (action === 'server:restart') {
          await docker(config, ['restart', config.container], { timeout: 60000 })
          result = { ok: true, message: '容器重启命令已执行' }
          audit.write('server', 'warn', `已重启容器 ${config.container}`, {}, actor)
        } else if (action === 'server:update') {
          await docker(config, ['compose', 'pull', 'palworld'], { timeout: 180000 }).catch(async () => {
            const inspect = await dockerInspect(config)
            if (inspect?.Config?.Image) await docker(config, ['pull', inspect.Config.Image], { timeout: 180000 })
          })
          await docker(config, ['compose', 'up', '-d', 'palworld'], { timeout: 180000 }).catch(() => docker(config, ['restart', config.container], { timeout: 60000 }))
          result = { ok: true, message: '更新流程已执行' }
          audit.write('update', 'warn', '已执行服务端更新流程', {}, actor)
        } else if (action === 'backup:create') {
          result = await this.createBackup(actor)
        } else if (action.startsWith('backup:restore:')) {
          result = await this.restoreBackup(action.replace('backup:restore:', ''), actor)
        } else {
          throw new Error(`未知维护动作: ${action}`)
        }
        repos.operations.finish(operation.id, 'success', result.message)
        return result
      } catch (err) {
        repos.operations.finish(operation.id, 'failed', err.message)
        throw err
      }
    },
  }
}

function startOperation(repos, action, actor, metadata = {}) {
  const row = {
    id: crypto.randomUUID(),
    action,
    status: 'running',
    actor,
    metadata,
    createdAt: new Date().toISOString(),
  }
  repos.operations.start(row)
  return row
}

function maintenancePolicy() {
  return {
    updateOnBoot: parseBool(process.env.PALWORLD_UPDATE_ON_BOOT || process.env.UPDATE_ON_BOOT, true),
    autoUpdate: parseBool(process.env.PALWORLD_AUTO_UPDATE_ENABLED || process.env.AUTO_UPDATE_ENABLED, true),
    autoUpdateCron: process.env.PALWORLD_AUTO_UPDATE_CRON || process.env.AUTO_UPDATE_CRON_EXPRESSION || '0 4 * * *',
    autoReboot: parseBool(process.env.PALWORLD_AUTO_REBOOT_ENABLED || process.env.AUTO_REBOOT_ENABLED, true),
    autoRebootCron: process.env.PALWORLD_AUTO_REBOOT_CRON || process.env.AUTO_REBOOT_CRON_EXPRESSION || '0 5 * * *',
    backupEnabled: parseBool(process.env.PALWORLD_BACKUP_ENABLED || process.env.BACKUP_ENABLED, true),
    backupCron: process.env.PALWORLD_BACKUP_CRON || process.env.BACKUP_CRON_EXPRESSION || '0 * * * *',
    backupRetention: Number(process.env.PALWORLD_BACKUP_RETENTION || process.env.BACKUP_RETENTION_AMOUNT_TO_KEEP || 72),
  }
}

async function syncBackups(config, repos) {
  try {
    const entries = await fs.readdir(config.backupsDir, { withFileTypes: true })
    for (const entry of entries) {
      const fullPath = path.join(config.backupsDir, entry.name)
      const stat = await fs.stat(fullPath)
      repos.backupRecords.upsert({
        id: entry.name,
        path: fullPath,
        type: entry.name.toLowerCase().includes('manual') ? 'manual' : 'automatic',
        status: entry.isDirectory() || entry.isFile() ? 'ready' : 'failed',
        sizeBytes: stat.size,
        note: entry.isDirectory() ? '目录备份，可直接恢复。' : '文件备份，恢复前需要手动解包。',
        createdAt: stat.mtime.toISOString(),
        updatedAt: new Date().toISOString(),
      })
    }
  } catch {}
}

async function dockerInspect(config) {
  const result = await safeDocker(config, ['inspect', config.container])
  if (!result?.stdout) return null
  try {
    return JSON.parse(result.stdout)[0] || null
  } catch {
    return null
  }
}

async function dockerStats(config) {
  const result = await safeDocker(config, ['stats', '--no-stream', '--format', '{{json .}}', config.container], null)
  if (!result?.stdout) return null
  try {
    return JSON.parse(result.stdout)
  } catch {
    return null
  }
}

async function getVersion(rcon) {
  try {
    const output = await rcon.execute('Info')
    const match = output.output.match(/v[\d.]+[^\s,)]*/i)
    return match?.[0] || output.output.split('\n')[0] || 'unknown'
  } catch {
    return 'unknown'
  }
}

async function pathSizeGb(target) {
  try {
    const { stdout } = await run('du', ['-sk', target], { timeout: 20000 })
    return Number((Number(stdout.split(/\s+/)[0] || 0) / 1024 / 1024).toFixed(2))
  } catch {
    return 0
  }
}

async function diskTotalGb(target) {
  try {
    const { stdout } = await run('df', ['-k', target], { timeout: 10000 })
    const [, row] = stdout.split('\n')
    const parts = row.trim().split(/\s+/)
    return Number((Number(parts[1] || 0) / 1024 / 1024).toFixed(1))
  } catch {
    return 0
  }
}

async function lastModified(target) {
  try {
    const stat = await fs.stat(target)
    return formatDate(stat.mtime)
  } catch {
    return '-'
  }
}

function safeBackupId(id) {
  const value = String(id || '')
  if (!/^[a-zA-Z0-9._-]+$/.test(value)) throw new Error('非法备份 ID')
  return value
}

function formatDate(value) {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

function formatUptime(startedAt) {
  if (!startedAt) return '-'
  const seconds = Math.max(0, Math.floor((Date.now() - new Date(startedAt).getTime()) / 1000))
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  return `${days ? `${days} 天 ` : ''}${hours} 小时 ${minutes} 分`
}

function parseMemoryGb(value) {
  const match = String(value || '').match(/([\d.]+)\s*([KMGTP]?i?B)/i)
  if (!match) return 0
  const amount = Number(match[1])
  const unit = match[2].toLowerCase()
  if (unit.startsWith('ki') || unit === 'kb') return amount / 1024 / 1024
  if (unit.startsWith('mi') || unit === 'mb') return amount / 1024
  if (unit.startsWith('gi') || unit === 'gb') return amount
  if (unit.startsWith('ti') || unit === 'tb') return amount * 1024
  return amount
}

function parseCpu(value) {
  return Number(String(value || '0').replace('%', '')) || 0
}
