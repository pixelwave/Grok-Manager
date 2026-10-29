package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const releaseAPI = "https://api.github.com/repos/pixelwave/Grok-Manager/releases/latest"

type releaseInfo struct {
	Tag    string
	Asset  string
	URL    string
	SumURL string
}

type UpdateStatus struct {
	Available bool   `json:"available"`
	Latest    string `json:"latest,omitempty"`
}

var (
	updateMu      sync.Mutex
	updateChecked time.Time
	updateStatus  UpdateStatus
	updateRunning bool
)

func CheckUpdate(version string) UpdateStatus {
	updateMu.Lock()
	defer updateMu.Unlock()
	if !updateChecked.IsZero() && time.Since(updateChecked) < 30*time.Minute {
		return updateStatus
	}
	if !updateRunning {
		updateRunning = true
		go refreshUpdate(version)
	}
	return updateStatus
}

func refreshUpdate(version string) {
	info, err := latestRelease()
	updateMu.Lock()
	defer updateMu.Unlock()
	updateRunning = false
	updateChecked = time.Now()
	if err != nil {
		updateStatus = UpdateStatus{}
		return
	}
	updateStatus = UpdateStatus{Available: Newer(info.Tag, version), Latest: strings.TrimPrefix(info.Tag, "v")}
}

func latestRelease() (releaseInfo, error) {
	request, err := http.NewRequest(http.MethodGet, releaseAPI, nil)
	if err != nil {
		return releaseInfo{}, err
	}
	request.Header.Set("User-Agent", "grok-manager")
	request.Header.Set("Accept", "application/vnd.github+json")
	client := &http.Client{Timeout: 20 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return releaseInfo{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return releaseInfo{}, fmt.Errorf("release lookup returned %d", response.StatusCode)
	}
	var body struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(bodyReader(response)).Decode(&body); err != nil {
		return releaseInfo{}, err
	}
	asset, err := AssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return releaseInfo{}, err
	}
	info := releaseInfo{Tag: body.TagName, Asset: asset}
	for _, item := range body.Assets {
		switch item.Name {
		case asset:
			info.URL = item.BrowserDownloadURL
		case "SHA256SUMS":
			info.SumURL = item.BrowserDownloadURL
		}
	}
	if info.URL == "" || info.SumURL == "" {
		return releaseInfo{}, fmt.Errorf("release is missing %s", asset)
	}
	return info, nil
}

func bodyReader(response *http.Response) io.Reader { return response.Body }

// ApplyUpdate downloads the latest binary, replaces this one, and removes leftovers.
func ApplyUpdate(version string) (string, error) {
	info, err := latestRelease()
	if err != nil {
		return "", err
	}
	if !Newer(info.Tag, version) {
		return strings.TrimPrefix(info.Tag, "v"), fmt.Errorf("already up to date")
	}
	dir, err := os.MkdirTemp("", "grok-manager-download-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	binary := filepath.Join(dir, info.Asset)
	sums := filepath.Join(dir, "SHA256SUMS")
	if err := download(info.URL, binary); err != nil {
		return "", err
	}
	if err := download(info.SumURL, sums); err != nil {
		return "", err
	}
	if err := verifySum(binary, sums, info.Asset); err != nil {
		return "", err
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	old := exe + ".old"
	_ = os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		return "", err
	}
	if err := copyFile(binary, exe); err != nil {
		_ = os.Rename(old, exe)
		return "", err
	}
	_ = os.Chmod(exe, 0o755)
	_ = os.Remove(old)
	cleanDownloads()
	updateMu.Lock()
	updateChecked = time.Time{}
	updateMu.Unlock()
	return strings.TrimPrefix(info.Tag, "v"), nil
}

func download(url, dest string) error {
	client := &http.Client{Timeout: 2 * time.Minute}
	response, err := client.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return fmt.Errorf("download returned %d", response.StatusCode)
	}
	file, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, response.Body)
	return err
}

func verifySum(path, sums, name string) error {
	body, err := os.ReadFile(sums)
	if err != nil {
		return err
	}
	want := ""
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == name {
			want = fields[0]
		}
	}
	if want == "" {
		return fmt.Errorf("checksum missing for %s", name)
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	got := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("checksum did not match")
	}
	return nil
}

func copyFile(src, dest string) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		output.Close()
		return err
	}
	return output.Close()
}

func cleanDownloads() {
	root := os.TempDir()
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "grok-manager-download-") {
			_ = os.RemoveAll(filepath.Join(root, entry.Name()))
		}
	}
}

// CleanLeftovers removes a previous binary left beside this one.
func CleanLeftovers() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	_ = os.Remove(exe + ".old")
	cleanDownloads()
}
