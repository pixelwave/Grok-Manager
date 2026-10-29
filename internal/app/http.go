package app

import (
	"encoding/json"
	"html"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/pixelwave/grok-manager/static"
)

type Server struct {
	Version string
	Port    int
}

func (server Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/sessions", server.sessions)
	mux.HandleFunc("/api/sessions/", server.session)
	mux.HandleFunc("/api/sessions/delete", server.deleteMany)
	mux.HandleFunc("/api/sessions/move", server.move)
	mux.HandleFunc("/api/locks", server.locks)
	mux.HandleFunc("/api/pins", server.pins)
	mux.HandleFunc("/api/machines", server.machines)
	mux.HandleFunc("/api/peers", server.peers)
	mux.HandleFunc("/api/update", server.update)
	mux.HandleFunc("/", server.page)
	return mux
}

func (server Server) page(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/index.html" {
		writeJSON(w, 404, map[string]any{"error": "Not found"})
		return
	}
	body := string(static.IndexHTML)
	body = strings.ReplaceAll(body, "__VERSION__", html.EscapeString(server.Version))
	body = strings.ReplaceAll(body, "__HOSTNAME__", html.EscapeString(MachineName()))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(200)
	_, _ = io.WriteString(w, body)
}

func (server Server) sessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 404, map[string]any{"error": "Not found"})
		return
	}
	writeJSON(w, 200, map[string]any{
		"groups":     ListGroups(),
		"pinned":     LoadPins(),
		"machines":   LoadMachines(),
		"discovered": BrowsePeers(server.Port),
		"update":     CheckUpdate(server.Version),
	})
}

func (server Server) peers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, 404, map[string]any{"error": "Not found"})
		return
	}
	writeJSON(w, 200, map[string]any{
		"discovered": BrowsePeers(server.Port),
		"update":     CheckUpdate(server.Version),
	})
}

func (server Server) session(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/sessions/delete" {
		server.deleteMany(w, r)
		return
	}
	if r.URL.Path == "/api/sessions/move" {
		server.move(w, r)
		return
	}
	if r.Method != http.MethodDelete {
		writeJSON(w, 404, map[string]any{"error": "Not found"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	if strings.Contains(id, "/") {
		writeJSON(w, 404, map[string]any{"error": "Not found"})
		return
	}
	status, message := DeleteSession(id)
	if status == 204 {
		w.WriteHeader(204)
		return
	}
	writeJSON(w, status, map[string]any{"error": message})
}

func (server Server) deleteMany(w http.ResponseWriter, r *http.Request) {
	payload, ok := readPayload(w, r, "Choose at least one session")
	if !ok {
		return
	}
	ids, _ := payload["ids"].([]any)
	status, body := DeleteSessions(ids)
	writeJSON(w, status, body)
}

func (server Server) move(w http.ResponseWriter, r *http.Request) {
	payload, ok := readPayload(w, r, "Unknown folder")
	if !ok {
		return
	}
	id, _ := payload["id"].(string)
	cwd, _ := payload["cwd"].(string)
	status, body := MoveSession(id, cwd)
	writeJSON(w, status, body)
}

func (server Server) locks(w http.ResponseWriter, r *http.Request) {
	payload, ok := readPayload(w, r, "Unknown lock")
	if !ok {
		return
	}
	kind, _ := payload["kind"].(string)
	id, _ := payload["id"].(string)
	locked, lockedOK := payload["locked"].(bool)
	if !lockedOK {
		writeJSON(w, 400, map[string]any{"error": "Unknown lock"})
		return
	}
	status, body := SetLock(kind, id, locked)
	writeJSON(w, status, body)
}

func (server Server) pins(w http.ResponseWriter, r *http.Request) {
	payload, ok := readPayload(w, r, "Unknown pin")
	if !ok {
		return
	}
	if _, present := payload["order"]; present {
		order, _ := payload["order"].([]any)
		status, body := ReorderPins(order)
		writeJSON(w, status, body)
		return
	}
	id, idOK := payload["id"].(string)
	pinned, pinOK := payload["pinned"].(bool)
	if !idOK || !pinOK {
		writeJSON(w, 400, map[string]any{"error": "Unknown pin"})
		return
	}
	status, body := SetPin(id, pinned)
	writeJSON(w, status, body)
}

func (server Server) machines(w http.ResponseWriter, r *http.Request) {
	payload, ok := readPayload(w, r, "Unknown machine")
	if !ok {
		return
	}
	if payload["remove"] == true {
		name, _ := payload["name"].(string)
		status, body := RemoveMachine(name)
		writeJSON(w, status, body)
		return
	}
	name, nameOK := payload["name"].(string)
	url, urlOK := payload["url"].(string)
	if !nameOK || !urlOK {
		writeJSON(w, 400, map[string]any{"error": "Unknown machine"})
		return
	}
	status, body := AddMachine(name, url)
	writeJSON(w, status, body)
}

func (server Server) update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 404, map[string]any{"error": "Not found"})
		return
	}
	version, err := ApplyUpdate(server.Version)
	if err != nil {
		if err.Error() == "already up to date" {
			writeJSON(w, 200, map[string]any{"ok": true, "version": version, "updated": false})
			return
		}
		writeJSON(w, 502, map[string]any{"error": "Could not update Grok Manager."})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "version": version, "updated": true})
	go RestartAfterUpdate()
}

func readPayload(w http.ResponseWriter, r *http.Request, emptyError string) (map[string]any, bool) {
	if r.Method != http.MethodPost {
		writeJSON(w, 404, map[string]any{"error": "Not found"})
		return nil, false
	}
	if r.ContentLength <= 0 || r.ContentLength > 65536 {
		writeJSON(w, 400, map[string]any{"error": emptyError})
		return nil, false
	}
	var payload map[string]any
	decoder := json.NewDecoder(io.LimitReader(r.Body, r.ContentLength))
	if err := decoder.Decode(&payload); err != nil || payload == nil {
		message := emptyError
		if payload == nil && err == nil {
			message = "Unknown lock"
		}
		writeJSON(w, 400, map[string]any{"error": message})
		return nil, false
	}
	return payload, true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		status = 500
		body = []byte(`{"error":"Could not encode a response."}`)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func MachineName() string {
	name, err := os.Hostname()
	if err != nil {
		return "this machine"
	}
	name = strings.TrimSpace(strings.Split(name, ".")[0])
	if name == "" {
		return "this machine"
	}
	return name
}
