import crypto from 'node:crypto'

export function createAuditService({ repos }) {
  return {
    write(source, level, message, metadata = {}, actor = null) {
      const row = {
        id: crypto.randomUUID(),
        timestamp: new Date().toISOString(),
        level,
        source,
        message,
        actor,
        metadata,
      }
      repos.auditLogs.insert(row)
      return row
    },
    list(limit = 80) {
      return repos.auditLogs.list(limit)
    },
  }
}
