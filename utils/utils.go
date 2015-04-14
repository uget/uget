package utils

import (
	"github.com/Wessie/appdirs"
	path "path/filepath"
)

var app = appdirs.New("uget", "", "")

func ConfigPath() string {
	return app.UserData()
}

func AccountsPath() string {
	return path.Join(ConfigPath(), "accounts.json")
}
