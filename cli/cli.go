package cli

import (
	"bufio"
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/codegangsta/cli"
	"github.com/uget/uget/api"
	"github.com/uget/uget/core"
	"github.com/uget/uget/core/account"
	"github.com/uget/uget/utils/console"
	"github.com/uget/uget/utils/units"
	"os"
	"os/exec"
	"strings"
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
			Name:  "accounts",
			Usage: "manage your accounts",
			Subcommands: []cli.Command{
				{
					Name:            "add",
					Usage:           "add an account",
					Action:          AddAccount,
					SkipFlagParsing: true,
				},
				{
					Name:            "list",
					Usage:           "list all accounts",
					Action:          ListAccounts,
					SkipFlagParsing: true,
				},
			},
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

func linksFromFile(links *[]string, f string) error {
	file, err := os.Open(f)
	if err != nil {
		log.Criticalf("Could not open file %s", f)
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		*links = append(*links, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		log.Critical(err)
		return err
	}
	return nil
}

func Get(c *cli.Context) {
	files := c.Args()
	links := make([]string, 0, 256)
	for _, file := range files {
		linksFromFile(&links, file)
	}
	client := core.NewDownloader()
	client.Queue.AddLinks(links, 1)
	client.Start(true)
	con := console.NewConsole()
	fn := func(name string, progress float64, total float64) string {
		return fmt.Sprintf("%s: %5.2f%% of %10s", name, progress/total*100, units.HumanSize(total))
	}
	for {
		select {
		case <-client.Finished():
			return
		case download := <-client.NewDownload():
			id := con.AddRows(
				// fmt.Sprintf("%s:", download.Filename()),
				fn(download.Filename(), 0, float64(download.Length())),
			)[0]
			download.UpdateInterval = 500 * time.Millisecond
			download.AddProgressListener(core.ProgressListener{
				Update: func(progress float64, total float64) {
					con.EditRow(id, fn(download.Filename(), progress, total))
				},
				Skip: func() {
					con.EditRow(id, fmt.Sprintf("%s: skipped...", download.Filename()))
				},
				Done: func(dur time.Duration, err error) {
					if err != nil {
						con.EditRow(id, fmt.Sprintf("%s: error: %v", download.Filename(), err))
					} else {
						con.EditRow(id, fmt.Sprintf("%s: done. Duration: %v", download.Filename(), dur))
					}
				},
			})
		}
	}
}

func AddAccount(c *cli.Context) {
	p := c.Args().First()
	mp := map[int]core.PersistentProvider{}
	var provider core.Provider
	i := 0
	if p == "" {
		for _, p := range core.AllProviders() {
			if pp, ok := p.(core.PersistentProvider); ok {
				i++
				mp[i] = pp
			}
		}
		// fmt.Println("Here is a list of available providers:")
		for index, pp := range mp {
			fmt.Printf("- %s (%v)\n", pp.Name(), index)
		}
		fmt.Print("Choose a provider from above: ")
		buf := make([]byte, 256)
		read, _ := os.Stdin.Read(buf)
		str := strings.TrimSpace(string(buf[:read]))
		if len(str) == 0 {
			fmt.Println("No input provided.")
			os.Exit(1)
		} else if _, err := fmt.Sscanf(str, "%d", &i); err == nil {
			provider = mp[i]
		} else {
			provider = core.GetProvider(str)
		}
	} else {
		provider = core.GetProvider(p)
		if provider == nil {
			fmt.Printf("No provider found for %s\n", p)
			return
		}
	}
	prompter := NewCliPrompter(c, provider.Name())
	if !core.TryAddAccount(provider, prompter) {
		fmt.Printf("This provider does not support accounts.\n")
	}
}

func ListAccounts(c *cli.Context) {
	pn := c.Args().First()
	var providers []core.Provider
	if pn == "" {
		providers = core.AllProviders()
	} else {
		providers = []core.Provider{core.GetProvider(pn)}
	}
	for _, p := range providers {
		pp, ok := p.(core.PersistentProvider)
		if ok {
			var accs []interface{}
			account.ManagerFor("", pp).Accounts(&accs)
			fmt.Printf("%s:\n", p.Name())
			for _, acc := range accs {
				fmt.Printf("    %s\n", acc)
			}
		}
	}
}
