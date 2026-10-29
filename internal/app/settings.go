package app

import (
	"os"
	"strconv"
)

type settingsFile struct {
	Port int `json:"port"`
}

func LoadPort() int {
	if value := os.Getenv("GROK_MANAGER_PORT"); value != "" {
		if port, err := strconv.Atoi(value); err == nil && port > 0 && port < 65536 {
			return port
		}
	}
	var file settingsFile
	if err := readJSON(settingsPath(), &file); err == nil && file.Port > 0 && file.Port < 65536 {
		return file.Port
	}
	return 18090
}

func SavePort(port int) error {
	return writeFileJSON(settingsPath(), settingsFile{Port: port})
}
