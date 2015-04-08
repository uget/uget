package utils

import (
	"github.com/cihub/seelog"
	"os"
)

var loggerConfig = `
<seelog>
  <outputs>
    <custom name="myreceiver" formatid="testFormat"/>
  </outputs>
  <formats>
    <format id="testFormat" format="%EscM(49)%Date(02.01.2006 15:04:05.000) [%Level] %Msg%EscM(0)%n"/>
  </formats>
</seelog>
`

type Receiver struct{}

func (r Receiver) ReceiveMessage(message string, level seelog.LogLevel, context seelog.LogContextInterface) error {
	os.Stderr.WriteString(message)
	return nil
}
func (r Receiver) AfterParse(initArgs seelog.CustomReceiverInitArgs) error {
	return nil
}
func (r Receiver) Flush() {}
func (r Receiver) Close() error {
	return nil
}

func InitLogger() {
	seelog.RegisterReceiver("myreceiver", &Receiver{})
	logger, err := seelog.LoggerFromConfigAsString(loggerConfig)
	if err != nil {
		seelog.Error(err)
	}
	seelog.ReplaceLogger(logger)
}
