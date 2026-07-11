#!/usr/bin/env python3

import asyncio
import hashlib
import hmac
import json
import os
import secrets
import sys
import threading
from datetime import datetime, timezone
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import unquote, urlparse
from uuid import UUID

RELEASE_DIR = Path(os.environ.get("SAVE_PAL_RELEASE_DIR", "/opt/save-pal"))
SNAPSHOT_DIR = Path(os.environ.get("SAVE_PATH", "/advanced/world-snapshot"))
STATE_FILE = Path(os.environ.get("STATE_FILE", "/app/state/index.json"))
PASSWORD = os.environ.get("WEB_PASSWORD", "")
PORT = int(os.environ.get("WEB_PORT", "16826"))
TOKEN = secrets.token_urlsafe(32)

os.chdir(RELEASE_DIR)
sys.path.append(str(RELEASE_DIR / "lib"))

from palworld_save_pal.game.save_manager import SaveManager  # noqa: E402


def iso_now():
    return datetime.now(timezone.utc).isoformat()


def uid_text(value):
    return str(value).replace("-", "").upper() if value else ""


def safe_value(factory, default=None):
    try:
        value = factory()
        return default if value is None else value
    except Exception:
        return default


def json_value(value):
    if isinstance(value, (str, int, float, bool)) or value is None:
        return value
    if isinstance(value, UUID):
        return uid_text(value)
    if hasattr(value, "value"):
        return json_value(value.value)
    if isinstance(value, dict):
        return {str(key): json_value(item) for key, item in value.items()}
    if isinstance(value, (list, tuple, set)):
        return [json_value(item) for item in value]
    return str(value)


def item_rows(container):
    if not container:
        return []
    rows = []
    for slot in safe_value(lambda: container.slots, []) or []:
        rows.append(
            {
                "SlotIndex": safe_value(lambda: slot.slot_index, 0),
                "ItemId": safe_value(lambda: slot.static_id, "") or "",
                "StackCount": safe_value(lambda: slot.count, 0),
            }
        )
    return rows


def pal_row(pal):
    skills = safe_value(lambda: pal.active_skills, []) or []
    return {
        "level": safe_value(lambda: pal.level, 0),
        "type": safe_value(lambda: pal.character_id, "") or "",
        "gender": json_value(safe_value(lambda: pal.gender, "")),
        "nickname": safe_value(lambda: pal.nickname, "") or "",
        "is_lucky": safe_value(lambda: pal.is_lucky, False),
        "is_boss": safe_value(lambda: pal.is_boss, False),
        "workspeed": 0,
        "melee": 0,
        "ranged": 0,
        "defense": 0,
        "skills": json_value(skills),
    }


def player_row(player, summary=None, include_detail=True):
    uid = safe_value(lambda: player.uid, None) if player else safe_value(lambda: summary.uid, None)
    nickname = safe_value(lambda: player.nickname, "") if player else safe_value(lambda: summary.nickname, "")
    level = safe_value(lambda: player.level, 0) if player else safe_value(lambda: summary.level, 0)
    row = {
        "player_uid": uid_text(uid),
        "nickname": nickname or "",
        "level": level or 0,
        "exp": 0,
        "hp": 0,
        "max_hp": 0,
        "shield_hp": 0,
        "shield_max_hp": 0,
        "full_stomach": 0,
        "save_last_online": "",
        "last_online": "",
        "steam_id": "",
        "user_id": "",
        "account_name": "",
        "ip": "",
        "ping": 0,
        "location_x": 0,
        "location_y": 0,
        "building_count": 0,
    }
    if not player or not include_detail:
        return row
    hp = safe_value(lambda: player.hp, 0)
    location = safe_value(lambda: player.location, None)
    last_online = safe_value(lambda: player.last_online_time, None)
    row.update(
        {
            "exp": safe_value(lambda: player.exp, 0),
            "hp": hp,
            "max_hp": 0,
            "full_stomach": safe_value(lambda: player.stomach, 0),
            "save_last_online": last_online.isoformat() if last_online else "",
            "last_online": last_online.isoformat() if last_online else "",
            "location_x": safe_value(lambda: location.x, 0) if location else 0,
            "location_y": safe_value(lambda: location.y, 0) if location else 0,
            "pals": [pal_row(pal) for pal in (safe_value(lambda: player.pals.values(), []) or [])],
            "items": {
                "CommonContainerId": item_rows(safe_value(lambda: player.common_container, None)),
                "DropSlotContainerId": [],
                "EssentialContainerId": item_rows(safe_value(lambda: player.essential_container, None)),
                "FoodEquipContainerId": item_rows(safe_value(lambda: player.food_equip_container, None)),
                "PlayerEquipArmorContainerId": item_rows(safe_value(lambda: player.player_equipment_armor_container, None)),
                "WeaponLoadOutContainerId": item_rows(safe_value(lambda: player.weapon_load_out_container, None)),
            },
        }
    )
    return row


def guild_row(guild, summary, player_names):
    guild_id = safe_value(lambda: guild.id, None) if guild else safe_value(lambda: summary.id, None)
    player_ids = safe_value(lambda: guild.players, []) if guild else []
    bases = []
    if guild:
        for base in safe_value(lambda: guild.bases.values(), []) or []:
            location = safe_value(lambda: base.location, None)
            bases.append(
                {
                    "id": uid_text(safe_value(lambda: base.id, None)),
                    "area": safe_value(lambda: base.area_range, 0),
                    "location_x": safe_value(lambda: location.x, 0) if location else 0,
                    "location_y": safe_value(lambda: location.y, 0) if location else 0,
                }
            )
    return {
        "id": uid_text(guild_id),
        "name": safe_value(lambda: guild.name, "") if guild else safe_value(lambda: summary.name, ""),
        "base_camp_level": safe_value(lambda: guild.base_camp_level, 0) if guild else 0,
        "admin_player_uid": uid_text(
            safe_value(lambda: summary.admin_player_uid, None)
            or (safe_value(lambda: guild.admin_player_uid, None) if guild else None)
        ),
        "players": [
            {"player_uid": uid_text(uid), "nickname": player_names.get(uid_text(uid), "")}
            for uid in player_ids
        ],
        "base_camp": bases,
    }


async def noop(_message):
    return None


async def build_index():
    level_path = SNAPSHOT_DIR / "Level.sav"
    players_dir = SNAPSHOT_DIR / "Players"
    if not level_path.is_file() or not players_dir.is_dir():
        raise FileNotFoundError("snapshot must contain Level.sav and Players/")

    refs = {}
    for path in players_dir.glob("*.sav"):
        stem = path.stem
        is_dps = stem.lower().endswith("_dps")
        raw_uid = stem[:-4] if is_dps else stem
        try:
            player_uid = UUID(raw_uid)
        except ValueError:
            continue
        refs.setdefault(player_uid, {})["dps" if is_dps else "sav"] = str(path)

    manager = await SaveManager(level_sav_path=str(level_path)).load_sav_files(
        level_path.read_bytes(),
        refs,
        (SNAPSHOT_DIR / "LevelMeta.sav").read_bytes()
        if (SNAPSHOT_DIR / "LevelMeta.sav").is_file()
        else None,
        noop,
    )

    players = []
    player_names = {}
    summaries = manager.get_player_summaries()
    for player_uid, summary in summaries.items():
        detail = None
        try:
            detail = await manager.load_player_on_demand(player_uid, noop)
        except Exception as error:
            print(f"player {uid_text(player_uid)} detail failed: {error}", flush=True)
        row = player_row(detail, summary)
        players.append(row)
        player_names[row["player_uid"]] = row["nickname"]

    guilds = []
    for guild_id, summary in manager.get_guild_summaries().items():
        detail = None
        try:
            detail = manager._load_guild_by_id(guild_id)
        except Exception as error:
            print(f"guild {uid_text(guild_id)} detail failed: {error}", flush=True)
        guilds.append(guild_row(detail, summary, player_names))

    players.sort(key=lambda row: row.get("save_last_online", ""), reverse=True)
    guilds.sort(key=lambda row: row.get("base_camp_level", 0), reverse=True)
    fingerprint = hashlib.sha256(level_path.read_bytes()).hexdigest()[:16]
    return {
        "version": 1,
        "source": "Palworld Save Pal v0.17.4",
        "fingerprint": fingerprint,
        "updated_at": iso_now(),
        "players": players,
        "guilds": guilds,
    }


class IndexStore:
    def __init__(self):
        self.lock = threading.RLock()
        self.data = None
        self.syncing = False
        self.last_error = ""
        self.load()

    def load(self):
        try:
            with STATE_FILE.open("r", encoding="utf-8") as handle:
                self.data = json.load(handle)
        except (FileNotFoundError, json.JSONDecodeError, OSError):
            self.data = None

    def status(self):
        with self.lock:
            return {
                "ready": self.data is not None,
                "syncing": self.syncing,
                "stale": bool(self.last_error and self.data),
                "last_error": self.last_error,
                "updated_at": self.data.get("updated_at", "") if self.data else "",
                "players": len(self.data.get("players", [])) if self.data else 0,
                "guilds": len(self.data.get("guilds", [])) if self.data else 0,
            }

    def start_sync(self):
        with self.lock:
            if self.syncing:
                return False
            self.syncing = True
            self.last_error = ""
        threading.Thread(target=self._sync, daemon=True).start()
        return True

    def _sync(self):
        try:
            data = asyncio.run(build_index())
            STATE_FILE.parent.mkdir(parents=True, exist_ok=True)
            temp = STATE_FILE.with_suffix(".tmp")
            with temp.open("w", encoding="utf-8") as handle:
                json.dump(data, handle, ensure_ascii=False, separators=(",", ":"))
            temp.replace(STATE_FILE)
            with self.lock:
                self.data = data
                self.last_error = ""
            print(
                f"index ready: {len(data['players'])} players, {len(data['guilds'])} guilds",
                flush=True,
            )
        except Exception as error:
            with self.lock:
                self.last_error = str(error)
            print(f"index sync failed: {error}", flush=True)
        finally:
            with self.lock:
                self.syncing = False


STORE = IndexStore()


class Handler(BaseHTTPRequestHandler):
    server_version = "palworld-readonly-index/1"

    def log_message(self, format_string, *args):
        print(f"{self.address_string()} {format_string % args}", flush=True)

    def write_json(self, status, payload):
        body = json.dumps(payload, ensure_ascii=False, separators=(",", ":")).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def read_json(self):
        length = min(int(self.headers.get("Content-Length", "0")), 65536)
        return json.loads(self.rfile.read(length) or b"{}")

    def authorized(self):
        header = self.headers.get("Authorization", "")
        return header.startswith("Bearer ") and hmac.compare_digest(header[7:], TOKEN)

    def require_auth(self):
        if self.authorized():
            return True
        self.write_json(HTTPStatus.UNAUTHORIZED, {"error": "unauthorized"})
        return False

    def do_POST(self):
        parsed = urlparse(self.path)
        if parsed.path == "/api/login":
            try:
                supplied = str(self.read_json().get("password", ""))
            except (ValueError, json.JSONDecodeError):
                self.write_json(HTTPStatus.BAD_REQUEST, {"error": "invalid json"})
                return
            if not PASSWORD or not hmac.compare_digest(supplied, PASSWORD):
                self.write_json(HTTPStatus.UNAUTHORIZED, {"error": "incorrect password"})
                return
            self.write_json(HTTPStatus.OK, {"token": TOKEN})
            return
        if parsed.path == "/api/sync":
            if not self.require_auth():
                return
            started = STORE.start_sync()
            self.write_json(HTTPStatus.OK, {"success": True, "started": started})
            return
        self.write_json(HTTPStatus.NOT_FOUND, {"error": "not found"})

    def do_GET(self):
        parsed = urlparse(self.path)
        if parsed.path == "/health":
            self.write_json(HTTPStatus.OK, {"ok": True})
            return
        if not self.require_auth():
            return
        if parsed.path == "/api/status":
            self.write_json(HTTPStatus.OK, STORE.status())
            return
        with STORE.lock:
            data = STORE.data
            if data is None:
                self.write_json(HTTPStatus.SERVICE_UNAVAILABLE, {"error": STORE.last_error or "index not ready"})
                return
            if parsed.path == "/api/player":
                self.write_json(HTTPStatus.OK, data.get("players", []))
                return
            if parsed.path.startswith("/api/player/"):
                uid = unquote(parsed.path.removeprefix("/api/player/")).upper()
                row = next((item for item in data.get("players", []) if item.get("player_uid", "").upper() == uid), None)
                self.write_json(HTTPStatus.OK if row else HTTPStatus.NOT_FOUND, row or {})
                return
            if parsed.path == "/api/guild":
                self.write_json(HTTPStatus.OK, data.get("guilds", []))
                return
            if parsed.path.startswith("/api/guild/"):
                uid = unquote(parsed.path.removeprefix("/api/guild/")).upper()
                row = next((item for item in data.get("guilds", []) if item.get("admin_player_uid", "").upper() == uid), None)
                self.write_json(HTTPStatus.OK if row else HTTPStatus.NOT_FOUND, row or {})
                return
        self.write_json(HTTPStatus.NOT_FOUND, {"error": "not found"})


if __name__ == "__main__":
    if not PASSWORD:
        raise SystemExit("WEB_PASSWORD is required")
    STORE.start_sync()
    server = ThreadingHTTPServer(("0.0.0.0", PORT), Handler)
    print(f"read-only world index listening on 0.0.0.0:{PORT}", flush=True)
    server.serve_forever()
