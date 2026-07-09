import Fastify from 'fastify'
import cors from '@fastify/cors'
import fastifyJwt from '@fastify/jwt'
import fastifyStatic from '@fastify/static'
import fs from 'node:fs'

import { authRoutes } from './routes/auth.mjs'
import { palworldRoutes } from './routes/palworld.mjs'
import { createAuditService } from './services/audit.mjs'
import { createAuthService } from './services/auth.mjs'
import { createPalworldService } from './services/palworld.mjs'
import { createRconService } from './services/rcon.mjs'
import { createSettingsService } from './services/settings.mjs'

export async function buildApp({ config, repos }) {
  const app = Fastify({
    logger: true,
  })

  await app.register(cors, {
    origin: config.corsOrigin,
    credentials: false,
  })
  await app.register(fastifyJwt, {
    secret: config.jwtSecret,
  })

  const rcon = createRconService(config)
  const settings = createSettingsService({ config, repos })
  const audit = createAuditService({ repos })
  app.decorate('services', {
    rcon,
    settings,
    audit,
    auth: null,
    palworld: null,
  })
  app.services.auth = createAuthService({ config, repos, app })
  app.services.palworld = createPalworldService({ config, repos, rcon, settings, audit })
  app.decorate('authenticate', async (request, reply) => {
    try {
      await app.services.auth.verify(request)
    } catch (err) {
      reply.code(err.statusCode || 401)
      throw err
    }
  })

  app.setErrorHandler((err, request, reply) => {
    const status = err.statusCode || 500
    request.log[status >= 500 ? 'error' : 'warn']({ err }, err.message)
    reply.code(status).send({ error: err.message || 'Server error' })
  })

  app.get('/api/health', async () => ({ ok: true }))
  await app.register(authRoutes, { prefix: '/api' })
  await app.register(palworldRoutes, { prefix: '/api' })

  if (fs.existsSync(config.webRoot)) {
    await app.register(fastifyStatic, {
      root: config.webRoot,
      prefix: '/',
    })
    app.setNotFoundHandler((request, reply) => {
      if (request.raw.url?.startsWith('/api/')) {
        reply.code(404).send({ error: '接口不存在' })
        return
      }
      reply.sendFile('index.html')
    })
  }

  app.addHook('onClose', async () => {
    repos.db?.close?.()
  })

  return app
}
