import fs from 'node:fs'
import path from 'node:path'
import { DatabaseSync } from 'node:sqlite'

export function openDatabase(config) {
  fs.mkdirSync(path.dirname(config.dbFile), { recursive: true })
  const db = new DatabaseSync(config.dbFile)
  db.exec('PRAGMA journal_mode = WAL;')
  db.exec('PRAGMA foreign_keys = ON;')
  db.exec(`
    CREATE TABLE IF NOT EXISTS panel_meta (
      key TEXT PRIMARY KEY,
      value TEXT NOT NULL,
      updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
    );

    CREATE TABLE IF NOT EXISTS users (
      id TEXT PRIMARY KEY,
      username TEXT NOT NULL UNIQUE,
      password_hash TEXT NOT NULL,
      created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
      updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
    );

    CREATE TABLE IF NOT EXISTS sessions (
      id TEXT PRIMARY KEY,
      user_id TEXT NOT NULL,
      issued_at TEXT NOT NULL,
      expires_at TEXT NOT NULL,
      revoked_at TEXT,
      FOREIGN KEY (user_id) REFERENCES users(id)
    );

    CREATE TABLE IF NOT EXISTS audit_logs (
      id TEXT PRIMARY KEY,
      timestamp TEXT NOT NULL,
      level TEXT NOT NULL,
      source TEXT NOT NULL,
      message TEXT NOT NULL,
      actor TEXT,
      metadata TEXT
    );

    CREATE TABLE IF NOT EXISTS operation_runs (
      id TEXT PRIMARY KEY,
      action TEXT NOT NULL,
      status TEXT NOT NULL,
      message TEXT,
      actor TEXT,
      metadata TEXT,
      created_at TEXT NOT NULL,
      completed_at TEXT
    );

    CREATE TABLE IF NOT EXISTS settings_snapshots (
      id TEXT PRIMARY KEY,
      payload TEXT NOT NULL,
      actor TEXT,
      created_at TEXT NOT NULL
    );

    CREATE TABLE IF NOT EXISTS backup_records (
      id TEXT PRIMARY KEY,
      path TEXT NOT NULL,
      type TEXT NOT NULL,
      status TEXT NOT NULL,
      size_bytes INTEGER NOT NULL DEFAULT 0,
      note TEXT,
      created_at TEXT NOT NULL,
      updated_at TEXT NOT NULL
    );
  `)
  return db
}

export function createRepositories(db) {
  return {
    users: {
      findByUsername(username) {
        return db.prepare('SELECT * FROM users WHERE username = ?').get(username)
      },
      upsert(user) {
        db.prepare(`
          INSERT INTO users (id, username, password_hash, created_at, updated_at)
          VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
          ON CONFLICT(username) DO UPDATE SET password_hash = excluded.password_hash, updated_at = CURRENT_TIMESTAMP
        `).run(user.id, user.username, user.passwordHash)
      },
    },
    sessions: {
      create(session) {
        db.prepare(`
          INSERT INTO sessions (id, user_id, issued_at, expires_at)
          VALUES (?, ?, ?, ?)
        `).run(session.id, session.userId, session.issuedAt, session.expiresAt)
      },
      findActive(id) {
        return db.prepare(`
          SELECT * FROM sessions
          WHERE id = ? AND revoked_at IS NULL AND expires_at > ?
        `).get(id, new Date().toISOString())
      },
      revoke(id) {
        db.prepare('UPDATE sessions SET revoked_at = CURRENT_TIMESTAMP WHERE id = ?').run(id)
      },
    },
    auditLogs: {
      insert(row) {
        db.prepare(`
          INSERT INTO audit_logs (id, timestamp, level, source, message, actor, metadata)
          VALUES (?, ?, ?, ?, ?, ?, ?)
        `).run(row.id, row.timestamp, row.level, row.source, row.message, row.actor || null, JSON.stringify(row.metadata || {}))
      },
      list(limit = 80) {
        return db.prepare(`
          SELECT id, timestamp, level, source, message, actor, metadata
          FROM audit_logs
          ORDER BY timestamp DESC
          LIMIT ?
        `).all(limit).map((row) => ({ ...row, metadata: parseJson(row.metadata, {}) }))
      },
    },
    operations: {
      start(row) {
        db.prepare(`
          INSERT INTO operation_runs (id, action, status, message, actor, metadata, created_at)
          VALUES (?, ?, ?, ?, ?, ?, ?)
        `).run(row.id, row.action, row.status, row.message || null, row.actor || null, JSON.stringify(row.metadata || {}), row.createdAt)
      },
      finish(id, status, message) {
        db.prepare(`
          UPDATE operation_runs
          SET status = ?, message = ?, completed_at = ?
          WHERE id = ?
        `).run(status, message || null, new Date().toISOString(), id)
      },
      list(limit = 50) {
        return db.prepare(`
          SELECT * FROM operation_runs
          ORDER BY created_at DESC
          LIMIT ?
        `).all(limit).map((row) => ({ ...row, metadata: parseJson(row.metadata, {}) }))
      },
    },
    settingsSnapshots: {
      insert(row) {
        db.prepare(`
          INSERT INTO settings_snapshots (id, payload, actor, created_at)
          VALUES (?, ?, ?, ?)
        `).run(row.id, JSON.stringify(row.payload), row.actor || null, row.createdAt)
      },
      latest() {
        const row = db.prepare(`
          SELECT payload FROM settings_snapshots
          ORDER BY created_at DESC
          LIMIT 1
        `).get()
        return row ? parseJson(row.payload, null) : null
      },
    },
    backupRecords: {
      upsert(row) {
        db.prepare(`
          INSERT INTO backup_records (id, path, type, status, size_bytes, note, created_at, updated_at)
          VALUES (?, ?, ?, ?, ?, ?, ?, ?)
          ON CONFLICT(id) DO UPDATE SET
            path = excluded.path,
            type = excluded.type,
            status = excluded.status,
            size_bytes = excluded.size_bytes,
            note = excluded.note,
            updated_at = excluded.updated_at
        `).run(row.id, row.path, row.type, row.status, row.sizeBytes || 0, row.note || null, row.createdAt, row.updatedAt)
      },
      list(limit = 200) {
        return db.prepare(`
          SELECT * FROM backup_records
          ORDER BY created_at DESC
          LIMIT ?
        `).all(limit)
      },
    },
  }
}

function parseJson(value, fallback) {
  try {
    return JSON.parse(value)
  } catch {
    return fallback
  }
}
