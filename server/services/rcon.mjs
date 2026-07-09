import net from 'node:net'

export const commandDefinitions = [
  { id: 'info', label: '查看服务器信息', command: 'Info', description: '显示服务器基础信息，用来确认 RCON 已连通。', risk: 'low', category: 'info' },
  { id: 'players', label: '查看在线玩家', command: 'ShowPlayers', description: '列出当前在线玩家、玩家 ID 和 SteamID。', risk: 'low', category: 'player' },
  { id: 'save', label: '立即保存世界', command: 'Save', description: '手动保存当前世界状态，备份或维护前建议先执行。', risk: 'low', category: 'world' },
  { id: 'broadcast', label: '广播消息', command: 'Broadcast 服务器将在5分钟后维护', description: '向所有在线玩家发送一条公告。', risk: 'low', category: 'broadcast' },
  { id: 'kick', label: '踢出玩家', command: 'KickPlayer <SteamID>', description: '把指定玩家踢下线，需要把 <SteamID> 替换成真实值。', risk: 'medium', category: 'player' },
  { id: 'ban', label: '封禁玩家', command: 'BanPlayer <SteamID>', description: '封禁指定玩家，需要谨慎执行并记录原因。', risk: 'high', category: 'player' },
  { id: 'shutdown', label: '延迟关服', command: 'Shutdown 300 服务器将在5分钟后关闭', description: '倒计时关服并给玩家提示，适合维护前使用。', risk: 'high', category: 'shutdown' },
]

const allowedRconPrefixes = ['Info', 'ShowPlayers', 'Save', 'Broadcast', 'KickPlayer', 'BanPlayer', 'Shutdown']

export function createRconService(config) {
  return {
    definitions() {
      return commandDefinitions
    },
    async execute(command) {
      const trimmed = String(command || '').trim()
      if (!trimmed) throw badRequest('RCON 命令不能为空')
      assertAllowed(config, trimmed)
      const output = await runRcon(config, trimmed)
      return {
        command: trimmed,
        output,
        executedAt: new Date().toLocaleString('zh-CN', { hour12: false }),
      }
    },
  }
}

function assertAllowed(config, command) {
  if (config.allowRawRcon) return
  const normalized = command.toLowerCase()
  const allowed = allowedRconPrefixes.some((prefix) => normalized === prefix.toLowerCase() || normalized.startsWith(`${prefix.toLowerCase()} `))
  if (!allowed) throw badRequest('该 RCON 命令不在白名单内；如需开放任意命令，请设置 PANEL_ALLOW_RAW_RCON=true')
}

function runRcon(config, command) {
  if (!config.rconPassword) throw new Error('PALWORLD_ADMIN_PASSWORD 未配置，无法连接 RCON')

  return new Promise((resolve, reject) => {
    const socket = net.createConnection({ host: config.rconHost, port: config.rconPort })
    let buffer = Buffer.alloc(0)
    let authed = false
    let output = ''
    let finishTimer
    const timeout = setTimeout(() => {
      socket.destroy()
      reject(new Error('RCON 连接超时'))
    }, 6000)

    const finish = () => {
      clearTimeout(timeout)
      clearTimeout(finishTimer)
      socket.end()
      resolve(output.trim() || 'OK')
    }

    socket.on('connect', () => socket.write(packet(1, 3, config.rconPassword)))
    socket.on('data', (chunk) => {
      buffer = Buffer.concat([buffer, chunk])
      const parsed = parsePackets(buffer)
      buffer = parsed.rest
      for (const item of parsed.packets) {
        if (!authed && item.id === -1) {
          clearTimeout(timeout)
          socket.destroy()
          reject(new Error('RCON 鉴权失败，请检查管理员密码'))
          return
        }
        if (!authed && item.id === 1) {
          authed = true
          socket.write(packet(2, 2, command))
          socket.write(packet(3, 2, ''))
          continue
        }
        if (authed && item.id === 2) output += item.body
        if (authed && item.id === 3) finishTimer = setTimeout(finish, 120)
      }
    })
    socket.on('error', (err) => {
      clearTimeout(timeout)
      clearTimeout(finishTimer)
      reject(err)
    })
  })
}

function packet(id, type, body = '') {
  const bodyBuffer = Buffer.from(body, 'utf8')
  const size = 4 + 4 + bodyBuffer.length + 2
  const buffer = Buffer.alloc(size + 4)
  buffer.writeInt32LE(size, 0)
  buffer.writeInt32LE(id, 4)
  buffer.writeInt32LE(type, 8)
  bodyBuffer.copy(buffer, 12)
  return buffer
}

function parsePackets(buffer) {
  const packets = []
  let offset = 0
  while (buffer.length - offset >= 4) {
    const size = buffer.readInt32LE(offset)
    if (buffer.length - offset < size + 4) break
    const start = offset + 4
    packets.push({
      id: buffer.readInt32LE(start),
      type: buffer.readInt32LE(start + 4),
      body: buffer.subarray(start + 8, offset + size + 2).toString('utf8').replace(/\0+$/, ''),
    })
    offset += size + 4
  }
  return { packets, rest: buffer.subarray(offset) }
}

function badRequest(message) {
  const err = new Error(message)
  err.statusCode = 400
  return err
}
