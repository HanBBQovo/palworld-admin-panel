# Palworld Ops 后端与 Docker 集成规划

这个项目使用 Go 后端代理层，后端在游戏服务器本机或同一个 Docker Compose 网络里执行 Docker、RCON 和文件操作。

## 目标形态

推荐最终用一个 Compose 同时管理游戏服务和面板：

```text
palworld-stack/
  .env
  docker-compose.yml
  palworld-data/
  panel/
```

服务拆分：

- `palworld`: 当前游戏服务器容器，继续使用 `thijsvanloef/palworld-server-docker`。
- `panel`: React 前端 + Go API 同容器运行，只监听本机或内网，用来访问 Docker socket、RCON、审计文件和存档目录。

## 统一 .env

建议用一个 `.env` 同时驱动 Palworld 和管理面板：

```env
TZ=Asia/Shanghai

PALWORLD_SERVER_NAME=Palworld Dedicated Server
PALWORLD_SERVER_DESCRIPTION=Managed by Palworld Ops
PALWORLD_PLAYERS=32
PALWORLD_SERVER_PASSWORD=change-me
PALWORLD_ADMIN_PASSWORD=change-me
PALWORLD_PUBLIC_IP=your.server.ip
PALWORLD_PUBLIC_DOMAIN=pal.example.com
PALWORLD_PORT=8211
PALWORLD_QUERY_PORT=27015
PALWORLD_RCON_PORT=25575
PALWORLD_REST_PORT=8212

PALWORLD_UPDATE_ON_BOOT=true
PALWORLD_AUTO_UPDATE_ENABLED=true
PALWORLD_AUTO_UPDATE_CRON=0 4 * * *
PALWORLD_AUTO_REBOOT_ENABLED=true
PALWORLD_AUTO_REBOOT_CRON=0 5 * * *
PALWORLD_BACKUP_ENABLED=true
PALWORLD_BACKUP_CRON=0 * * * *
PALWORLD_BACKUP_RETENTION=72

PANEL_AUTH_PASSWORD=change-panel-password
PANEL_JWT_SECRET=change-long-random-secret
PANEL_API_BIND=0.0.0.0
PANEL_API_PORT=16824
PANEL_STATE_DIR=/data/panel-state
PANEL_DB_FILE=/data/panel-state/panel.sqlite
PANEL_ALLOW_RAW_RCON=false
```

## Compose 草案

```yaml
services:
  palworld:
    image: thijsvanloef/palworld-server-docker:latest
    container_name: palworld-server
    restart: unless-stopped
    stop_grace_period: 120s
    env_file: .env
    ports:
      - "${PALWORLD_PORT:-8211}:8211/udp"
      - "${PALWORLD_QUERY_PORT:-27015}:27015/udp"
      - "127.0.0.1:${PALWORLD_RCON_PORT:-25575}:25575/tcp"
      - "127.0.0.1:${PALWORLD_REST_PORT:-8212}:8212/tcp"
    environment:
      TZ: "${TZ}"
      PORT: "8211"
      QUERY_PORT: "27015"
      PLAYERS: "${PALWORLD_PLAYERS}"
      SERVER_NAME: "${PALWORLD_SERVER_NAME}"
      SERVER_DESCRIPTION: "${PALWORLD_SERVER_DESCRIPTION}"
      SERVER_PASSWORD: "${PALWORLD_SERVER_PASSWORD}"
      ADMIN_PASSWORD: "${PALWORLD_ADMIN_PASSWORD}"
      RCON_ENABLED: "true"
      RCON_PORT: "25575"
      REST_API_ENABLED: "true"
      REST_API_PORT: "8212"
      UPDATE_ON_BOOT: "${PALWORLD_UPDATE_ON_BOOT}"
      AUTO_UPDATE_ENABLED: "${PALWORLD_AUTO_UPDATE_ENABLED}"
      AUTO_UPDATE_CRON_EXPRESSION: "${PALWORLD_AUTO_UPDATE_CRON}"
      AUTO_REBOOT_ENABLED: "${PALWORLD_AUTO_REBOOT_ENABLED}"
      AUTO_REBOOT_CRON_EXPRESSION: "${PALWORLD_AUTO_REBOOT_CRON}"
      BACKUP_ENABLED: "${PALWORLD_BACKUP_ENABLED}"
      BACKUP_CRON_EXPRESSION: "${PALWORLD_BACKUP_CRON}"
      BACKUP_RETENTION_POLICY: "true"
      BACKUP_RETENTION_AMOUNT_TO_KEEP: "${PALWORLD_BACKUP_RETENTION}"
    volumes:
      - ./palworld-data:/palworld

  panel:
    build: ./palworld-admin-panel
    restart: unless-stopped
    ports:
      - "127.0.0.1:16824:16824"
    env_file: .env
    environment:
      PALWORLD_RCON_HOST: palworld
      PANEL_WEB_ROOT: /app/dist
      PANEL_STATE_DIR: /data/panel-state
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./palworld-data:/palworld:rw
      - ./panel-state:/data/panel-state:rw
    depends_on:
      - palworld
```

公网入口只反代 `panel-web`，不要把 RCON/REST API 暴露公网。

## 后端接口契约

前端已经按这些能力拆好页面，后端可以按这个契约实现：

- `POST /api/auth/login`
- `POST /api/auth/logout`
- `GET /api/auth/status`
- `GET /api/palworld/status`
- `GET /api/palworld/players`
- `GET /api/palworld/logs`
- `POST /api/palworld/rcon`
- `GET /api/palworld/backups`
- `POST /api/palworld/backups`
- `POST /api/palworld/backups/:id/restore`
- `POST /api/palworld/maintenance`
- `GET /api/palworld/settings`
- `PUT /api/palworld/settings`
- `GET /api/palworld/rcon-commands`

## 后端必须做的安全限制

- 登录密码从 `PANEL_AUTH_PASSWORD` 读取。
- token secret 从 `PANEL_JWT_SECRET` 读取。
- 所有危险动作必须鉴权。
- RCON 命令不要允许任意公网请求直通；至少限制命令白名单或强确认。
- 恢复备份、关服、封禁玩家、更新服务必须写审计日志。
- 不要公网暴露 `25575` 和 `8212`。
- Docker socket 如果挂载，后端必须极小权限、极少 API 面，不能做通用 Docker 管理器。

## 前端当前状态

- 默认走真实 `apiRequest`，开发环境请求 Vite 代理后的 Go API。
- 只有设置 `VITE_USE_MOCK_API=true` 时，前端才使用内置 mock。
- 面板密码来自 `PANEL_AUTH_PASSWORD`，管理员/RCON 密码来自 `PALWORLD_ADMIN_PASSWORD`。

## 数据库取舍

这个项目需要数据库，但不需要 PostgreSQL/MySQL 这种重量级数据库。SQLite 足够覆盖面板自身状态：

- `users` / `sessions`: 面板登录和会话。
- `audit_logs`: RCON、备份、恢复、重启、更新等审计记录。
- `operation_runs`: 高风险动作执行记录。
- `settings_snapshots`: 参数保存快照。
- `backup_records`: 备份索引。

Palworld 世界存档和备份本体仍然放在挂载目录，不进入数据库。
