package utils

import (
	"fmt"
	"github.com/cihub/seelog"
	path "path/filepath"
	"time"
)

const loggerConfig = `
<seelog>
  <outputs>
   <file path="%s" formatid="logformat" />
  </outputs>
  <formats>
    <format id="logformat" format="%%EscM(49)%%Date(02.01.2006 15:04:05.000) [%%Level] %%Msg%%EscM(0)%%n"/>
  </formats>
</seelog>
`

func InitLogger() {
	logfile := path.Join(app.UserLog(), time.Now().Local().Format("2006-01-02.log"))
	logger, err := seelog.LoggerFromConfigAsString(fmt.Sprintf(loggerConfig, logfile))
	if err != nil {
		seelog.Error(err)
	}
	seelog.ReplaceLogger(logger)
}
