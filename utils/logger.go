package utils

import (
	"os"
	path "path/filepath"
	"time"

	"github.com/Sirupsen/logrus"
	isatty "github.com/mattn/go-isatty"
)

// InitLogger initiates the logger to log into the APP_USER_LOG path
func InitLogger() {
	logrus.SetLevel(logrus.DebugLevel)
	if !isatty.IsTerminal(os.Stderr.Fd()) {
		logrus.SetOutput(os.Stderr)
		return
	}

	logfile := path.Join(app.UserLog(), time.Now().Local().Format("2006-01-02.log"))
	err1 := os.MkdirAll(path.Dir(logfile), 0755)
	f, err2 := os.OpenFile(logfile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err1 != nil || err2 != nil {
		logrus.SetOutput(os.Stderr)
		logrus.WithFields(logrus.Fields{
			"file": logfile,
		}).Error("Could not create file or parent directories.")
	} else {
		logrus.SetOutput(f)
	}
}
