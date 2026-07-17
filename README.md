# Palworld Admin Panel

Palworld 专用服务器管理面板。项目包含 React 前端和 Go 后端，后端负责登录鉴权、读取容器状态、执行 RCON、创建/恢复备份、保存面板参数，并能通过同一个 `.env` 和 Docker Compose 跟游戏服务器一起部署。

技术栈：Vite 6 · React 19 · TypeScript 5.6 · Tailwind 3 · Go 标准库。

---

## 快速开始

```bash
npm install
cp .env.example .env
npm run dev:api  # Go API，默认 http://127.0.0.1:16824
npm run dev      # React 前端，默认 http://127.0.0.1:5278
```

默认登录密码来自 `.env` 的 `PANEL_AUTH_PASSWORD`。如果只想看纯前端 mock，可以设置 `VITE_USE_MOCK_API=true`。

常用命令：

```bash
npm run check:api
npm run build
npm run lint
npm start        # 本地启动 Go API，同时提供 /api 和 dist 静态文件
```

---

## 后端能力

- 后端在 `backend-go/`，生产镜像只运行 Go 二进制，不再依赖 Node 后端。
- 面板状态默认写入 `PANEL_STATE_DIR` 下的 JSON/JSONL 文件，不保存 Palworld 世界存档。
- `POST /api/auth/login`：单密码登录，返回 Bearer token。
- `GET /api/palworld/status`：Docker inspect/stats、磁盘、端口策略、最近玩家缓存；不会同步等待 RCON。
- `GET /api/palworld/players`：通过 RCON `ShowPlayers` 读取在线玩家；RCON 超时时返回空列表，不让页面 500。
- `GET /api/palworld/logs`：聚合审计日志和容器日志。
- `POST /api/palworld/rcon`：执行白名单内 RCON 命令。
- `GET/POST /api/palworld/backups`：读取备份列表、创建手动备份。
- `POST /api/palworld/backups/:id/restore`：恢复目录备份。
- `POST /api/palworld/maintenance`：保存世界、重启容器、更新、延迟关服。
- `GET/PUT /api/palworld/settings`：读取和保存参数；保存时会更新 `PANEL_ENV_FILE` 指向的 `.env`。游戏参数集对齐 `palworld-server-docker 2.6.0` 的 118 项生成模板，其中 116 项可编辑，RCON/REST 内部端口由部署层固定。

高级控制台通过同一个 Go API 接入三层能力：

- `GET /api/palworld/live/*`：官方 REST 实时玩家、坐标、等级、延迟和性能指标。
- `GET /api/palworld/world/*`：读取固定版本 Save Pal 兼容索引生成的玩家、帕鲁、背包、公会和基地数据。
- `POST /api/palworld/editor/previews`：创建存档编辑预览；不会直接写游戏存档。
- `GET /api/palworld/capabilities`：返回每层的安装、可达、待重启和安全门禁状态。

世界索引只解析一份已经完成的压缩备份快照。维护编辑器默认休眠，且 `PANEL_EDITOR_APPLY_ENABLED=false`；游戏仍在运行、存在在线玩家或没有预备份时，生产存档写回必须保持锁定。

面板通过宿主 Docker socket 启动高级服务时，`PALWORLD_HOST_STACK_DIR` 必须是 Compose 目录在宿主机上的绝对路径；直接在宿主机运行示例 Compose 时可保持为 `.`。

RCON 默认只允许这些前缀：`Info`、`ShowPlayers`、`Save`、`KickPlayer`、`BanPlayer`、`Shutdown`。中文广播使用官方 REST 公告接口，不经过 RCON。需要开放任意 RCON 时设置 `PANEL_ALLOW_RAW_RCON=true`。RCON 超时时间默认由 `PANEL_RCON_TIMEOUT_MS=1800` 控制。

CPU 显示使用主机口径：Docker `stats` 的容器 CPU 百分比会除以宿主机核心数，避免把多核累计值误读成整机 CPU。

## 状态存储边界

当前不需要额外数据库。面板只需要本地轻量状态文件：

- `settings.json`：面板保存后的参数快照。
- `audit.jsonl`：登录、RCON、备份、维护等审计日志。
- `operations.jsonl`：维护任务执行记录。
- 备份列表实时从备份目录扫描，不再写入数据库。

不应该把 Palworld 存档放进数据库。世界存档、自动备份目录、恢复源文件仍然走文件系统和挂载目录。

## Docker 部署

参考文件：

- `.env.example`：统一配置游戏服务器和面板密码。
- `deploy/docker-compose.example.yml`：游戏容器 + 面板容器的组合部署示例。
- `deploy/docker-compose.advanced.yml`：只读世界索引侧车和维护期编辑器配置。
- `Dockerfile`：Node 只用于前端构建，最终镜像运行 Go API 并提供静态文件。
- 生产镜像内置 `docker` 和 `docker compose` CLI，用于面板触发重启、更新等维护动作。

公网只需要反代面板端口，例如 `127.0.0.1:16824`。游戏连接端口 `8211/udp` 不能用普通 HTTP 反代；RCON `25575/tcp` 和 REST `8212/tcp` 不要公网裸露。

## 安全边界

- 面板密码：`PANEL_AUTH_PASSWORD`。
- 游戏服务器密码：`PALWORLD_SERVER_PASSWORD`。
- 管理员/RCON 密码：`PALWORLD_ADMIN_PASSWORD`。
- 后端会写审计日志到 `PANEL_STATE_DIR/audit.jsonl`。
- 第三方侧车不能访问 Docker socket，也不能读取面板认证和审计目录。
- `.env` 不应提交到仓库，已经在 `.gitignore` 和 `.dockerignore` 里排除。
