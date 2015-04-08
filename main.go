package main

import (
	"github.com/cihub/seelog"
	"github.com/uget/uget/cli"
	"github.com/uget/uget/utils"
	"os"
)

func main() {
	defer seelog.Flush()
	utils.InitLogger()
	cli.CreateApp().Run(os.Args)
}
