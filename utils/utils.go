package utils

import (
	"os/user"
	path "path/filepath"
)

func ConfigPath() string {
	return path.Join(AppData(), "ugett")
}

func AccountsPath() string {
	return path.Join(ConfigPath(), "accounts.json")
}

func HomeDir() string {
	user, err := user.Current()
	if err != nil {
		panic(err)
	}
	return user.HomeDir
}
