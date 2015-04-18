package cli

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/uget/uget/api"
	"github.com/uget/uget/core"
	"github.com/uget/uget/core/account"
	"github.com/uget/uget/utils/console"
	"github.com/uget/uget/utils/units"
	"os"
	"os/exec"
	"time"
)

func CmdAddAccount(args []string, opt *Options) int {
	provider := selectPProvider(args[0])
	if provider == nil {
		return 1
	}
	prompter := NewCliPrompter(provider.Name(), opt.Unknowns)
	if !core.TryAddAccount(provider, prompter) {
		fmt.Fprintln(os.Stderr, "This provider does not support accounts.\n")
		return 1
	}
	return 0
}

func CmdListAccounts(args []string, opt *Options) int {
	var providers []core.Provider
	if len(args) == 0 {
		providers = core.AllProviders()
	} else {
		providers = []core.Provider{core.GetProvider(args[0])}
	}
	for _, p := range providers {
		pp, ok := p.(core.PersistentProvider)
		if ok {
			var accs []interface{}
			account.ManagerFor("", pp).Accounts(&accs)
			fmt.Printf("%s:\n", p.Name())
			for _, acc := range accs {
				fmt.Printf("    %v\n", acc)
			}
		}
	}
	return 0
}

func CmdSelectAccounts(args []string, opt *Options) int {
	var arg string
	if len(args) != 0 {
		arg = args[0]
	}
	provider := selectPProvider(arg)
	if provider == nil {
		return 1
	}
	mgr := account.ManagerFor("", provider)
	ids := []string{}
	mgr.Accounts(&ids)
	i := userSelection(ids, "Select an account")
	if i < 0 {
		fmt.Fprintln(os.Stderr, "Invalid selection")
		return 1
	}
	mgr.SelectAccount(ids[i])
	return 0
}

func CmdGet(files []string, opts *Options) int {
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "No files provided")
		return 1
	}
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
		download.Skip = !opts.Get.NoSkip
		id := con.AddRows(
			// fmt.Sprintf("%s:", download.Filename()),
			fprog(download.Filename(), 0, float64(download.Length())),
		)[0]
		download.OnUpdate(func(progress int64, total int64) {
			con.EditRow(id, fprog(download.Filename(), float64(progress), float64(total)))
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
	return 0
}

func CmdServer(args []string, opts *Options) int {
	server := &api.Server{}
	server.BindAddr = opts.Server.BindAddr
	server.Port = opts.Server.Port
	if server.Port != 9666 {
		fmt.Fprintln(os.Stderr, "Click'n'Load v2 will only work for port 9666!")
	}
	server.Run()
	return 1
}

func CmdDaemon(args []string, opts *Options) int {
	cmd := exec.Command(os.Args[0], append([]string{"server"}, os.Args[2:]...)...)
	fi, err := os.OpenFile("server.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Error(err)
	}
	cmd.Stdout, cmd.Stderr = fi, fi
	err = cmd.Start()
	if err != nil {
		log.Error("Error starting the daemon: ", err, ".")
		return 1
	} else {
		log.Info("Daemon running with pid ", cmd.Process.Pid)
		return 0
	}
}

func CmdPush(args []string, opts *Options) int {
	log.Error("Not implemented yet.")
	return 3
}
