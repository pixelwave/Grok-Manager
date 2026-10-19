#!/usr/bin/env python3
"""Checks for the session scanner and delete path. Uses a temp tree only."""

import json
import os
import shutil
import sqlite3
import tempfile
import unittest
import urllib.parse

import server


def summary(session_id, title, updated, cwd, model="grok-4.7"):
    return {
        "info": {"id": session_id, "cwd": cwd},
        "generated_title": title,
        "updated_at": updated,
        "created_at": "2026-01-01T00:00:00Z",
        "current_model_id": model,
    }


class ScannerTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp(prefix="grok-manager-")
        self.active = os.path.join(self.tmp, "active.json")
        os.environ["GROK_SESSIONS_DIR"] = os.path.join(self.tmp, "sessions")
        os.environ["GROK_ACTIVE_SESSIONS"] = self.active
        os.environ["GROK_LOCKS_PATH"] = os.path.join(self.tmp, "locks.json")
        os.environ["GROK_PINS_PATH"] = os.path.join(self.tmp, "pins.json")
        os.environ["GROK_MACHINES_PATH"] = os.path.join(self.tmp, "machines.json")
        os.makedirs(os.environ["GROK_SESSIONS_DIR"])
        with open(self.active, "w", encoding="utf-8") as handle:
            json.dump([], handle)

    def tearDown(self):
        shutil.rmtree(self.tmp)
        os.environ.pop("GROK_SESSIONS_DIR", None)
        os.environ.pop("GROK_ACTIVE_SESSIONS", None)
        os.environ.pop("GROK_LOCKS_PATH", None)
        os.environ.pop("GROK_PINS_PATH", None)
        os.environ.pop("GROK_MACHINES_PATH", None)
        os.environ.pop("GROK_MEMORY_WORKSPACES", None)
        os.environ.pop("GROK_DASHBOARD_DB", None)

    def add_session(self, folder, session_id, title, updated, cwd=None):
        root = os.environ["GROK_SESSIONS_DIR"]
        encoded = urllib.parse.quote(folder, safe="")
        path = os.path.join(root, encoded, session_id)
        os.makedirs(path)
        payload = summary(session_id, title, updated, cwd or folder)
        with open(os.path.join(path, "summary.json"), "w", encoding="utf-8") as handle:
            json.dump(payload, handle)
        history = os.path.join(root, encoded, "prompt_history.jsonl")
        if not os.path.exists(history):
            with open(history, "w", encoding="utf-8") as handle:
                handle.write("keep\n")
        return path

    def test_groups_sorted_and_sessions_newest_first(self):
        self.add_session("/root/projects/plannr", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1", "Older", "2026-02-01T00:00:00Z")
        self.add_session("/root/projects/plannr", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa2", "Newer", "2026-03-01T00:00:00Z")
        self.add_session("/root/projects/netadmin", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb1", "Net", "2026-04-01T00:00:00Z")
        groups = server.list_groups()
        self.assertEqual([group["cwd"] for group in groups], ["/root/projects/netadmin", "/root/projects/plannr"])
        self.assertEqual(groups[0]["label"], "netadmin")
        self.assertEqual([row["title"] for row in groups[1]["sessions"]], ["Newer", "Older"])
        self.assertEqual(groups[1]["sessions"][0]["model"], "grok-4.7")
        self.assertFalse(groups[1]["sessions"][0]["live"])

    def test_extra_project_sessions_are_listed_and_deleted(self):
        folder = os.path.join("/root", "projects", "sample")
        secret = "cccccccc-cccc-cccc-cccc-ccccccccccc1"
        path = self.add_session(folder, secret, "Shown", "2026-05-01T00:00:00Z")
        self.add_session("/root/projects/plannr", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1", "Visible", "2026-05-02T00:00:00Z")
        groups = {group["cwd"]: group for group in server.list_groups()}
        self.assertIn(folder, groups)
        self.assertFalse(groups[folder]["locked"])
        self.assertEqual(groups[folder]["sessions"][0]["id"], secret)
        self.assertFalse(groups[folder]["sessions"][0]["locked"])
        self.assertEqual(server.delete_session(secret), (204, None))
        self.assertFalse(os.path.exists(path))

    def test_cwd_marker_for_extra_project_is_listed(self):
        folder = os.path.join("/root", "projects", "sample")
        root = os.environ["GROK_SESSIONS_DIR"]
        session_id = "dddddddd-dddd-dddd-dddd-ddddddddddd1"
        group = os.path.join(root, "hashed-name")
        session = os.path.join(group, session_id)
        os.makedirs(session)
        with open(os.path.join(group, ".cwd"), "w", encoding="utf-8") as handle:
            handle.write(folder + "\n")
        with open(os.path.join(session, "summary.json"), "w", encoding="utf-8") as handle:
            json.dump(summary(session_id, "Shown", "2026-01-01T00:00:00Z", folder), handle)
        groups = server.list_groups()
        self.assertEqual([group["cwd"] for group in groups], [folder])
        self.assertEqual(server.delete_session(session_id), (204, None))
        self.assertFalse(os.path.exists(session))

    def test_live_session_is_not_deleted(self):
        session_id = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeee1"
        path = self.add_session("/root/projects/plannr", session_id, "Live", "2026-06-01T00:00:00Z")
        with open(self.active, "w", encoding="utf-8") as handle:
            json.dump([{"session_id": session_id, "pid": os.getpid(), "cwd": "/root/projects/plannr"}], handle)
        groups = server.list_groups()
        self.assertTrue(groups[0]["sessions"][0]["live"])
        status, message = server.delete_session(session_id)
        self.assertEqual((status, message), (409, "This session is open in Grok"))
        self.assertTrue(os.path.isdir(path))

    def test_delete_removes_only_that_session(self):
        keep_id = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1"
        drop_id = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa2"
        keep = self.add_session("/root/projects/plannr", keep_id, "Keep", "2026-01-01T00:00:00Z")
        drop = self.add_session("/root/projects/plannr", drop_id, "Drop", "2026-01-02T00:00:00Z")
        parent = os.path.dirname(keep)
        history = os.path.join(parent, "prompt_history.jsonl")
        database = os.path.join(os.environ["GROK_SESSIONS_DIR"], "session_search.sqlite")
        connection = sqlite3.connect(database)
        connection.execute(
            "CREATE TABLE session_docs (session_id TEXT, cwd TEXT, updated_at TEXT, title TEXT, content TEXT, content_hash TEXT)"
        )
        connection.execute(
            "INSERT INTO session_docs VALUES (?, ?, ?, ?, ?, ?)",
            (drop_id, "/root/projects/plannr", "", "Drop", "transcript text", "hash"),
        )
        connection.commit()
        connection.close()

        status, message = server.delete_session(drop_id)
        self.assertEqual((status, message), (204, None))
        self.assertFalse(os.path.exists(drop))
        self.assertTrue(os.path.isdir(keep))
        with open(history, encoding="utf-8") as handle:
            self.assertEqual(handle.read(), "keep\n")
        connection = sqlite3.connect(database)
        left = connection.execute("SELECT session_id, content FROM session_docs").fetchall()
        connection.close()
        self.assertEqual(left, [])

    def test_bad_id_rejected(self):
        self.assertEqual(server.delete_session("../etc")[0], 400)
        self.assertEqual(server.delete_session("not-a-uuid")[0], 400)

    def test_delete_many_keeps_the_live_session(self):
        keep_id = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1"
        drop_id = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa2"
        live_id = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa3"
        keep = self.add_session("/root/projects/plannr", keep_id, "Keep", "2026-01-01T00:00:00Z")
        drop = self.add_session("/root/projects/plannr", drop_id, "Drop", "2026-01-02T00:00:00Z")
        live = self.add_session("/root/projects/plannr", live_id, "Live", "2026-01-03T00:00:00Z")
        with open(self.active, "w", encoding="utf-8") as handle:
            json.dump([{"session_id": live_id, "pid": os.getpid(), "cwd": "/root/projects/plannr"}], handle)
        status, body = server.delete_sessions([drop_id, live_id, drop_id, "nope"])
        self.assertEqual(status, 200)
        self.assertEqual(body["deleted"], [drop_id])
        self.assertEqual([row["id"] for row in body["failed"]], [live_id, "nope"])
        self.assertFalse(os.path.exists(drop))
        self.assertTrue(os.path.isdir(keep))
        self.assertTrue(os.path.isdir(live))
        self.assertEqual(server.delete_sessions([])[0], 400)

    def test_session_lock_blocks_delete_until_unlocked(self):
        session_id = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1"
        path = self.add_session("/root/projects/plannr", session_id, "Keep", "2026-01-01T00:00:00Z")
        status, body = server.set_lock("session", session_id, True)
        self.assertEqual(status, 200)
        self.assertTrue(body["ok"])
        groups = server.list_groups()
        self.assertTrue(groups[0]["sessions"][0]["locked"])
        self.assertFalse(groups[0]["locked"])
        blocked = server.delete_session(session_id)
        self.assertEqual(blocked, (409, "This session is locked"))
        self.assertTrue(os.path.isdir(path))
        self.assertEqual(server.set_lock("session", session_id, False)[0], 200)
        self.assertEqual(server.delete_session(session_id)[0], 204)
        self.assertFalse(os.path.exists(path))

    def test_folder_lock_blocks_that_folder_only(self):
        locked_id = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1"
        free_id = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb1"
        locked_path = self.add_session("/root/projects/plannr", locked_id, "Inside", "2026-01-01T00:00:00Z")
        free_path = self.add_session("/root/projects/untl", free_id, "Outside", "2026-01-02T00:00:00Z")
        self.assertEqual(server.set_lock("folder", "/root/projects/plannr", True)[0], 200)
        sample = os.path.join("/root", "projects", "sample")
        sample_id = "cccccccc-cccc-cccc-cccc-ccccccccccc1"
        self.add_session(sample, sample_id, "Sample", "2026-01-03T00:00:00Z")
        self.assertEqual(server.set_lock("folder", sample, True)[0], 200)
        self.assertEqual(server.set_lock("folder", sample, False)[0], 200)
        groups = {group["cwd"]: group for group in server.list_groups()}
        self.assertTrue(groups["/root/projects/plannr"]["locked"])
        self.assertFalse(groups["/root/projects/untl"]["locked"])
        self.assertFalse(groups["/root/projects/plannr"]["sessions"][0]["locked"])
        batch = server.delete_sessions([locked_id, free_id])
        self.assertEqual(batch[1]["deleted"], [free_id])
        self.assertEqual(batch[1]["failed"][0]["error"], "This session is locked")
        self.assertTrue(os.path.isdir(locked_path))
        self.assertFalse(os.path.exists(free_path))
        self.assertEqual(server.set_lock("folder", "/root/projects/missing", True)[0], 404)

    def test_move_session_updates_folder_and_leaves_transcript(self):
        session_id = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1"
        sibling_id = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa2"
        source = self.add_session("/root/projects/plannr", session_id, "Mobile", "2026-03-01T00:00:00Z")
        sibling = self.add_session("/root/projects/plannr", sibling_id, "Other", "2026-03-02T00:00:00Z")
        self.add_session("/root/projects/netadmin", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb1", "Net", "2026-04-01T00:00:00Z")
        history = os.path.join(os.path.dirname(source), "prompt_history.jsonl")
        transcript = os.path.join(source, "chat_history.jsonl")
        with open(transcript, "w", encoding="utf-8") as handle:
            handle.write("old /root/projects/plannr mention\n")
        context = {
            "working_directory": "/root/projects/plannr",
            "memory_workspace_path": "/tmp/old-workspace",
            "keep": "yes",
        }
        with open(os.path.join(source, "prompt_context.json"), "w", encoding="utf-8") as handle:
            json.dump(context, handle)
        workspaces = os.path.join(self.tmp, "workspaces")
        os.makedirs(os.path.join(workspaces, "netadmin-abc12345"))
        os.environ["GROK_MEMORY_WORKSPACES"] = workspaces
        search = os.path.join(os.environ["GROK_SESSIONS_DIR"], "session_search.sqlite")
        dashboard = os.path.join(self.tmp, "workspace.db")
        os.environ["GROK_DASHBOARD_DB"] = dashboard
        for path, table in ((search, "session_docs"), (dashboard, "members")):
            connection = sqlite3.connect(path)
            connection.execute(
                f"CREATE TABLE {table} (session_id TEXT, cwd TEXT, title TEXT, content TEXT)"
            )
            connection.execute(
                f"INSERT INTO {table} VALUES (?, ?, ?, ?)",
                (session_id, "/root/projects/plannr", "Mobile", "leave me"),
            )
            connection.commit()
            connection.close()

        status, body = server.move_session(session_id, "/root/projects/netadmin")
        self.assertEqual(status, 200)
        self.assertEqual(body["cwd"], "/root/projects/netadmin")
        moved = os.path.join(os.environ["GROK_SESSIONS_DIR"], urllib.parse.quote("/root/projects/netadmin", safe=""), session_id)
        self.assertTrue(os.path.isdir(moved))
        self.assertFalse(os.path.exists(source))
        self.assertTrue(os.path.isdir(sibling))
        with open(history, encoding="utf-8") as handle:
            self.assertEqual(handle.read(), "keep\n")
        with open(os.path.join(moved, "chat_history.jsonl"), encoding="utf-8") as handle:
            self.assertEqual(handle.read(), "old /root/projects/plannr mention\n")
        with open(os.path.join(moved, "summary.json"), encoding="utf-8") as handle:
            summary_body = json.load(handle)
        self.assertEqual(summary_body["info"]["cwd"], "/root/projects/netadmin")
        with open(os.path.join(moved, "prompt_context.json"), encoding="utf-8") as handle:
            prompt = json.load(handle)
        self.assertEqual(prompt["working_directory"], "/root/projects/netadmin")
        self.assertEqual(prompt["memory_workspace_path"], os.path.join(workspaces, "netadmin-abc12345"))
        self.assertEqual(prompt["keep"], "yes")
        for path, table in ((search, "session_docs"), (dashboard, "members")):
            connection = sqlite3.connect(path)
            row = connection.execute(f"SELECT cwd, content FROM {table} WHERE session_id = ?", (session_id,)).fetchone()
            connection.close()
            self.assertEqual(row, ("/root/projects/netadmin", "leave me"))
        groups = {group["cwd"]: [row["id"] for row in group["sessions"]] for group in server.list_groups()}
        self.assertIn(session_id, groups["/root/projects/netadmin"])
        self.assertNotIn(session_id, groups["/root/projects/plannr"])

        with open(self.active, "w", encoding="utf-8") as handle:
            json.dump([{"session_id": sibling_id, "pid": os.getpid(), "cwd": "/root/projects/plannr"}], handle)
        self.assertEqual(server.move_session(sibling_id, "/root/projects/netadmin")[0], 409)
        self.assertTrue(os.path.isdir(sibling))
        self.assertEqual(server.move_session(session_id, "/root/projects/missing")[0], 404)
        self.assertEqual(server.set_lock("session", session_id, True)[0], 200)
        self.assertEqual(server.move_session(session_id, "/root/projects/plannr")[0], 409)
        self.assertTrue(os.path.isdir(moved))

    def test_pin_orders_newest_first_and_delete_clears_it(self):
        first = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1"
        second = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb1"
        self.add_session("/root/projects/plannr", first, "Mobile", "2026-03-01T00:00:00Z")
        self.add_session("/root/projects/netadmin", second, "Net", "2026-04-01T00:00:00Z")
        self.assertEqual(server.set_pin(first, True)[0], 200)
        self.assertEqual(server.set_pin(second, True)[0], 200)
        self.assertEqual(server.load_pins(), [second, first])
        self.assertEqual(server.reorder_pins([first, second])[0], 200)
        self.assertEqual(server.load_pins(), [first, second])
        self.assertEqual(server.reorder_pins([first])[0], 409)
        self.assertEqual(server.reorder_pins([first, first])[0], 400)
        groups = {group["cwd"]: group["sessions"][0] for group in server.list_groups()}
        self.assertTrue(groups["/root/projects/plannr"]["pinned"])
        self.assertTrue(groups["/root/projects/netadmin"]["pinned"])
        self.assertEqual(server.set_pin("cccccccc-cccc-cccc-cccc-ccccccccccc1", True)[0], 404)
        self.assertEqual(server.set_pin("not-an-id", True), (400, {"error": "Invalid session id"}))
        self.assertEqual(server.move_session(first, "/root/projects/netadmin")[0], 200)
        self.assertEqual(server.load_pins(), [first, second])
        self.assertEqual(server.delete_session(second)[0], 204)
        self.assertEqual(server.load_pins(), [first])
        self.assertEqual(server.set_pin(first, False)[0], 200)
        self.assertEqual(server.load_pins(), [])

    def test_app_version_is_numbered(self):
        self.assertRegex(server.app_version(), r"^\d+\.\d+$")

    def test_added_machines_persist_and_reject_bad_urls(self):
        self.assertEqual(server.load_machines(), [])
        status, body = server.add_machine("Office", "https://office.example:8090/")
        self.assertEqual(status, 200)
        self.assertEqual(body["machines"], [{"name": "Office", "url": "https://office.example:8090/"}])
        self.assertEqual(server.load_machines(), body["machines"])
        self.assertEqual(server.add_machine("office", "https://other.example/")[0], 409)
        self.assertEqual(server.add_machine("  ", "https://office.example/")[0], 400)
        self.assertEqual(server.add_machine("Lab", "javascript:alert(1)")[0], 400)
        self.assertEqual(server.add_machine("Lab", "http://lab.example/")[0], 200)
        removed = server.remove_machine("Office")
        self.assertEqual(removed[0], 200)
        self.assertEqual([item["name"] for item in removed[1]["machines"]], ["Lab"])
        self.assertEqual(server.remove_machine("Office")[0], 404)


if __name__ == "__main__":
    unittest.main()
