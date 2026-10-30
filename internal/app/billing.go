package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Quota is the weekly account bar for the login on this machine.
// Percent is nil when Grok answered but did not include a usage percentage.
type Quota struct {
	Percent *float64 `json:"percent,omitempty"`
	Until   string   `json:"until,omitempty"`
}

var (
	errQuotaAuth    = errors.New("quota auth")
	errQuotaMissing = errors.New("quota missing")

	quotaMu      sync.Mutex
	quotaLoaded  bool
	quotaRunning bool
	quotaGen     int
	quotaOK      bool
	quotaErr     string
	quotaValue   Quota
)

// WarmQuota loads the saved weekly quota. It asks Grok only when nothing is saved.
func WarmQuota() {
	_, _ = CheckQuota()
}

// CheckQuota returns the saved weekly quota. A missing file starts one read.
func CheckQuota() (Quota, bool) {
	if os.Getenv("GROK_QUOTA") == "0" {
		return Quota{}, false
	}
	quotaMu.Lock()
	defer quotaMu.Unlock()
	if !quotaLoaded {
		quotaLoaded = true
		if saved, ok := readSavedQuota(); ok {
			quotaValue = saved
			quotaOK = true
			return quotaValue, quotaOK
		}
		if !quotaRunning {
			quotaRunning = true
			go refreshQuota()
		}
	}
	return quotaValue, quotaOK
}

func quotaJSON() any {
	quota, ok := CheckQuota()
	if !ok {
		return nil
	}
	return quota
}

func quotaError() string {
	if os.Getenv("GROK_QUOTA") == "0" {
		return ""
	}
	quotaMu.Lock()
	defer quotaMu.Unlock()
	if quotaRunning || !quotaLoaded {
		return ""
	}
	return quotaErr
}

// ForceQuota reads the weekly quota now and stores it for the next start.
func ForceQuota() (Quota, bool) {
	if os.Getenv("GROK_QUOTA") == "0" {
		return Quota{}, false
	}
	quotaMu.Lock()
	quotaGen++
	gen := quotaGen
	quotaMu.Unlock()
	quota, err := fetchBilling()
	return storeQuota(gen, quota, err)
}

func refreshQuota() {
	quotaMu.Lock()
	gen := quotaGen
	quotaMu.Unlock()
	quota, err := fetchBilling()
	storeQuota(gen, quota, err)
}

func storeQuota(gen int, quota Quota, err error) (Quota, bool) {
	quotaMu.Lock()
	defer quotaMu.Unlock()
	quotaRunning = false
	quotaLoaded = true
	if gen != quotaGen {
		return quotaValue, quotaOK
	}
	if err == nil {
		quotaErr = ""
		quotaValue = quota
		quotaOK = quota.Percent != nil || quota.Until != ""
		if quota.Percent != nil {
			_ = writeFileJSON(quotaPath(), quota)
		}
		return quotaValue, quotaOK
	}
	if quotaOK {
		return quotaValue, quotaOK
	}
	quotaValue = Quota{}
	if errors.Is(err, errQuotaAuth) {
		quotaErr = "Grok is not signed in on this machine"
		_ = os.Remove(quotaPath())
	} else if errors.Is(err, errQuotaMissing) {
		quotaErr = "Grok was not found on this machine"
		_ = os.Remove(quotaPath())
	} else {
		quotaErr = "Weekly usage could not be read"
	}
	return quotaValue, false
}

func quotaPath() string { return configFile("GROK_QUOTA_PATH", "quota.json") }

func readSavedQuota() (Quota, bool) {
	body, err := os.ReadFile(quotaPath())
	if err != nil {
		return Quota{}, false
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(body, &raw) != nil {
		return Quota{}, false
	}
	if _, ok := raw["percent"]; !ok {
		return Quota{}, false
	}
	var quota Quota
	if json.Unmarshal(body, &quota) != nil || quota.Percent == nil || *quota.Percent < 0 || *quota.Percent > 100 {
		return Quota{}, false
	}
	return quota, true
}

func grokBinary() string {
	if path := os.Getenv("GROK_BIN"); path != "" {
		return path
	}
	if home := homeDir(); home != "" {
		for _, name := range []string{"grok", "grok.exe"} {
			path := filepath.Join(home, ".grok", "bin", name)
			info, err := os.Stat(path)
			if err == nil && !info.IsDir() {
				return path
			}
		}
	}
	if path, err := exec.LookPath("grok"); err == nil {
		return path
	}
	return ""
}

func grokEnv() []string {
	env := os.Environ()
	home := homeDir()
	if home == "" {
		return env
	}
	for _, item := range env {
		if strings.HasPrefix(item, "HOME=") && len(item) > len("HOME=") {
			return env
		}
	}
	return append(env, "HOME="+home)
}

func fetchBilling() (Quota, error) {
	bin := grokBinary()
	if bin == "" {
		return Quota{}, errQuotaMissing
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "agent", "stdio")
	cmd.Env = grokEnv()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return Quota{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Quota{}, err
	}
	if err := cmd.Start(); err != nil {
		return Quota{}, err
	}
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()

	writer := json.NewEncoder(stdin)
	requests := []struct {
		method string
		params any
	}{
		{"initialize", map[string]any{"protocolVersion": 1, "clientInfo": map[string]string{"name": "grok-manager", "version": "0"}, "capabilities": map[string]any{}}},
		{"authenticate", map[string]string{"methodId": "cached_token"}},
		{"_x.ai/billing", map[string]any{}},
	}
	for id, request := range requests {
		if err := writer.Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      id + 1,
			"method":  request.method,
			"params":  request.params,
		}); err != nil {
			return Quota{}, err
		}
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		var message struct {
			ID    int `json:"id"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
			Result json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			continue
		}
		if message.ID == 2 && message.Error != nil {
			return Quota{}, errQuotaAuth
		}
		if message.ID != 3 {
			continue
		}
		if message.Error != nil {
			if strings.Contains(strings.ToLower(message.Error.Message), "auth") {
				return Quota{}, errQuotaAuth
			}
			return Quota{}, fmt.Errorf("billing")
		}
		var result map[string]any
		if err := json.Unmarshal(message.Result, &result); err != nil {
			return Quota{}, err
		}
		return parseBilling(result)
	}
	if err := scanner.Err(); err != nil {
		return Quota{}, err
	}
	return Quota{}, fmt.Errorf("billing")
}

func parseBilling(result map[string]any) (Quota, error) {
	config, _ := result["config"].(map[string]any)
	if config == nil {
		return Quota{}, fmt.Errorf("billing")
	}
	until := ""
	if period, ok := config["currentPeriod"].(map[string]any); ok {
		until = formatUntil(period["end"])
	}
	if until == "" {
		until = formatUntil(config["billingPeriodEnd"])
	}
	if percent, ok := usagePercent(config); ok {
		value := percent
		return Quota{Percent: &value, Until: until}, nil
	}
	if until == "" {
		return Quota{}, fmt.Errorf("billing")
	}
	return Quota{Until: until}, nil
}

func usagePercent(config map[string]any) (float64, bool) {
	if percent, ok := asFloat(config["creditUsagePercent"]); ok {
		return clampPercent(percent), true
	}
	if percent, ok := productPercent(config["productUsage"]); ok {
		return clampPercent(percent), true
	}
	used, usedOK := centValue(config["used"])
	limit, limitOK := centValue(config["monthlyLimit"])
	if usedOK && limitOK && limit > 0 {
		return clampPercent(float64(used) / float64(limit) * 100), true
	}
	return 0, false
}

func productPercent(value any) (float64, bool) {
	items, ok := value.([]any)
	if !ok {
		return 0, false
	}
	var fallback *float64
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		percent, ok := asFloat(entry["usagePercent"])
		if !ok {
			continue
		}
		name, _ := entry["product"].(string)
		if name == "PRODUCT_GROK_BUILD" || name == "GrokBuild" {
			return percent, true
		}
		if fallback == nil {
			copied := percent
			fallback = &copied
		}
	}
	if fallback == nil {
		return 0, false
	}
	return *fallback, true
}

func centValue(value any) (float64, bool) {
	entry, ok := value.(map[string]any)
	if !ok {
		return 0, false
	}
	return asFloat(entry["val"])
}

func clampPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func asFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	default:
		return 0, false
	}
}

func formatUntil(value any) string {
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return ""
	}
	if parsed, err := time.Parse(time.RFC3339, text); err == nil {
		return parsed.In(time.Local).Format("2 Jan")
	}
	parsed, err := time.Parse("2006-01-02", text)
	if err != nil {
		return ""
	}
	return parsed.Format("2 Jan")
}
