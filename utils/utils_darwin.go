// +build darwin

package utils

import (
	"os/user"
	"path"
)

func AppData() string {
	user, _ := user.Current()
	return path.Join(user.HomeDir, "Library", "Application Support")
}
