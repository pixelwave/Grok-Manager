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
	saved := 12.0
	if err := writeFileJSON(quotaPath(), Quota{Percent: &saved, Until: "4 Nov"}); err != nil {
		t.Fatal(err)
	}
	quotaMu.Lock()
	quotaLoaded = false
	quotaRunning = false
	quotaOK = false
	quotaValue = Quota{}
	quotaMu.Unlock()

	quota, ok := CheckQuota()
	if !ok || quota.Percent == nil || *quota.Percent != 12 || quota.Until != "4 Nov" {
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
	if err != nil || quota.Percent == nil || *quota.Percent != 72.4 || quota.Until == "" {
		t.Fatalf("%+v %v", quota, err)
	}
	fallback, err := parseBilling(map[string]any{
		"config": map[string]any{
			"creditUsagePercent": float64(0),
			"billingPeriodEnd":   "2026-11-04",
		},
	})
	if err != nil || fallback.Percent == nil || *fallback.Percent != 0 || fallback.Until != "4 Nov" {
		t.Fatalf("%+v %v", fallback, err)
	}
	legacy, err := parseBilling(map[string]any{
		"config": map[string]any{
			"monthlyLimit":     map[string]any{"val": float64(2000)},
			"used":             map[string]any{"val": float64(500)},
			"billingPeriodEnd": "2026-11-04",
		},
	})
	if err != nil || legacy.Percent == nil || *legacy.Percent != 25 {
		t.Fatalf("%+v %v", legacy, err)
	}
	build, err := parseBilling(map[string]any{
		"config": map[string]any{
			"productUsage": []any{
				map[string]any{"product": "PRODUCT_GROK_CHAT", "usagePercent": float64(3)},
				map[string]any{"product": "PRODUCT_GROK_BUILD", "usagePercent": float64(61.2)},
			},
			"billingPeriodEnd": "2026-11-04",
		},
	})
	if err != nil || build.Percent == nil || *build.Percent != 61.2 {
		t.Fatalf("%+v %v", build, err)
	}
	dated, err := parseBilling(map[string]any{
		"config": map[string]any{"billingPeriodEnd": "2026-11-04"},
	})
	if err != nil || dated.Percent != nil || dated.Until != "4 Nov" {
		t.Fatalf("%+v %v", dated, err)
	}
	if _, err := parseBilling(map[string]any{"config": map[string]any{}}); err == nil {
		t.Fatal("expected an empty billing reply to fail")
	}
}
