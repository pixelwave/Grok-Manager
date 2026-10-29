package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPRoutes(t *testing.T) {
	setup(t)
	addSession(t, "/root/projects/plannr", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1", "Keep", "2026-01-01T00:00:00Z")
	server := httptest.NewServer(Server{Version: "1.4", Port: 18090}.Handler())
	defer server.Close()
	response, err := http.Get(server.URL + "/api/sessions")
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	groups := body["groups"].([]any)
	if len(groups) != 1 {
		t.Fatalf("%#v", body)
	}
	page, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer page.Body.Close()
	if page.StatusCode != 200 || !strings.Contains(page.Header.Get("Content-Type"), "text/html") {
		t.Fatal(page.Status)
	}
	request, _ := http.NewRequest(http.MethodDelete, server.URL+"/api/sessions/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1", nil)
	deleted, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	deleted.Body.Close()
	if deleted.StatusCode != 204 {
		t.Fatal(deleted.StatusCode)
	}
}
