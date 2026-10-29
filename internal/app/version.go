package app

import "strings"

// AssetName is the release file for an OS and CPU.
func AssetName(goos, goarch string) (string, error) {
	var arch string
	switch goarch {
	case "amd64":
		arch = "amd64"
	case "arm64":
		arch = "arm64"
	default:
		return "", errUnsupported("unsupported architecture: " + goarch)
	}
	switch goos {
	case "linux", "darwin":
		return "grok-manager_" + goos + "_" + arch, nil
	case "windows":
		return "grok-manager_windows_" + arch + ".exe", nil
	default:
		return "", errUnsupported("This installer supports Linux, Mac, and Windows.")
	}
}

type unsupported string

func (e unsupported) Error() string { return string(e) }

func errUnsupported(message string) error { return unsupported(message) }

// Newer reports whether latest is a higher version than current.
// A release is YY.MM.DD. The first publish of a day has three numbers and
// counts as revision 1. A later publish the same day is YY.MM.DD.2, then .3.
// The next day starts again at three numbers.
func Newer(latest, current string) bool {
	return compareVersions(latest, current) > 0
}

func compareVersions(a, b string) int {
	left := splitVersion(a)
	right := splitVersion(b)
	for i := 0; i < len(left); i++ {
		if left[i] != right[i] {
			if left[i] > right[i] {
				return 1
			}
			return -1
		}
	}
	return 0
}

func splitVersion(value string) [4]int {
	value = strings.TrimPrefix(strings.TrimSpace(value), "v")
	var out [4]int
	out[3] = 1
	parts := strings.Split(value, ".")
	for i := 0; i < len(parts) && i < 3; i++ {
		out[i] = leadingInt(parts[i])
	}
	if len(parts) >= 4 {
		if number, ok := revision(parts[3]); ok {
			out[3] = number
		}
	}
	return out
}

func leadingInt(part string) int {
	number := 0
	for _, char := range part {
		if char < '0' || char > '9' {
			break
		}
		number = number*10 + int(char-'0')
	}
	return number
}

func revision(part string) (int, bool) {
	if part == "" {
		return 0, false
	}
	number := 0
	for _, char := range part {
		if char < '0' || char > '9' {
			return 0, false
		}
		number = number*10 + int(char-'0')
	}
	if number < 2 {
		return 0, false
	}
	return number, true
}
