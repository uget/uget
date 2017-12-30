package utils

import (
	path "path/filepath"

	"github.com/Wessie/appdirs"
)

var app = appdirs.New("uget", "", "")

func ConfigPath() string {
	return app.UserData()
}

func AccountsPath() string {
	return path.Join(ConfigPath(), "accounts.json")
}
