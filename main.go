package main

import (
	"os"

	_ "github.com/uget/providers"
	"github.com/uget/uget/cli"
	"github.com/uget/uget/utils"
)

func main() {
	utils.InitLogger()
	os.Exit(cli.RunApp(os.Args))
}
