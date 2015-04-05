package cli

import (
	"github.com/codegangsta/cli"
	"github.com/muja/uget/api"
	"log"
	"os"
	"os/exec"
)

func CreateApp() *cli.App {
	app := cli.NewApp()
	app.Name = "uget"
	app.Usage = "universal getter of remote files"
	app.Authors = []cli.Author{
		{
			Name:  "Danyel Bayraktar",
			Email: "danyel@webhippie.de",
		},
	}
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "verbose",
			Usage: "Show more output",
		},
	}
	app.Commands = []cli.Command{
		{
			Name: "push",
			// Flags: []cli.Flag{
			//   cli.BoolFlag{
			//     Name: "force",
			//     Usage: "force the push",
			//   },
			// },
			Usage:  "push the container specs to the daemon",
			Action: Push,
		},
		{
			Name:   "daemon",
			Usage:  "setup the server as daemon",
			Action: Daemon,
		},
		{
			Name: "server",
			Flags: []cli.Flag{
				cli.IntFlag{
					Name:  "port, p",
					Usage: "port the server listens on",
					Value: 9666,
				},
				cli.StringFlag{
					Name:  "bind, b",
					Usage: "address to bind the server to",
					Value: "0.0.0.0",
				},
			},
			Usage:  "start the server that accepts requests and executes them",
			Action: Server,
		},
	}
	return app
}

func Server(c *cli.Context) {
	server := &api.Server{}
	server.BindAddr = c.String("bind")
	server.Port = uint16(c.Int("port"))
	if server.Port != 9666 {
		log.Print("Warning: Click'n'Load v2 will only work for port 9666!")
	}
	server.Run()
}

func Daemon(c *cli.Context) {
	cmd := exec.Command(os.Args[0], append([]string{"server"}, os.Args[2:]...)...)
	fi, err := os.OpenFile("server.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}
	cmd.Stdout, cmd.Stderr = fi, fi
	err = cmd.Start()
	if err != nil {
		log.Fatal("Error starting the daemon: ", err, ".")
	} else {
		log.Print("Daemon running with pid ", cmd.Process.Pid)
	}
}

func Push(c *cli.Context) {
	log.Fatal("Not implemented yet.")
}
