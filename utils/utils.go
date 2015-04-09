package utils

import (
	path "path/filepath"
)

func ConfigPath() string {
	return path.Join(AppData(), "uget")
}

func AccountsPath() string {
	return path.Join(ConfigPath(), "accounts.json")
}
