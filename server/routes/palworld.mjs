export async function palworldRoutes(app) {
  app.addHook('preHandler', app.authenticate)

  app.get('/palworld/status', () => app.services.palworld.status())
  app.get('/palworld/players', () => app.services.palworld.players())
  app.get('/palworld/logs', () => app.services.palworld.logs())
  app.get('/palworld/backups', () => app.services.palworld.backups())
  app.post('/palworld/backups', (request) => app.services.palworld.createBackup(request.user?.username))
  app.post('/palworld/backups/:id/restore', (request) => app.services.palworld.restoreBackup(request.params.id, request.user?.username))
  app.get('/palworld/settings', () => app.services.settings.read())
  app.put('/palworld/settings', async (request) => {
    const result = await app.services.settings.save(request.body, request.user?.username)
    app.services.audit.write('server', 'info', '面板配置已保存；部分 Palworld 环境变量需要重启容器后生效。', {}, request.user?.username)
    return result
  })
  app.get('/palworld/rcon-commands', () => app.services.rcon.definitions())
  app.post('/palworld/rcon', async (request) => {
    const result = await app.services.rcon.execute(request.body?.command)
    const command = result.command.toLowerCase()
    app.services.audit.write('rcon', command.startsWith('shutdown') || command.startsWith('banplayer') ? 'warn' : 'info', `RCON executed: ${result.command}`, {}, request.user?.username)
    return result
  })
  app.post('/palworld/maintenance', (request) => app.services.palworld.maintenance(String(request.body?.action || ''), request.user?.username))
}
