package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	_ = os.Setenv("GROK_QUOTA", "0")
	os.Exit(m.Run())
}

func TestSavedQuotaSkipsRead(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GROK_QUOTA", "")
	t.Setenv("GROK_QUOTA_PATH", filepath.Join(dir, "quota.json"))
	if err := writeFileJSON(quotaPath(), Quota{Percent: 12, Until: "4 Nov"}); err != nil {
		t.Fatal(err)
	}
	quotaMu.Lock()
	quotaLoaded = false
	quotaRunning = false
	quotaOK = false
	quotaValue = Quota{}
	quotaMu.Unlock()

	quota, ok := CheckQuota()
	if !ok || quota.Percent != 12 || quota.Until != "4 Nov" {
		t.Fatalf("%+v %v", quota, ok)
	}
	quotaMu.Lock()
	running := quotaRunning
	quotaMu.Unlock()
	if running {
		t.Fatal("saved quota started a read")
	}
}

func TestQuotaRefreshRoute(t *testing.T) {
	server := httptest.NewServer(Server{Version: "1.4", Port: 18090}.Handler())
	defer server.Close()
	response, err := http.Post(server.URL+"/api/quota", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal(response.Status)
	}
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["quota"]; !ok || body["quota"] != nil {
		t.Fatalf("%#v", body)
	}
}

func TestParseBilling(t *testing.T) {
	quota, err := parseBilling(map[string]any{
		"config": map[string]any{
			"creditUsagePercent": 72.4,
			"currentPeriod": map[string]any{
				"type": "WEEKLY",
				"end":  "2026-11-04T23:00:00Z",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if quota.Percent != 72.4 {
		t.Fatalf("percent %v", quota.Percent)
	}
	if quota.Until == "" {
		t.Fatal("missing until")
	}
	fallback, err := parseBilling(map[string]any{
		"config": map[string]any{
			"creditUsagePercent": float64(0),
			"billingPeriodEnd":   "2026-11-04",
		},
	})
	if err != nil || fallback.Until != "4 Nov" {
		t.Fatalf("%+v %v", fallback, err)
	}
	if _, err := parseBilling(map[string]any{"config": map[string]any{}}); err == nil {
		t.Fatal("expected missing percent to fail")
	}
}
