package main

import (
	"github.com/cihub/seelog"
	"github.com/muja/uget/cli"
	"github.com/muja/uget/utils"
	"os"
)

func main() {
	defer seelog.Flush()
	utils.InitLogger()
	cli.CreateApp().Run(os.Args)
}
