package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	_ "modernc.org/sqlite"
)

var idRE = regexp.MustCompile(`^[0-9a-f-]{36}$`)

const maxBatch = 200

type Session struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	UpdatedAt string  `json:"updatedAt"`
	CreatedAt string  `json:"createdAt"`
	Model     *string `json:"model"`
	Live      bool    `json:"live"`
	Locked    bool    `json:"locked"`
	Pinned    bool    `json:"pinned"`
}

type Group struct {
	Cwd      string    `json:"cwd"`
	Label    string    `json:"label"`
	Locked   bool      `json:"locked"`
	Sessions []Session `json:"sessions"`
}

type Machine struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Failure struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

func sessionsRoot() string {
	if value := os.Getenv("GROK_SESSIONS_DIR"); value != "" {
		return value
	}
	return filepath.Join(homeDir(), ".grok", "sessions")
}

func activeSessionsPath() string {
	if value := os.Getenv("GROK_ACTIVE_SESSIONS"); value != "" {
		return value
	}
	return filepath.Join(homeDir(), ".grok", "active_sessions.json")
}

func memoryWorkspacesRoot() string {
	if value := os.Getenv("GROK_MEMORY_WORKSPACES"); value != "" {
		return value
	}
	return filepath.Join(homeDir(), ".grok", "memory-v2", "workspaces")
}

func pyQuote(value string) string {
	var b strings.Builder
	for i := 0; i < len(value); {
		r, size := utf8.DecodeRuneInString(value[i:])
		if size == 1 && ((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '.' || r == '-') {
			b.WriteRune(r)
		} else {
			for _, c := range []byte(value[i : i+size]) {
				b.WriteByte('%')
				const hex = "0123456789ABCDEF"
				b.WriteByte(hex[c>>4])
				b.WriteByte(hex[c&15])
			}
		}
		i += size
	}
	return b.String()
}

func pyUnquote(value string) string {
	var b strings.Builder
	for i := 0; i < len(value); {
		if value[i] == '%' && i+2 < len(value) {
			hi := unhex(value[i+1])
			lo := unhex(value[i+2])
			if hi >= 0 && lo >= 0 {
				b.WriteByte(byte(hi<<4 | lo))
				i += 3
				continue
			}
		}
		b.WriteByte(value[i])
		i++
	}
	return b.String()
}

func unhex(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	default:
		return -1
	}
}

func readJSON(path string, dest any) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dest)
}

func writeFileJSON(path string, payload any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, body, 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

type lockFile struct {
	Sessions []string `json:"sessions"`
	Folders  []string `json:"folders"`
}

func loadLocks() (map[string]struct{}, map[string]struct{}) {
	var file lockFile
	_ = readJSON(locksPath(), &file)
	sessions := map[string]struct{}{}
	folders := map[string]struct{}{}
	for _, item := range file.Sessions {
		if idRE.MatchString(item) {
			sessions[item] = struct{}{}
		}
	}
	for _, item := range file.Folders {
		if absolutePath(item) {
			folders[item] = struct{}{}
		}
	}
	return sessions, folders
}

func saveLocks(sessions, folders map[string]struct{}) error {
	file := lockFile{Sessions: mapKeys(sessions), Folders: mapKeys(folders)}
	sort.Strings(file.Sessions)
	sort.Strings(file.Folders)
	return writeFileJSON(locksPath(), file)
}

func mapKeys(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	return out
}

func absolutePath(value string) bool {
	if value == "" || len(value) > 512 {
		return false
	}
	return filepath.IsAbs(value)
}

type pinFile struct {
	Sessions []string `json:"sessions"`
}

func LoadPins() []string {
	var file pinFile
	if err := readJSON(pinsPath(), &file); err != nil {
		return []string{}
	}
	out := make([]string, 0, len(file.Sessions))
	seen := map[string]struct{}{}
	for _, item := range file.Sessions {
		if idRE.MatchString(item) {
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			out = append(out, item)
		}
	}
	return out
}

func savePins(pins []string) error {
	if pins == nil {
		pins = []string{}
	}
	return writeFileJSON(pinsPath(), pinFile{Sessions: pins})
}

type machineFile struct {
	Machines []Machine `json:"machines"`
}

func LoadMachines() []Machine {
	var file machineFile
	if err := readJSON(machinesPath(), &file); err != nil {
		return []Machine{}
	}
	out := make([]Machine, 0, len(file.Machines))
	seen := map[string]struct{}{}
	for _, item := range file.Machines {
		key := strings.ToLower(item.Name)
		if key == "" || item.URL == "" {
			continue
		}
		if _, ok := seen[key]; ok || machineURL(item.URL) == "" {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func saveMachines(machines []Machine) error {
	if machines == nil {
		machines = []Machine{}
	}
	return writeFileJSON(machinesPath(), machineFile{Machines: machines})
}

func machineURL(value string) string {
	text := strings.TrimSpace(value)
	if text == "" || len(text) > 500 {
		return ""
	}
	for _, r := range text {
		if r < 32 {
			return ""
		}
	}
	if !strings.HasPrefix(text, "http://") && !strings.HasPrefix(text, "https://") {
		return ""
	}
	rest := strings.TrimPrefix(strings.TrimPrefix(text, "https://"), "http://")
	if rest == "" || strings.HasPrefix(rest, "/") {
		return ""
	}
	return text
}

func AddMachine(name, url string) (int, map[string]any) {
	cleaned := strings.Join(strings.Fields(name), " ")
	if cleaned == "" || len(cleaned) > 80 {
		if strings.TrimSpace(name) == "" && name != "" || cleaned == "" {
			return 400, map[string]any{"error": "Enter a machine name"}
		}
	}
	if cleaned == "" || len(cleaned) > 80 {
		return 400, map[string]any{"error": "Enter a machine name"}
	}
	target := machineURL(url)
	if target == "" {
		return 400, map[string]any{"error": "Enter a URL that starts with http:// or https://"}
	}
	machines := LoadMachines()
	for _, item := range machines {
		if strings.EqualFold(item.Name, cleaned) {
			return 409, map[string]any{"error": "That machine is already listed"}
		}
	}
	if len(machines) >= 30 {
		return 400, map[string]any{"error": "Too many machines"}
	}
	machines = append(machines, Machine{Name: cleaned, URL: target})
	if err := saveMachines(machines); err != nil {
		return 500, map[string]any{"error": "Could not update machines."}
	}
	return 200, map[string]any{"machines": machines}
}

func RemoveMachine(name string) (int, map[string]any) {
	key := strings.ToLower(strings.Join(strings.Fields(name), " "))
	if key == "" {
		return 400, map[string]any{"error": "Unknown machine"}
	}
	machines := LoadMachines()
	kept := make([]Machine, 0, len(machines))
	for _, item := range machines {
		if strings.ToLower(item.Name) != key {
			kept = append(kept, item)
		}
	}
	if len(kept) == len(machines) {
		return 404, map[string]any{"error": "Machine not found"}
	}
	if err := saveMachines(kept); err != nil {
		return 500, map[string]any{"error": "Could not update machines."}
	}
	return 200, map[string]any{"machines": kept}
}

func groupCwd(groupPath, dirname string) string {
	cwd := pyUnquote(dirname)
	marker := filepath.Join(groupPath, ".cwd")
	body, err := os.ReadFile(marker)
	if err == nil {
		if recorded := strings.TrimSpace(string(body)); recorded != "" {
			cwd = recorded
		}
	}
	return cwd
}

func titleOf(summary map[string]any) string {
	for _, key := range []string{"generated_title", "session_summary"} {
		if value, ok := summary[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "Untitled"
}

func sessionRow(summaryPath string, live map[string]struct{}) *Session {
	var summary map[string]any
	if err := readJSON(summaryPath, &summary); err != nil || summary == nil {
		return nil
	}
	info, _ := summary["info"].(map[string]any)
	sid, _ := info["id"].(string)
	if sid == "" {
		sid = filepath.Base(filepath.Dir(summaryPath))
	}
	updated, _ := summary["updated_at"].(string)
	created, _ := summary["created_at"].(string)
	var model *string
	if value, ok := summary["current_model_id"].(string); ok {
		model = &value
	}
	_, isLive := live[sid]
	return &Session{
		ID: sid, Title: titleOf(summary), UpdatedAt: updated, CreatedAt: created,
		Model: model, Live: isLive,
	}
}

func pidNumber(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case json.Number:
		number, err := typed.Int64()
		if err != nil {
			return 0
		}
		return int(number)
	default:
		return 0
	}
}

func liveIDs() map[string]struct{} {
	var rows []map[string]any
	found := map[string]struct{}{}
	if err := readJSON(activeSessionsPath(), &rows); err != nil {
		return found
	}
	for _, row := range rows {
		sid, _ := row["session_id"].(string)
		pid := pidNumber(row["pid"])
		if sid != "" && pid > 0 && processAlive(pid) {
			found[sid] = struct{}{}
		}
	}
	return found
}

func ListGroups() []Group {
	root := sessionsRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		return []Group{}
	}
	live := liveIDs()
	sessionLocks, folderLocks := loadLocks()
	pinned := map[string]struct{}{}
	for _, id := range LoadPins() {
		pinned[id] = struct{}{}
	}
	grouped := map[string][]Session{}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		groupPath := filepath.Join(root, name)
		info, err := os.Stat(groupPath)
		if err != nil || !info.IsDir() {
			continue
		}
		cwd := groupCwd(groupPath, name)
		children, err := os.ReadDir(groupPath)
		if err != nil {
			continue
		}
		var rows []Session
		for _, child := range children {
			sessionPath := filepath.Join(groupPath, child.Name())
			summaryPath := filepath.Join(sessionPath, "summary.json")
			if stat, err := os.Stat(sessionPath); err != nil || !stat.IsDir() {
				continue
			}
			if _, err := os.Stat(summaryPath); err != nil {
				continue
			}
			row := sessionRow(summaryPath, live)
			if row == nil {
				continue
			}
			_, row.Locked = sessionLocks[row.ID]
			_, row.Pinned = pinned[row.ID]
			rows = append(rows, *row)
		}
		if len(rows) == 0 {
			continue
		}
		grouped[cwd] = append(grouped[cwd], rows...)
	}
	cwds := make([]string, 0, len(grouped))
	for cwd := range grouped {
		cwds = append(cwds, cwd)
	}
	sort.Strings(cwds)
	groups := make([]Group, 0, len(cwds))
	for _, cwd := range cwds {
		sessions := grouped[cwd]
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].UpdatedAt > sessions[j].UpdatedAt
		})
		label := cwd
		trimmed := strings.TrimRight(cwd, `/\`)
		if base := filepath.Base(trimmed); base != "" && base != "." {
			label = base
		}
		_, locked := folderLocks[cwd]
		groups = append(groups, Group{Cwd: cwd, Label: label, Locked: locked, Sessions: sessions})
	}
	return groups
}

func realPath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return resolved
}

func insideRoot(root, path string) bool {
	root = realPath(root)
	path = realPath(path)
	sep := string(os.PathSeparator)
	return path == root || strings.HasPrefix(path, root+sep)
}

func findSessionDir(sessionID string) string {
	if !idRE.MatchString(sessionID) {
		return ""
	}
	root := sessionsRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sessionPath := filepath.Join(root, entry.Name(), sessionID)
		info, err := os.Stat(sessionPath)
		if err != nil || !info.IsDir() {
			continue
		}
		if filepath.Base(realPath(sessionPath)) != sessionID {
			continue
		}
		if !insideRoot(root, sessionPath) {
			continue
		}
		return sessionPath
	}
	return ""
}

func sessionCwd(sessionPath string) string {
	groupPath := filepath.Dir(sessionPath)
	return groupCwd(groupPath, filepath.Base(groupPath))
}

func sessionLocked(sessionID, cwd string) bool {
	sessions, folders := loadLocks()
	if _, ok := sessions[sessionID]; ok {
		return true
	}
	_, ok := folders[cwd]
	return cwd != "" && ok
}

func usingRealStore() bool {
	real := realPath(filepath.Join(homeDir(), ".grok", "sessions"))
	current := realPath(sessionsRoot())
	return current == real
}

func officialDelete(sessionID string) {
	if !usingRealStore() {
		return
	}
	cmd := exec.Command("grok", "sessions", "delete", sessionID)
	done := make(chan struct{})
	go func() {
		_ = cmd.Run()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
}

func dropIndexRow(sessionID string) {
	database := filepath.Join(sessionsRoot(), "session_search.sqlite")
	setIndex(database, "session_docs", sessionID, "", true)
}

func setIndex(database, table, sessionID, cwd string, drop bool) {
	if database == "" {
		return
	}
	if _, err := os.Stat(database); err != nil {
		return
	}
	if table != "session_docs" && table != "members" {
		return
	}
	db, err := sql.Open("sqlite", database)
	if err != nil {
		return
	}
	defer db.Close()
	var name string
	err = db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
	if err != nil {
		return
	}
	if drop {
		_, _ = db.Exec(`DELETE FROM `+table+` WHERE session_id = ?`, sessionID)
		return
	}
	_, _ = db.Exec(`UPDATE `+table+` SET cwd = ? WHERE session_id = ?`, cwd, sessionID)
}

func clearSessionLock(sessionID string) {
	sessions, folders := loadLocks()
	if _, ok := sessions[sessionID]; !ok {
		return
	}
	delete(sessions, sessionID)
	_ = saveLocks(sessions, folders)
}

func clearPin(sessionID string) {
	pins := LoadPins()
	kept := make([]string, 0, len(pins))
	found := false
	for _, item := range pins {
		if item == sessionID {
			found = true
			continue
		}
		kept = append(kept, item)
	}
	if found {
		_ = savePins(kept)
	}
}

func DeleteSession(sessionID string) (int, string) {
	if !idRE.MatchString(sessionID) {
		return 400, "Invalid session id"
	}
	path := findSessionDir(sessionID)
	if path == "" {
		return 404, "Session not found"
	}
	if _, ok := liveIDs()[sessionID]; ok {
		return 409, "This session is open in Grok"
	}
	if sessionLocked(sessionID, sessionCwd(path)) {
		return 409, "This session is locked"
	}
	officialDelete(sessionID)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		_ = os.RemoveAll(path)
	}
	dropIndexRow(sessionID)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return 500, "Could not delete session"
	}
	clearSessionLock(sessionID)
	clearPin(sessionID)
	return 204, ""
}

func DeleteSessions(ids []any) (int, map[string]any) {
	if len(ids) == 0 {
		return 400, map[string]any{"error": "Choose at least one session"}
	}
	if len(ids) > maxBatch {
		return 400, map[string]any{"error": "Too many sessions"}
	}
	deleted := []string{}
	failed := []Failure{}
	seen := map[string]struct{}{}
	for _, raw := range ids {
		id, ok := raw.(string)
		if !ok {
			failed = append(failed, Failure{ID: "", Error: "Invalid session id"})
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		status, message := DeleteSession(id)
		if status == 204 {
			deleted = append(deleted, id)
			continue
		}
		if message == "" {
			message = "Could not delete session"
		}
		failed = append(failed, Failure{ID: id, Error: message})
	}
	return 200, map[string]any{"deleted": deleted, "failed": failed}
}

func knownCwds() map[string]struct{} {
	found := map[string]struct{}{}
	for _, group := range ListGroups() {
		found[group.Cwd] = struct{}{}
	}
	return found
}

func SetLock(kind, ident string, locked bool) (int, map[string]any) {
	if kind != "session" && kind != "folder" {
		return 400, map[string]any{"error": "Unknown lock"}
	}
	sessions, folders := loadLocks()
	if kind == "session" {
		if !idRE.MatchString(ident) {
			return 400, map[string]any{"error": "Invalid session id"}
		}
		if locked && findSessionDir(ident) == "" {
			return 404, map[string]any{"error": "Session not found"}
		}
		if locked {
			sessions[ident] = struct{}{}
		} else {
			delete(sessions, ident)
		}
	} else {
		if !absolutePath(ident) {
			return 400, map[string]any{"error": "Unknown folder"}
		}
		if locked {
			if _, ok := knownCwds()[ident]; !ok {
				return 404, map[string]any{"error": "Folder not found"}
			}
			folders[ident] = struct{}{}
		} else {
			delete(folders, ident)
		}
	}
	if err := saveLocks(sessions, folders); err != nil {
		return 500, map[string]any{"error": "Could not update locks."}
	}
	return 200, map[string]any{"ok": true}
}

func SetPin(ident string, pinned bool) (int, map[string]any) {
	if !idRE.MatchString(ident) {
		return 400, map[string]any{"error": "Invalid session id"}
	}
	if pinned && findSessionDir(ident) == "" {
		return 404, map[string]any{"error": "Session not found"}
	}
	pins := []string{}
	for _, item := range LoadPins() {
		if item != ident {
			pins = append(pins, item)
		}
	}
	if pinned {
		pins = append([]string{ident}, pins...)
	}
	if err := savePins(pins); err != nil {
		return 500, map[string]any{"error": "Could not update pins."}
	}
	return 200, map[string]any{"ok": true}
}

func ReorderPins(order []any) (int, map[string]any) {
	cleaned := make([]string, 0, len(order))
	seen := map[string]struct{}{}
	for _, item := range order {
		id, ok := item.(string)
		if !ok || !idRE.MatchString(id) {
			return 400, map[string]any{"error": "Invalid session id"}
		}
		if _, dup := seen[id]; dup {
			return 400, map[string]any{"error": "Invalid session id"}
		}
		seen[id] = struct{}{}
		cleaned = append(cleaned, id)
	}
	current := LoadPins()
	if len(cleaned) != len(current) {
		return 409, map[string]any{"error": "Pin list changed"}
	}
	have := map[string]struct{}{}
	for _, id := range current {
		have[id] = struct{}{}
	}
	for _, id := range cleaned {
		if _, ok := have[id]; !ok {
			return 409, map[string]any{"error": "Pin list changed"}
		}
	}
	if err := savePins(cleaned); err != nil {
		return 500, map[string]any{"error": "Could not update pins."}
	}
	return 200, map[string]any{"ok": true}
}

type groupMatch struct {
	notPreferred bool
	name         string
	path         string
}

func findGroupDir(cwd string) string {
	root := sessionsRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	preferred := pyQuote(cwd)
	var matches []groupMatch
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		groupPath := filepath.Join(root, entry.Name())
		if groupCwd(groupPath, entry.Name()) != cwd {
			continue
		}
		matches = append(matches, groupMatch{notPreferred: entry.Name() != preferred, name: entry.Name(), path: groupPath})
	}
	if len(matches) == 0 {
		return ""
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].notPreferred != matches[j].notPreferred {
			return !matches[i].notPreferred
		}
		return matches[i].name < matches[j].name
	})
	if !insideRoot(root, matches[0].path) {
		return ""
	}
	return matches[0].path
}

func memoryWorkspaceFor(cwd string) string {
	root := memoryWorkspacesRoot()
	label := filepath.Base(strings.TrimRight(cwd, `/\`))
	if label == "" || label == "." {
		return ""
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	prefix := label + "-"
	var matches []string
	hexRE := regexp.MustCompile(`^[0-9a-f]{8}$`)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) || !entry.IsDir() {
			continue
		}
		if hexRE.MatchString(name[len(prefix):]) {
			matches = append(matches, name)
		}
	}
	if len(matches) != 1 {
		return ""
	}
	return filepath.Join(root, matches[0])
}

func pointSessionAt(sessionDir, cwd string) error {
	summaryPath := filepath.Join(sessionDir, "summary.json")
	if _, err := os.Stat(summaryPath); err == nil {
		var summary map[string]any
		if err := readJSON(summaryPath, &summary); err != nil {
			return err
		}
		if summary == nil {
			summary = map[string]any{}
		}
		info, _ := summary["info"].(map[string]any)
		if info == nil {
			info = map[string]any{}
		}
		info["cwd"] = cwd
		summary["info"] = info
		if err := writeFileJSON(summaryPath, summary); err != nil {
			return err
		}
	}
	contextPath := filepath.Join(sessionDir, "prompt_context.json")
	if _, err := os.Stat(contextPath); err != nil {
		return nil
	}
	var context map[string]any
	if err := readJSON(contextPath, &context); err != nil {
		return err
	}
	if context == nil {
		return nil
	}
	context["working_directory"] = cwd
	if workspace := memoryWorkspaceFor(cwd); workspace != "" {
		context["memory_workspace_path"] = workspace
	}
	return writeFileJSON(contextPath, context)
}

func MoveSession(sessionID, target string) (int, map[string]any) {
	if !idRE.MatchString(sessionID) {
		return 400, map[string]any{"error": "Invalid session id"}
	}
	if !absolutePath(target) {
		return 400, map[string]any{"error": "Unknown folder"}
	}
	path := findSessionDir(sessionID)
	if path == "" {
		return 404, map[string]any{"error": "Session not found"}
	}
	source := sessionCwd(path)
	if source == target {
		return 400, map[string]any{"error": "Session is already in that folder"}
	}
	if _, ok := knownCwds()[target]; !ok {
		return 404, map[string]any{"error": "Folder not found"}
	}
	if _, ok := liveIDs()[sessionID]; ok {
		return 409, map[string]any{"error": "This session is open in Grok"}
	}
	if sessionLocked(sessionID, source) {
		return 409, map[string]any{"error": "This session is locked"}
	}
	parent := findGroupDir(target)
	if parent == "" {
		return 404, map[string]any{"error": "Folder not found"}
	}
	destination := filepath.Join(parent, sessionID)
	if _, err := os.Stat(destination); err == nil {
		return 409, map[string]any{"error": "A session with that id is already in that folder"}
	}
	if err := os.Rename(path, destination); err != nil {
		return 500, map[string]any{"error": "Could not update the session folder"}
	}
	if err := pointSessionAt(destination, target); err != nil {
		return 500, map[string]any{"error": "Could not update the session folder"}
	}
	setIndex(filepath.Join(sessionsRoot(), "session_search.sqlite"), "session_docs", sessionID, target, false)
	dashboard := os.Getenv("GROK_DASHBOARD_DB")
	if dashboard == "" && usingRealStore() {
		dashboard = filepath.Join(homeDir(), ".grok", "dashboard", "workspace.db")
	}
	setIndex(dashboard, "members", sessionID, target, false)
	return 200, map[string]any{"ok": true, "cwd": target}
}

// ErrEmpty is used when a caller passes a non-object where the API expects JSON.
var ErrEmpty = errors.New("empty")
