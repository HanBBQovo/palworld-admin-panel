import crypto from 'node:crypto'

const USERNAME = 'admin'

export function createAuthService({ config, repos, app }) {
  ensureAdminUser(config, repos)

  return {
    async login(password) {
      const user = repos.users.findByUsername(USERNAME)
      if (!user || !verifyPassword(password, user.password_hash)) {
        throw unauthorized('面板密码错误')
      }

      const now = Math.floor(Date.now() / 1000)
      const session = {
        id: crypto.randomUUID(),
        userId: user.id,
        issuedAt: new Date(now * 1000).toISOString(),
        expiresAt: new Date((now + config.tokenTtlSeconds) * 1000).toISOString(),
      }
      repos.sessions.create(session)
      const token = app.jwt.sign({ sid: session.id, sub: user.id, username: user.username }, { expiresIn: config.tokenTtlSeconds })
      return { token }
    },
    async logout(sessionId) {
      if (sessionId) repos.sessions.revoke(sessionId)
      return { ok: true }
    },
    async verify(request) {
      try {
        const payload = await request.jwtVerify()
        const session = repos.sessions.findActive(payload.sid)
        if (!session) throw unauthorized('未登录或登录已过期')
        request.user = { id: payload.sub, username: payload.username, sessionId: payload.sid }
        return request.user
      } catch {
        throw unauthorized('未登录或登录已过期')
      }
    },
  }
}

function ensureAdminUser(config, repos) {
  repos.users.upsert({
    id: 'admin',
    username: USERNAME,
    passwordHash: hashPassword(config.authPassword),
  })
}

function hashPassword(password) {
  const salt = crypto.createHash('sha256').update('palworld-ops-panel').digest('hex').slice(0, 16)
  const hash = crypto.scryptSync(String(password), salt, 64).toString('hex')
  return `scrypt:${salt}:${hash}`
}

function verifyPassword(password, stored) {
  const [, salt, hash] = String(stored || '').split(':')
  if (!salt || !hash) return false
  const next = crypto.scryptSync(String(password), salt, 64)
  const expected = Buffer.from(hash, 'hex')
  return next.length === expected.length && crypto.timingSafeEqual(next, expected)
}

function unauthorized(message) {
  const err = new Error(message)
  err.statusCode = 401
  return err
}
