package app

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func setup(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("GROK_SESSIONS_DIR", filepath.Join(dir, "sessions"))
	t.Setenv("GROK_ACTIVE_SESSIONS", filepath.Join(dir, "active.json"))
	t.Setenv("GROK_LOCKS_PATH", filepath.Join(dir, "locks.json"))
	t.Setenv("GROK_PINS_PATH", filepath.Join(dir, "pins.json"))
	t.Setenv("GROK_MACHINES_PATH", filepath.Join(dir, "machines.json"))
	t.Setenv("GROK_SETTINGS_PATH", filepath.Join(dir, "settings.json"))
	if err := os.MkdirAll(os.Getenv("GROK_SESSIONS_DIR"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("GROK_ACTIVE_SESSIONS"), []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func addSession(t *testing.T, folder, id, title, updated string) string {
	t.Helper()
	group := filepath.Join(os.Getenv("GROK_SESSIONS_DIR"), pyQuote(folder))
	path := filepath.Join(group, id)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{
		"info":             map[string]any{"id": id, "cwd": folder},
		"generated_title":  title,
		"updated_at":       updated,
		"created_at":       "2026-01-01T00:00:00Z",
		"current_model_id": "grok-4.7",
	}
	raw, _ := json.Marshal(body)
	if err := os.WriteFile(filepath.Join(path, "summary.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	history := filepath.Join(group, "prompt_history.jsonl")
	if _, err := os.Stat(history); err != nil {
		_ = os.WriteFile(history, []byte("keep\n"), 0o644)
	}
	return path
}

func TestGroupsSorted(t *testing.T) {
	setup(t)
	addSession(t, "/root/projects/plannr", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1", "Older", "2026-02-01T00:00:00Z")
	addSession(t, "/root/projects/plannr", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa2", "Newer", "2026-03-01T00:00:00Z")
	addSession(t, "/root/projects/netadmin", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb1", "Net", "2026-04-01T00:00:00Z")
	groups := ListGroups()
	if groups[0].Cwd != "/root/projects/netadmin" || groups[0].Label != "netadmin" {
		t.Fatalf("groups %#v", groups)
	}
	if groups[1].Sessions[0].Title != "Newer" || groups[1].Sessions[0].Model == nil || *groups[1].Sessions[0].Model != "grok-4.7" || groups[1].Sessions[0].Live {
		t.Fatalf("session %#v", groups[1].Sessions[0])
	}
}

func TestDeleteAndMarker(t *testing.T) {
	setup(t)
	folder := filepath.Join("/root", "projects", "sample")
	id := "cccccccc-cccc-cccc-cccc-ccccccccccc1"
	path := addSession(t, folder, id, "Shown", "2026-05-01T00:00:00Z")
	if status, message := DeleteSession(id); status != 204 || message != "" || exists(path) {
		t.Fatalf("delete %d %s", status, message)
	}
	markerID := "dddddddd-dddd-dddd-dddd-ddddddddddd1"
	group := filepath.Join(os.Getenv("GROK_SESSIONS_DIR"), "hashed-name")
	session := filepath.Join(group, markerID)
	_ = os.MkdirAll(session, 0o755)
	_ = os.WriteFile(filepath.Join(group, ".cwd"), []byte(folder+"\n"), 0o644)
	raw, _ := json.Marshal(map[string]any{"info": map[string]any{"id": markerID, "cwd": folder}, "generated_title": "Shown", "updated_at": "2026-01-01T00:00:00Z"})
	_ = os.WriteFile(filepath.Join(session, "summary.json"), raw, 0o644)
	groups := ListGroups()
	found := false
	for _, group := range groups {
		if group.Cwd == folder {
			found = true
		}
	}
	if !found {
		t.Fatal("marker cwd missing")
	}
	if status, _ := DeleteSession(markerID); status != 204 || exists(session) {
		t.Fatal("marker delete")
	}
}

func TestLiveAndBatchAndLock(t *testing.T) {
	setup(t)
	liveID := "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeee1"
	path := addSession(t, "/root/projects/plannr", liveID, "Live", "2026-06-01T00:00:00Z")
	raw, _ := json.Marshal([]map[string]any{{"session_id": liveID, "pid": os.Getpid(), "cwd": "/root/projects/plannr"}})
	_ = os.WriteFile(os.Getenv("GROK_ACTIVE_SESSIONS"), raw, 0o644)
	if !ListGroups()[0].Sessions[0].Live {
		t.Fatal("expected live")
	}
	if status, message := DeleteSession(liveID); status != 409 || message != "This session is open in Grok" || !exists(path) {
		t.Fatalf("live delete %d %s", status, message)
	}
	_ = os.WriteFile(os.Getenv("GROK_ACTIVE_SESSIONS"), []byte("[]"), 0o644)
	if status, _ := DeleteSession("../etc"); status != 400 {
		t.Fatal(status)
	}
	drop := addSession(t, "/root/projects/plannr", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa2", "Drop", "2026-01-02T00:00:00Z")
	keep := addSession(t, "/root/projects/plannr", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1", "Keep", "2026-01-01T00:00:00Z")
	database := filepath.Join(os.Getenv("GROK_SESSIONS_DIR"), "session_search.sqlite")
	db, err := sql.Open("sqlite", database)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = db.Exec(`CREATE TABLE session_docs (session_id TEXT, cwd TEXT, content TEXT)`)
	_, _ = db.Exec(`INSERT INTO session_docs VALUES (?, ?, ?)`, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa2", "/root/projects/plannr", "transcript text")
	db.Close()
	if status, _ := DeleteSession("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa2"); status != 204 || exists(drop) || !exists(keep) {
		t.Fatal("delete one")
	}
	db, _ = sql.Open("sqlite", database)
	var count int
	_ = db.QueryRow(`SELECT count(*) FROM session_docs`).Scan(&count)
	db.Close()
	if count != 0 {
		t.Fatal("index row remains")
	}
	locked := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1"
	if status, _ := SetLock("session", locked, true); status != 200 {
		t.Fatal(status)
	}
	if status, message := DeleteSession(locked); status != 409 || message != "This session is locked" {
		t.Fatal(message)
	}
	if status, _ := SetLock("session", locked, false); status != 200 {
		t.Fatal(status)
	}
	if status, _ := DeleteSession(locked); status != 204 {
		t.Fatal(status)
	}
}

func TestMovePinAndMachines(t *testing.T) {
	setup(t)
	id := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1"
	source := addSession(t, "/root/projects/plannr", id, "Mobile", "2026-03-01T00:00:00Z")
	addSession(t, "/root/projects/netadmin", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb1", "Net", "2026-04-01T00:00:00Z")
	_ = os.WriteFile(filepath.Join(source, "chat_history.jsonl"), []byte("old /root/projects/plannr mention\n"), 0o644)
	context := map[string]any{"working_directory": "/root/projects/plannr", "memory_workspace_path": "/tmp/old-workspace", "keep": "yes"}
	raw, _ := json.Marshal(context)
	_ = os.WriteFile(filepath.Join(source, "prompt_context.json"), raw, 0o644)
	workspaces := filepath.Join(t.TempDir(), "workspaces")
	_ = os.MkdirAll(filepath.Join(workspaces, "netadmin-abc12345"), 0o755)
	t.Setenv("GROK_MEMORY_WORKSPACES", workspaces)
	search := filepath.Join(os.Getenv("GROK_SESSIONS_DIR"), "session_search.sqlite")
	dashboard := filepath.Join(t.TempDir(), "workspace.db")
	t.Setenv("GROK_DASHBOARD_DB", dashboard)
	for _, item := range []struct{ path, table string }{{search, "session_docs"}, {dashboard, "members"}} {
		db, err := sql.Open("sqlite", item.path)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = db.Exec(`CREATE TABLE ` + item.table + ` (session_id TEXT, cwd TEXT, content TEXT)`)
		_, _ = db.Exec(`INSERT INTO `+item.table+` VALUES (?, ?, ?)`, id, "/root/projects/plannr", "leave me")
		db.Close()
	}
	status, body := MoveSession(id, "/root/projects/netadmin")
	if status != 200 || body["cwd"] != "/root/projects/netadmin" || exists(source) {
		t.Fatalf("move %d %#v", status, body)
	}
	moved := filepath.Join(os.Getenv("GROK_SESSIONS_DIR"), pyQuote("/root/projects/netadmin"), id)
	transcript, _ := os.ReadFile(filepath.Join(moved, "chat_history.jsonl"))
	if string(transcript) != "old /root/projects/plannr mention\n" {
		t.Fatal(string(transcript))
	}
	var prompt map[string]any
	_ = readJSON(filepath.Join(moved, "prompt_context.json"), &prompt)
	if prompt["working_directory"] != "/root/projects/netadmin" || prompt["memory_workspace_path"] != filepath.Join(workspaces, "netadmin-abc12345") || prompt["keep"] != "yes" {
		t.Fatalf("%#v", prompt)
	}
	if status, _ := SetPin(id, true); status != 200 || len(LoadPins()) != 1 {
		t.Fatal("pin")
	}
	if status, _ := SetPin(id, false); status != 200 || len(LoadPins()) != 0 {
		t.Fatal("unpin")
	}
	if status, _ := AddMachine("Office", "https://office.example:8090/"); status != 200 {
		t.Fatal(status)
	}
	if status, _ := AddMachine("office", "https://other.example/"); status != 409 {
		t.Fatal(status)
	}
	if status, _ := AddMachine("  ", "https://office.example/"); status != 400 {
		t.Fatal(status)
	}
	if status, _ := AddMachine("Lab", "javascript:alert(1)"); status != 400 {
		t.Fatal(status)
	}
	if status, _ := AddMachine("Lab", "http://lab.example/"); status != 200 {
		t.Fatal(status)
	}
	status, body = RemoveMachine("Office")
	machines := body["machines"].([]Machine)
	if status != 200 || len(machines) != 1 || machines[0].Name != "Lab" {
		t.Fatalf("%#v", body)
	}
	if status, _ := RemoveMachine("Office"); status != 404 {
		t.Fatal(status)
	}
}

func TestVersionsAndAssets(t *testing.T) {
	setup(t)
	if !Newer("26.10.28.2", "26.10.28") || !Newer("26.10.28.3", "26.10.28.2") || !Newer("26.10.28.10", "26.10.28.3") {
		t.Fatal("same-day revision")
	}
	if !Newer("26.10.29", "26.10.28.3") || !Newer("v26.10.29", "26.10.28") {
		t.Fatal("next day")
	}
	if Newer("26.10.28", "26.10.28") || Newer("26.10.28.2", "26.10.29") || Newer("26.10.28-2", "26.10.28") || Newer("26.10.28.1", "26.10.28") {
		t.Fatal("not a newer release")
	}
	name, err := AssetName("linux", "amd64")
	if err != nil || name != "grok-manager_linux_amd64" {
		t.Fatal(name, err)
	}
	name, err = AssetName("windows", "arm64")
	if err != nil || name != "grok-manager_windows_arm64.exe" {
		t.Fatal(name, err)
	}
	if _, err := AssetName("linux", "arm"); err == nil {
		t.Fatal("expected unsupported")
	}
	if LoadPort() != 18090 {
		t.Fatal(LoadPort())
	}
	if err := SavePort(6969); err != nil || LoadPort() != 6969 {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
