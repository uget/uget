package main

import (
	"github.com/muja/uget/cli"
	"os"
)

func main() {
	cli.CreateApp().Run(os.Args)
}
