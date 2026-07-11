#!/usr/bin/env python3

import os
import sys
from contextlib import asynccontextmanager
from pathlib import Path
from uuid import UUID

RELEASE_DIR = Path(os.environ.get("SAVE_PAL_RELEASE_DIR", "/opt/editor"))
WORKSPACE_DIR = Path(os.environ.get("WORKSPACE_DIR", "/workspace"))
STATE_DIR = Path(os.environ.get("STATE_DIR", "/app/state"))
PORT = int(os.environ.get("WEB_PORT", "16827"))

os.chdir(STATE_DIR)
sys.path.append(str(RELEASE_DIR / "lib"))

import uvicorn  # noqa: E402
from fastapi import FastAPI, Request, WebSocket, WebSocketDisconnect  # noqa: E402
from fastapi.responses import FileResponse, RedirectResponse  # noqa: E402
from palworld_save_pal.api.convert import router as convert_router  # noqa: E402
from palworld_save_pal.db.bootstrap import create_db_and_tables  # noqa: E402
from palworld_save_pal.game.gvas_codec import SaveType  # noqa: E402
from palworld_save_pal.state import get_app_state  # noqa: E402
from palworld_save_pal.utils.logging_config import setup_logging  # noqa: E402
from palworld_save_pal.ws.manager import ConnectionManager  # noqa: E402


async def noop(_message):
    return None


async def load_workspace():
    level_path = WORKSPACE_DIR / "Level.sav"
    players_dir = WORKSPACE_DIR / "Players"
    if not level_path.is_file() or not players_dir.is_dir():
        raise RuntimeError("maintenance workspace must contain Level.sav and Players/")

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

    state = get_app_state()
    await state.process_save_files(
        sav_id=level_path.name,
        level_sav=level_path.read_bytes(),
        level_meta=(WORKSPACE_DIR / "LevelMeta.sav").read_bytes()
        if (WORKSPACE_DIR / "LevelMeta.sav").is_file()
        else None,
        player_file_refs=refs,
        ws_callback=noop,
        save_type=SaveType.STEAM,
    )


@asynccontextmanager
async def lifespan(_app):
    create_db_and_tables()
    setup_logging(dev_mode=False)
    await load_workspace()
    yield


app = FastAPI(lifespan=lifespan)
app.include_router(convert_router)
manager = ConnectionManager()


@app.middleware("http")
async def static_files(request: Request, call_next):
    path = request.url.path
    if path.startswith("/ws") or path.startswith("/api"):
        return await call_next(request)

    target = RELEASE_DIR / "ui" / path.lstrip("/")
    if target.is_dir() and (target / "index.html").is_file():
        return FileResponse(target / "index.html")
    if target.is_file():
        return FileResponse(target)
    if path != "/":
        return RedirectResponse(url=f"/?path={path}")
    return FileResponse(RELEASE_DIR / "ui" / "index.html")


@app.websocket("/ws/{client_id}")
async def websocket_endpoint(websocket: WebSocket, client_id: int):
    del client_id
    await manager.connect(websocket)
    try:
        while True:
            await manager.process_message(await websocket.receive_text(), websocket)
    except WebSocketDisconnect:
        manager.disconnect(websocket)


if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=PORT, ws_max_size=2**30)
