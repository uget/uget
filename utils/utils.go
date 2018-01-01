package utils

import (
	path "path/filepath"

	"github.com/Wessie/appdirs"
)

var app = appdirs.New("uget", "", "")

// ConfigPath denotes the directory in the filesystem where data is persisted
func ConfigPath() string {
	return app.UserData()
}

// AccountsPath denotes the file where the accounts are stored
func AccountsPath() string {
	return path.Join(ConfigPath(), "accounts.json")
}
