package app

import "strings"

func pathListHas(path, dir string) bool {
	return pathListIndex(path, dir) >= 0
}

func pathListAdd(path, dir string) string {
	if pathListHas(path, dir) {
		return path
	}
	if strings.TrimSpace(path) == "" {
		return dir
	}
	if strings.HasSuffix(path, ";") {
		return path + dir
	}
	return path + ";" + dir
}

func pathListRemove(path, dir string) string {
	parts := strings.Split(path, ";")
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if pathEntryMatch(part, dir) {
			continue
		}
		kept = append(kept, part)
	}
	return strings.Join(kept, ";")
}

func pathListIndex(path, dir string) int {
	for i, part := range strings.Split(path, ";") {
		if pathEntryMatch(part, dir) {
			return i
		}
	}
	return -1
}

func pathEntryMatch(part, dir string) bool {
	part = cleanPathEntry(part)
	dir = cleanPathEntry(dir)
	return part != "" && strings.EqualFold(part, dir)
}

func cleanPathEntry(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	return strings.TrimRight(value, `\/`)
}
