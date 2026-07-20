#!/usr/bin/env python3

import hmac
import json
import os
import secrets
import subprocess
import threading
from datetime import datetime, timezone
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import unquote, urlparse

SNAPSHOT_DIR = Path(os.environ.get("SAVE_PATH", "/advanced/world-snapshot"))
STATE_FILE = Path(os.environ.get("STATE_FILE", "/app/state/index.json"))
INDEXER_BIN = Path(os.environ.get("SAVE_PAL_INDEXER_BIN", "/usr/local/bin/palworld-world-indexer"))
GAME_DATA_DIR = Path(os.environ.get("SAVE_PAL_DATA_DIR", "/opt/save-pal-data/json"))
PASSWORD = os.environ.get("WEB_PASSWORD", "")
PORT = int(os.environ.get("WEB_PORT", "16826"))
TOKEN = secrets.token_urlsafe(32)


def iso_now():
    return datetime.now(timezone.utc).isoformat()


def build_index():
    level_path = SNAPSHOT_DIR / "Level.sav"
    players_dir = SNAPSHOT_DIR / "Players"
    if not level_path.is_file() or not players_dir.is_dir():
        raise FileNotFoundError("snapshot must contain Level.sav and Players/")
    process = subprocess.run(
        [str(INDEXER_BIN), str(SNAPSHOT_DIR), str(GAME_DATA_DIR)],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        timeout=180,
        check=False,
    )
    if process.returncode != 0:
        detail = process.stderr.strip()[-4096:] or f"indexer exited with status {process.returncode}"
        raise RuntimeError(detail)
    data = json.loads(process.stdout)
    data["updated_at"] = iso_now()
    return data


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
                "fingerprint": self.data.get("fingerprint", "") if self.data else "",
                "source": self.data.get("source", "") if self.data else "",
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
            data = build_index()
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
