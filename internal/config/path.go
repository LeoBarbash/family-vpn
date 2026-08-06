package config

import (
	"path/filepath"
	"strings"
)

func NameFromPath(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	if base == "" {
		return "profile"
	}
	return base
}
