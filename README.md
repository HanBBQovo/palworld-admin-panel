# Palworld Ops

Palworld 专用服务器管理面板。项目现在包含 React 前端和 Fastify 后端，后端负责登录鉴权、读取容器状态、执行 RCON、创建/恢复备份、保存面板参数，并能通过同一个 `.env` 和 Docker Compose 跟游戏服务器一起部署。

技术栈：Vite 6 · React 19 · TypeScript 5.6 · Tailwind 3 · Fastify · Node built-in SQLite。

---

## 快速开始

```bash
npm install
cp .env.example .env
npm run dev:api  # Fastify API，默认 http://127.0.0.1:16824
npm run dev      # React 前端，默认 http://127.0.0.1:5278
```

默认登录密码来自 `.env` 的 `PANEL_AUTH_PASSWORD`。如果只想看纯前端 mock，可以设置 `VITE_USE_MOCK_API=true`。

常用命令：

```bash
npm run check:api
npm run build
npm run lint
npm start        # 生产模式：Fastify 同时提供 /api 和 dist 静态文件
```

---

## 后端能力

- 后端采用 Fastify 分层结构：`routes/`、`services/`、`db/`、`config/`。
- SQLite 数据库默认在 `PANEL_DB_FILE`，用于面板自己的状态，不保存 Palworld 世界存档。
- `POST /api/auth/login`：单密码登录，返回 Bearer token。
- `GET /api/palworld/status`：Docker inspect/stats、磁盘、端口策略、玩家数量。
- `GET /api/palworld/players`：通过 RCON `ShowPlayers` 读取在线玩家。
- `GET /api/palworld/logs`：聚合审计日志和容器日志。
- `POST /api/palworld/rcon`：执行白名单内 RCON 命令。
- `GET/POST /api/palworld/backups`：读取备份列表、创建手动备份。
- `POST /api/palworld/backups/:id/restore`：恢复目录备份。
- `POST /api/palworld/maintenance`：保存世界、重启容器、更新、延迟关服。
- `GET/PUT /api/palworld/settings`：读取和保存参数；保存时会更新 `PANEL_ENV_FILE` 指向的 `.env`。

RCON 默认只允许这些前缀：`Info`、`ShowPlayers`、`Save`、`Broadcast`、`KickPlayer`、`BanPlayer`、`Shutdown`。需要开放任意 RCON 时设置 `PANEL_ALLOW_RAW_RCON=true`。

## 数据库边界

需要数据库，但只需要轻量 SQLite。它负责保存：

- 面板管理员用户和登录会话。
- 审计日志和维护操作记录。
- 参数保存快照。
- 备份索引和状态。

不应该把 Palworld 存档放进数据库。世界存档、自动备份目录、恢复源文件仍然走文件系统和挂载目录。

## Docker 部署

参考文件：

- `.env.example`：统一配置游戏服务器和面板密码。
- `deploy/docker-compose.example.yml`：游戏容器 + 面板容器的组合部署示例。
- `Dockerfile`：前端构建后由 Node 后端同时提供静态文件和 API。

公网只需要反代面板端口，例如 `127.0.0.1:16824`。游戏连接端口 `8211/udp` 不能用普通 HTTP 反代；RCON `25575/tcp` 和 REST `8212/tcp` 不要公网裸露。

## 安全边界

- 面板密码：`PANEL_AUTH_PASSWORD`。
- 游戏服务器密码：`PALWORLD_SERVER_PASSWORD`。
- 管理员/RCON 密码：`PALWORLD_ADMIN_PASSWORD`。
- 后端会写审计日志到 SQLite 的 `audit_logs` 表。
- `.env` 不应提交到仓库，已经在 `.gitignore` 和 `.dockerignore` 里排除。
