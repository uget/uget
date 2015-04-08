package cli

import (
	// "fmt"
	log "github.com/cihub/seelog"
	"github.com/codegangsta/cli"
	"github.com/uget/uget/api"
	"github.com/uget/uget/core"
	"os"
	"os/exec"
	"time"
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
			Name:   "push",
			Usage:  "push the container specs to the daemon",
			Action: Push,
		},
		{
			Name:   "get",
			Usage:  "download a single link",
			Action: Get,
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
		log.Warn("Click'n'Load v2 will only work for port 9666!")
	}
	server.Run()
}

func Daemon(c *cli.Context) {
	cmd := exec.Command(os.Args[0], append([]string{"server"}, os.Args[2:]...)...)
	fi, err := os.OpenFile("server.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Error(err)
	}
	cmd.Stdout, cmd.Stderr = fi, fi
	err = cmd.Start()
	if err != nil {
		log.Error("Error starting the daemon: ", err, ".")
	} else {
		log.Info("Daemon running with pid ", cmd.Process.Pid)
	}
}

func Push(c *cli.Context) {
	log.Error("Not implemented yet.")
}

func Get(c *cli.Context) {
	links := c.Args()
	client := core.NewDownloader()
	client.Queue.AddLinks(links, 1)
	client.Start(true)
	for {
		select {
		case <-client.Finished():
			return
		case download := <-client.NewDownload():
			download.UpdateInterval = 500 * time.Millisecond
			download.AddProgressListener(core.ProgressListener{
				Update: func(progress float64, total float64) {
					log.Tracef("%v - progress: %.2f%%", download.Filename(), progress/total*100)
				},
				Done: func(dur time.Duration, err error) {
					if err != nil {
						log.Errorf("Error! %v", err)
					} else {
						log.Infof("Done! Duration: %v", dur)
					}
				},
			})

		}
	}
}
