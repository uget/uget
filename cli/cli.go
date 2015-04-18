package cli

import (
	"bufio"
	"fmt"
	log "github.com/Sirupsen/logrus"
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
					Name:            "select",
					Usage:           "select an account",
					Action:          SelectAccount,
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
		log.WithField("file", f).Fatal("could not open")
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		*links = append(*links, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
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
	fprog := func(name string, progress float64, total float64) string {
		return fmt.Sprintf("%s: %5.2f%% of %10s", name, progress/total*100, units.HumanSize(total))
	}
	client.OnDownload(func(download *core.Download) {
		download.UpdateInterval = 500 * time.Millisecond
		id := con.AddRows(
			// fmt.Sprintf("%s:", download.Filename()),
			fprog(download.Filename(), 0, float64(download.Length())),
		)[0]
		download.OnUpdate(func(progress float64, total float64) {
			con.EditRow(id, fprog(download.Filename(), progress, total))
		})
		download.OnSkip(func() {
			con.EditRow(id, fmt.Sprintf("%s: skipped...", download.Filename()))
		})
		download.OnDone(func(dur time.Duration, err error) {
			if err != nil {
				con.EditRow(id, fmt.Sprintf("%s: error: %v", download.Filename(), err))
			} else {
				con.EditRow(id, fmt.Sprintf("%s: done. Duration: %v", download.Filename(), dur))
			}
		})
	})
	<-client.Finished()
}

func SelectAccount(c *cli.Context) {
	provider := selectPProvider(c.Args().First())
	mgr := account.ManagerFor("", provider)
	ids := []string{}
	mgr.Accounts(&ids)
	i := userSelection(ids, "Select an account")
	if i < 0 {
		fmt.Fprintln(os.Stderr, "Invalid selection")
		os.Exit(1)
	}
	mgr.SelectAccount(ids[i])
}

func AddAccount(c *cli.Context) {
	provider := selectPProvider(c.Args().First())
	if provider == nil {
		return
	}
	prompter := NewCliPrompter(c, provider.Name())
	if !core.TryAddAccount(provider, prompter) {
		fmt.Fprintln(os.Stderr, "This provider does not support accounts.\n")
	}
}

func selectPProvider(arg string) core.PersistentProvider {
	if arg == "" {
		ps := make([]string, 0)
		for _, p := range core.AllProviders() {
			if pp, ok := p.(core.PersistentProvider); ok {
				ps = append(ps, pp.Name())
			}
		}
		i := userSelection(ps, "Choose a provider")
		if i < 0 {
			fmt.Fprintln(os.Stderr, "Invalid selection.\n")
			os.Exit(1)
		}
		arg = ps[i]
	}
	provider := core.GetProvider(arg).(core.PersistentProvider)
	if provider == nil {
		fmt.Printf("No provider found for %s\n", arg)
	}
	return provider
}

func userSelection(arr []string, prompt string) int {
	for i, x := range arr {
		fmt.Printf("- %s (%v)\n", x, i+1)
	}
	i := -1
	fmt.Printf("%s: ", prompt)
	buf := make([]byte, 256)
	read, err := os.Stdin.Read(buf)
	if err != nil {
		panic(err)
	}
	str := strings.TrimSpace(string(buf[:read]))
	if len(str) > 0 {
		if _, err := fmt.Sscanf(str, "%d", &i); err != nil {
			for index, s := range arr {
				if s == str {
					i = index + 1
					break
				}
			}
		}
	}
	return i - 1
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
