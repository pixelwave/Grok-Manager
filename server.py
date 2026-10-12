#!/usr/bin/env python3
"""Local portal for Grok sessions on this machine.

Reads session metadata from ~/.grok/sessions and can delete one session.
Bind address is localhost; nginx publishes it on the LAN.
"""

from __future__ import annotations

import json
import os
import re
import shutil
import sqlite3
import subprocess
import sys
import threading
import urllib.parse
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

ID_RE = re.compile(r"^[0-9a-f-]{36}$")
HOST = "127.0.0.1"
PORT = 18090
ROOT_DIR = os.path.dirname(os.path.abspath(__file__))
STATIC_DIR = os.path.join(ROOT_DIR, "static")


def app_version() -> str:
    try:
        with open(os.path.join(ROOT_DIR, "VERSION"), encoding="utf-8") as handle:
            version = handle.read().strip()
    except OSError:
        return "0"
    return version or "0"
_LOCKS_GUARD = threading.Lock()


def sessions_root() -> str:
    return os.environ.get("GROK_SESSIONS_DIR") or os.path.expanduser("~/.grok/sessions")


def locks_path() -> str:
    return os.environ.get("GROK_LOCKS_PATH") or os.path.join(
        os.path.dirname(os.path.abspath(__file__)), "locks.json"
    )


def load_locks() -> dict[str, set[str]]:
    try:
        with open(locks_path(), encoding="utf-8") as handle:
            data = json.load(handle)
    except (OSError, json.JSONDecodeError):
        data = {}
    if not isinstance(data, dict):
        data = {}
    session_items = data.get("sessions")
    folder_items = data.get("folders")
    if not isinstance(session_items, list):
        session_items = []
    if not isinstance(folder_items, list):
        folder_items = []
    return {
        "sessions": {
            item
            for item in session_items
            if isinstance(item, str) and ID_RE.fullmatch(item)
        },
        "folders": {
            item
            for item in folder_items
            if isinstance(item, str) and item.startswith("/")
        },
    }


def save_locks(locks: dict[str, set[str]]) -> None:
    path = locks_path()
    os.makedirs(os.path.dirname(path), exist_ok=True)
    temporary = path + ".tmp"
    payload = {
        "sessions": sorted(locks["sessions"]),
        "folders": sorted(locks["folders"]),
    }
    with open(temporary, "w", encoding="utf-8") as handle:
        json.dump(payload, handle, indent=2)
        handle.write("\n")
    os.replace(temporary, path)


def session_cwd(session_path: str) -> str | None:
    group_path = os.path.dirname(session_path)
    return group_cwd(group_path, os.path.basename(group_path))


def session_is_locked(session_id: str, cwd: str | None) -> bool:
    locks = load_locks()
    if session_id in locks["sessions"]:
        return True
    return bool(cwd) and cwd in locks["folders"]


def clear_session_lock(session_id: str) -> None:
    with _LOCKS_GUARD:
        locks = load_locks()
        if session_id not in locks["sessions"]:
            return
        locks["sessions"].discard(session_id)
        save_locks(locks)


def pins_path() -> str:
    return os.environ.get("GROK_PINS_PATH") or os.path.join(
        os.path.dirname(os.path.abspath(__file__)), "pins.json"
    )


def load_pins() -> list[str]:
    try:
        with open(pins_path(), encoding="utf-8") as handle:
            data = json.load(handle)
    except (OSError, json.JSONDecodeError):
        data = {}
    if not isinstance(data, dict):
        data = {}
    items = data.get("sessions")
    if not isinstance(items, list):
        return []
    pins: list[str] = []
    seen: set[str] = set()
    for item in items:
        if isinstance(item, str) and ID_RE.fullmatch(item) and item not in seen:
            seen.add(item)
            pins.append(item)
    return pins


def save_pins(pins: list[str]) -> None:
    path = pins_path()
    os.makedirs(os.path.dirname(path), exist_ok=True)
    temporary = path + ".tmp"
    with open(temporary, "w", encoding="utf-8") as handle:
        json.dump({"sessions": pins}, handle, indent=2)
        handle.write("\n")
    os.replace(temporary, path)


def clear_pin(session_id: str) -> None:
    with _LOCKS_GUARD:
        pins = load_pins()
        if session_id not in pins:
            return
        save_pins([item for item in pins if item != session_id])


def set_pin(ident: object, pinned: object) -> tuple[int, dict]:
    if not isinstance(ident, str) or pinned not in (True, False):
        return 400, {"error": "Unknown pin"}
    if not ID_RE.fullmatch(ident):
        return 400, {"error": "Invalid session id"}
    if pinned and find_session_dir(ident) is None:
        return 404, {"error": "Session not found"}
    with _LOCKS_GUARD:
        pins = [item for item in load_pins() if item != ident]
        if pinned:
            pins.insert(0, ident)
        save_pins(pins)
    return 200, {"ok": True}


def reorder_pins(order: object) -> tuple[int, dict]:
    if not isinstance(order, list):
        return 400, {"error": "Unknown pin"}
    cleaned: list[str] = []
    seen: set[str] = set()
    for item in order:
        if not isinstance(item, str) or not ID_RE.fullmatch(item) or item in seen:
            return 400, {"error": "Invalid session id"}
        seen.add(item)
        cleaned.append(item)
    with _LOCKS_GUARD:
        current = load_pins()
        if set(cleaned) != set(current):
            return 409, {"error": "Pin list changed"}
        save_pins(cleaned)
    return 200, {"ok": True}


def set_lock(kind: object, ident: object, locked: object) -> tuple[int, dict]:
    if kind not in ("session", "folder") or not isinstance(ident, str) or locked not in (True, False):
        return 400, {"error": "Unknown lock"}
    with _LOCKS_GUARD:
        locks = load_locks()
        if kind == "session":
            if not ID_RE.fullmatch(ident):
                return 400, {"error": "Invalid session id"}
            if locked and find_session_dir(ident) is None:
                return 404, {"error": "Session not found"}
            if locked:
                locks["sessions"].add(ident)
            else:
                locks["sessions"].discard(ident)
        else:
            if not ident.startswith("/") or len(ident) > 512:
                return 400, {"error": "Unknown folder"}
            if locked and ident not in {group["cwd"] for group in list_groups()}:
                return 404, {"error": "Folder not found"}
            if locked:
                locks["folders"].add(ident)
            else:
                locks["folders"].discard(ident)
        save_locks(locks)
    return 200, {"ok": True}


def active_sessions_path() -> str:
    return os.environ.get("GROK_ACTIVE_SESSIONS") or os.path.expanduser(
        "~/.grok/active_sessions.json"
    )


def using_real_store() -> bool:
    real = os.path.realpath(os.path.expanduser("~/.grok/sessions"))
    try:
        current = os.path.realpath(sessions_root())
    except OSError:
        return False
    return current == real


def pid_alive(pid: object) -> bool:
    try:
        number = int(pid)
    except (TypeError, ValueError):
        return False
    if number <= 0:
        return False
    try:
        os.kill(number, 0)
    except OSError:
        return False
    return True


def live_ids() -> set[str]:
    path = active_sessions_path()
    try:
        with open(path, encoding="utf-8") as handle:
            data = json.load(handle)
    except (OSError, json.JSONDecodeError):
        return set()
    if not isinstance(data, list):
        return set()
    found: set[str] = set()
    for row in data:
        if not isinstance(row, dict):
            continue
        sid = row.get("session_id")
        if isinstance(sid, str) and pid_alive(row.get("pid")):
            found.add(sid)
    return found


def group_cwd(group_path: str, dirname: str) -> str:
    """Return the working directory recorded for a session group."""
    cwd = urllib.parse.unquote(dirname)
    marker = os.path.join(group_path, ".cwd")
    if os.path.isfile(marker):
        try:
            with open(marker, encoding="utf-8") as handle:
                recorded = handle.read().strip()
        except OSError:
            recorded = ""
        if recorded:
            cwd = recorded
    return cwd


def _title(summary: dict) -> str:
    for key in ("generated_title", "session_summary"):
        value = summary.get(key)
        if isinstance(value, str) and value.strip():
            return value.strip()
    return "Untitled"


def _session_row(summary_path: str, live: set[str]) -> dict | None:
    try:
        with open(summary_path, encoding="utf-8") as handle:
            summary = json.load(handle)
    except (OSError, json.JSONDecodeError):
        return None
    if not isinstance(summary, dict):
        return None
    info = summary.get("info") if isinstance(summary.get("info"), dict) else {}
    sid = info.get("id")
    if not isinstance(sid, str) or not sid:
        sid = os.path.basename(os.path.dirname(summary_path))
    updated = summary.get("updated_at") if isinstance(summary.get("updated_at"), str) else ""
    created = summary.get("created_at") if isinstance(summary.get("created_at"), str) else ""
    model = summary.get("current_model_id")
    if not isinstance(model, str):
        model = None
    return {
        "id": sid,
        "title": _title(summary),
        "updatedAt": updated,
        "createdAt": created,
        "model": model,
        "live": sid in live,
    }


def list_groups() -> list[dict]:
    root = sessions_root()
    if not os.path.isdir(root):
        return []
    live = live_ids()
    locks = load_locks()
    pins = set(load_pins())
    grouped: dict[str, list[dict]] = {}
    try:
        names = sorted(os.listdir(root))
    except OSError:
        return []
    for name in names:
        group_path = os.path.join(root, name)
        if not os.path.isdir(group_path):
            continue
        cwd = group_cwd(group_path, name)
        try:
            children = os.listdir(group_path)
        except OSError:
            continue
        rows: list[dict] = []
        for child in children:
            session_path = os.path.join(group_path, child)
            summary_path = os.path.join(session_path, "summary.json")
            if not os.path.isdir(session_path) or not os.path.isfile(summary_path):
                continue
            row = _session_row(summary_path, live)
            if row is not None:
                row["locked"] = row["id"] in locks["sessions"]
                row["pinned"] = row["id"] in pins
                rows.append(row)
        if not rows:
            continue
        grouped.setdefault(cwd, []).extend(rows)

    groups = []
    for cwd in sorted(grouped):
        sessions = sorted(grouped[cwd], key=lambda row: row["updatedAt"], reverse=True)
        label = os.path.basename(cwd.rstrip(os.sep)) or cwd
        groups.append({
            "cwd": cwd,
            "label": label,
            "locked": cwd in locks["folders"],
            "sessions": sessions,
        })
    return groups


def find_session_dir(session_id: str) -> str | None:
    if not ID_RE.fullmatch(session_id):
        return None
    root = os.path.realpath(sessions_root())
    if not os.path.isdir(root):
        return None
    try:
        names = os.listdir(root)
    except OSError:
        return None
    for name in names:
        group_path = os.path.join(root, name)
        if not os.path.isdir(group_path):
            continue
        session_path = os.path.join(group_path, session_id)
        if not os.path.isdir(session_path):
            continue
        real = os.path.realpath(session_path)
        if os.path.basename(real) != session_id:
            continue
        if not real.startswith(root + os.sep):
            continue
        return real
    return None


def _official_delete(session_id: str) -> None:
    if not using_real_store():
        return
    try:
        subprocess.run(
            ["grok", "sessions", "delete", session_id],
            check=False,
            capture_output=True,
            text=True,
            timeout=60,
        )
    except (OSError, subprocess.TimeoutExpired):
        return


def _drop_index_row(session_id: str) -> None:
    database = os.path.join(sessions_root(), "session_search.sqlite")
    if not os.path.isfile(database):
        return
    try:
        connection = sqlite3.connect(database, timeout=2)
    except sqlite3.Error:
        return
    try:
        connection.execute("DELETE FROM session_docs WHERE session_id = ?", (session_id,))
        connection.commit()
    except sqlite3.Error:
        return
    finally:
        connection.close()


def delete_session(session_id: str) -> tuple[int, str | None]:
    if not ID_RE.fullmatch(session_id):
        return 400, "Invalid session id"
    path = find_session_dir(session_id)
    if path is None:
        return 404, "Session not found"
    if session_id in live_ids():
        return 409, "This session is open in Grok"
    if session_is_locked(session_id, session_cwd(path)):
        return 409, "This session is locked"
    _official_delete(session_id)
    if os.path.isdir(path):
        shutil.rmtree(path)
    _drop_index_row(session_id)
    if os.path.isdir(path):
        return 500, "Could not delete session"
    clear_session_lock(session_id)
    clear_pin(session_id)
    return 204, None


def _post_error(path: str) -> str:
    if path == "/api/sessions/delete":
        return "Choose at least one session"
    if path == "/api/sessions/move":
        return "Unknown folder"
    if path == "/api/pins":
        return "Unknown pin"
    return "Unknown lock"


MAX_BATCH = 200


def find_group_dir(cwd: str) -> str | None:
    root = sessions_root()
    if not os.path.isdir(root):
        return None
    preferred = urllib.parse.quote(cwd, safe="")
    matches: list[tuple[bool, str, str]] = []
    try:
        names = os.listdir(root)
    except OSError:
        return None
    for name in names:
        group_path = os.path.join(root, name)
        if not os.path.isdir(group_path):
            continue
        if group_cwd(group_path, name) != cwd:
            continue
        matches.append((name != preferred, name, group_path))
    if not matches:
        return None
    matches.sort()
    chosen = matches[0][2]
    real_root = os.path.realpath(root)
    real_group = os.path.realpath(chosen)
    if not real_group.startswith(real_root + os.sep):
        return None
    return chosen


def memory_workspaces_root() -> str:
    return os.environ.get("GROK_MEMORY_WORKSPACES") or os.path.expanduser(
        "~/.grok/memory-v2/workspaces"
    )


def memory_workspace_for(cwd: str) -> str | None:
    root = memory_workspaces_root()
    if not os.path.isdir(root):
        return None
    label = os.path.basename(cwd.rstrip(os.sep))
    if not label:
        return None
    prefix = label + "-"
    matches: list[str] = []
    try:
        names = os.listdir(root)
    except OSError:
        return None
    for name in names:
        if not name.startswith(prefix):
            continue
        suffix = name[len(prefix) :]
        if re.fullmatch(r"[0-9a-f]{8}", suffix) and os.path.isdir(os.path.join(root, name)):
            matches.append(name)
    if len(matches) != 1:
        return None
    return os.path.join(root, matches[0])


def _write_json(path: str, payload: dict) -> None:
    temporary = path + ".tmp"
    with open(temporary, "w", encoding="utf-8") as handle:
        json.dump(payload, handle, indent=2)
        handle.write("\n")
    os.replace(temporary, path)


def _point_session_at(session_dir: str, cwd: str) -> None:
    summary_path = os.path.join(session_dir, "summary.json")
    if os.path.isfile(summary_path):
        with open(summary_path, encoding="utf-8") as handle:
            summary = json.load(handle)
        if not isinstance(summary, dict):
            summary = {}
        info = summary.get("info") if isinstance(summary.get("info"), dict) else {}
        info["cwd"] = cwd
        summary["info"] = info
        _write_json(summary_path, summary)
    context_path = os.path.join(session_dir, "prompt_context.json")
    if not os.path.isfile(context_path):
        return
    with open(context_path, encoding="utf-8") as handle:
        context = json.load(handle)
    if not isinstance(context, dict):
        return
    context["working_directory"] = cwd
    workspace = memory_workspace_for(cwd)
    if workspace:
        context["memory_workspace_path"] = workspace
    _write_json(context_path, context)


def _set_index_cwd(database: str | None, table: str, session_id: str, cwd: str) -> None:
    if not database or not os.path.isfile(database):
        return
    try:
        connection = sqlite3.connect(database, timeout=5)
    except sqlite3.Error:
        return
    try:
        names = {row[0] for row in connection.execute("SELECT name FROM sqlite_master WHERE type='table'")}
        if table not in names:
            return
        connection.execute(f"UPDATE {table} SET cwd = ? WHERE session_id = ?", (cwd, session_id))
        connection.commit()
    except sqlite3.Error:
        return
    finally:
        connection.close()


def move_session(session_id: object, target: object) -> tuple[int, dict]:
    if not isinstance(session_id, str) or not ID_RE.fullmatch(session_id):
        return 400, {"error": "Invalid session id"}
    if not isinstance(target, str) or not target.startswith("/") or len(target) > 512:
        return 400, {"error": "Unknown folder"}
    path = find_session_dir(session_id)
    if path is None:
        return 404, {"error": "Session not found"}
    source = session_cwd(path)
    if source == target:
        return 400, {"error": "Session is already in that folder"}
    if target not in {group["cwd"] for group in list_groups()}:
        return 404, {"error": "Folder not found"}
    if session_id in live_ids():
        return 409, {"error": "This session is open in Grok"}
    if session_is_locked(session_id, source):
        return 409, {"error": "This session is locked"}
    destination_parent = find_group_dir(target)
    if destination_parent is None:
        return 404, {"error": "Folder not found"}
    destination = os.path.join(destination_parent, session_id)
    if os.path.exists(destination):
        return 409, {"error": "A session with that id is already in that folder"}
    shutil.move(path, destination)
    try:
        _point_session_at(destination, target)
    except (OSError, json.JSONDecodeError):
        return 500, {"error": "Could not update the session folder"}
    _set_index_cwd(os.path.join(sessions_root(), "session_search.sqlite"), "session_docs", session_id, target)
    dashboard = os.environ.get("GROK_DASHBOARD_DB")
    if not dashboard and using_real_store():
        dashboard = os.path.expanduser("~/.grok/dashboard/workspace.db")
    _set_index_cwd(dashboard, "members", session_id, target)
    return 200, {"ok": True, "cwd": target}


def delete_sessions(ids: object) -> tuple[int, dict]:
    if not isinstance(ids, list) or not ids:
        return 400, {"error": "Choose at least one session"}
    if len(ids) > MAX_BATCH:
        return 400, {"error": "Too many sessions"}
    deleted: list[str] = []
    failed: list[dict] = []
    seen: set[str] = set()
    for raw in ids:
        if not isinstance(raw, str) or raw in seen:
            if not isinstance(raw, str):
                failed.append({"id": "", "error": "Invalid session id"})
            continue
        seen.add(raw)
        status, message = delete_session(raw)
        if status == 204:
            deleted.append(raw)
        else:
            failed.append({"id": raw, "error": message or "Could not delete session"})
    return 200, {"deleted": deleted, "failed": failed}


class Handler(BaseHTTPRequestHandler):
    server_version = "GrokManager"

    def version_string(self) -> str:
        return f"GrokManager/{app_version()}"

    def do_GET(self) -> None:  # noqa: N802
        path = urllib.parse.urlparse(self.path).path
        if path == "/api/sessions":
            self._send_json(200, {"groups": list_groups(), "pinned": load_pins()})
            return
        if path in ("/", "/index.html"):
            self._send_file("index.html", "text/html; charset=utf-8")
            return
        self._send_json(404, {"error": "Not found"})

    def do_POST(self) -> None:  # noqa: N802
        path = urllib.parse.urlparse(self.path).path
        try:
            length = int(self.headers.get("Content-Length") or "0")
        except ValueError:
            length = 0
        if length <= 0 or length > 65536:
            self._send_json(400, {"error": _post_error(path)})
            return
        try:
            payload = json.loads(self.rfile.read(length).decode("utf-8"))
        except (UnicodeError, json.JSONDecodeError):
            self._send_json(400, {"error": _post_error(path)})
            return
        if not isinstance(payload, dict):
            self._send_json(400, {"error": "Unknown lock"})
            return
        if path == "/api/sessions/delete":
            status, body = delete_sessions(payload.get("ids"))
            self._send_json(status, body)
            return
        if path == "/api/locks":
            status, body = set_lock(payload.get("kind"), payload.get("id"), payload.get("locked"))
            self._send_json(status, body)
            return
        if path == "/api/pins":
            if "order" in payload:
                status, body = reorder_pins(payload.get("order"))
            else:
                status, body = set_pin(payload.get("id"), payload.get("pinned"))
            self._send_json(status, body)
            return
        if path == "/api/sessions/move":
            status, body = move_session(payload.get("id"), payload.get("cwd"))
            self._send_json(status, body)
            return
        self._send_json(404, {"error": "Not found"})

    def do_DELETE(self) -> None:  # noqa: N802
        path = urllib.parse.urlparse(self.path).path
        prefix = "/api/sessions/"
        if not path.startswith(prefix) or path.count("/") != 3:
            self._send_json(404, {"error": "Not found"})
            return
        session_id = urllib.parse.unquote(path[len(prefix) :])
        status, message = delete_session(session_id)
        if status == 204:
            self._send_empty(204)
            return
        self._send_json(status, {"error": message or "Could not delete session"})

    def log_message(self, fmt: str, *args) -> None:
        sys.stderr.write("%s - %s\n" % (self.log_date_time_string(), fmt % args))

    def _send_json(self, status: int, payload: dict) -> None:
        body = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Cache-Control", "no-store")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _send_empty(self, status: int) -> None:
        self.send_response(status)
        self.send_header("Content-Length", "0")
        self.end_headers()

    def _send_file(self, name: str, content_type: str) -> None:
        path = os.path.join(STATIC_DIR, name)
        try:
            with open(path, "rb") as handle:
                body = handle.read()
        except OSError:
            self._send_json(404, {"error": "Not found"})
            return
        if name == "index.html":
            body = body.replace(b"__VERSION__", app_version().encode("utf-8"))
        self.send_response(200)
        self.send_header("Content-Type", content_type)
        self.send_header("Cache-Control", "no-store")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


def main() -> None:
    server = ThreadingHTTPServer((HOST, PORT), Handler)
    print(f"Grok Manager listening on http://{HOST}:{PORT}", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()


if __name__ == "__main__":
    main()
