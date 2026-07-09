export async function authRoutes(app) {
  app.post('/auth/login', async (request) => {
    return app.services.auth.login(request.body?.password)
  })

  app.post('/auth/logout', { preHandler: app.authenticate }, async (request) => {
    return app.services.auth.logout(request.user?.sessionId)
  })

  app.get('/auth/status', async (request) => {
    try {
      await app.services.auth.verify(request)
      return { authenticated: true }
    } catch {
      return { authenticated: false }
    }
  })
}
