package main

import (
	_ "github.com/uget/providers"
	"github.com/uget/uget/cli"
	"github.com/uget/uget/utils"
	"os"
)

func main() {
	utils.InitLogger()
	os.Exit(cli.RunApp(os.Args))
}
