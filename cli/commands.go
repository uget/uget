package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/uget/uget/api"
	"github.com/uget/uget/core"
	"github.com/uget/uget/utils/console"
	"github.com/uget/uget/utils/rate"
	"github.com/uget/uget/utils/units"
)

func cmdAddAccount(args []string, opt *options) int {
	pName := ""
	if len(args) != 0 {
		pName = args[0]
	}
	provider := selectPProvider(pName)
	if provider == nil {
		return 1
	}
	prompter := newCliPrompter(provider.Name(), opt.Unknowns)
	if err := core.TryAddAccount(provider, prompter); err != nil {
		prompter.Error(err.Error())
		return 1
	}
	prompter.Success()
	return 0
}

func cmdListAccounts(args []string, opt *options) int {
	var providers []core.Provider
	if len(args) == 0 {
		providers = core.AllProviders()
	} else {
		providers = []core.Provider{core.GetProvider(args[0])}
	}
	for _, p := range providers {
		pp, ok := p.(core.Accountant)
		if ok {
			var accs []interface{}
			core.AccountManagerFor("", pp).Accounts(&accs)
			fmt.Printf("%s:\n", p.Name())
			for _, acc := range accs {
				fmt.Printf("    %v\n", acc)
			}
		}
	}
	return 0
}

func cmdSelectAccounts(args []string, opt *options) int {
	var arg string
	if len(args) != 0 {
		arg = args[0]
	}
	provider := selectPProvider(arg)
	if provider == nil {
		return 1
	}
	mgr := core.AccountManagerFor("", provider.(core.Accountant))
	ids := []string{}
	mgr.Accounts(&ids)
	i, err := userSelection(ids, "Select an account", 2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	mgr.SelectAccount(ids[i])
	return 0
}

func cmdResolve(args []string, opts *options) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "No arguments provided")
		return 1
	}
	urls := grabURLs(args, opts.Resolve.urlArgs)
	client := core.NewClient()
	files, err := client.ResolveSync(urls)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving: %v\n", err.Error())
		return 1
	}
	var totalLength int64
	for _, f := range files {
		if f.Length() != -1 {
			totalLength += f.Length()
			length := units.BytesSize(float64(f.Length()))
			fmt.Printf("%9s   %s", length, f.URL())
			sum, algo, _ := f.Checksum()
			pathSegments := strings.Split(f.URL().RequestURI(), "/")
			uriDiffersFromFile := pathSegments[len(pathSegments)-1] != f.Name()
			if opts.Resolve.Full && (sum != "" || uriDiffersFromFile) {
				if sum == "" {
					fmt.Printf(" (%s)", f.Name())
				} else if uriDiffersFromFile {
					fmt.Printf(" (%s, %s: %s)", f.Name(), algo, sum)
				} else {
					fmt.Printf(" (%s: %s)", algo, sum)
				}
			}
			fmt.Println()
		} else {
			fmt.Printf("offline     %s\n", f.URL())
		}
	}
	fmt.Println(units.BytesSize(float64(totalLength)))
	return 0
}

func cmdVersion(args []string, opts *options) int {
	fmt.Println("uget v" + core.Version)
	return 0
}

func cmdGet(args []string, opts *options) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "No arguments provided")
		return 1
	}
	urls := grabURLs(args, opts.Get.urlArgs)
	if opts.Get.Jobs < 1 {
		opts.Get.Jobs = 1
	}
	downloader := core.NewClientWith(opts.Get.Jobs)
	downloader.Skip = !opts.Get.NoSkip
	wg := downloader.AddURLs(urls)
	if opts.Get.DryRun {
		logrus.SetOutput(os.Stderr)
		downloader.DryRun()
		wg.Wait()
		return 0
	}
	exit := 0
	con := console.NewConsole()
	rootRater := rate.SmoothRate(10)
	fprog := func(name string, progress, total, speed float64, via string) string {
		s := fmt.Sprintf("%s: %5.2f%% of %9s @ %9s/s%s", name, progress/total*100, units.BytesSize(total), units.BytesSize(speed), via)
		return s
	}
	go func() {
		for {
			con.Summary(fmt.Sprintf("TOTAL %9s/s", units.BytesSize(float64(rootRater.Rate()))))
			<-time.After(500 * time.Millisecond)
		}
	}()
	downloader.OnDownload(func(download *core.Download) {
		download.UpdateInterval = 500 * time.Millisecond
		var progress int64
		rater := rate.SmoothRate(10)
		via := ""
		if download.Provider != download.File.Provider() {
			via = fmt.Sprintf(" (via %s)", download.Provider.Name())
		}
		id := con.AddRow(
			// fmt.Sprintf("%s:", download.File.Name()),
			fprog(download.File.Name(), 0, float64(download.File.Length()), 0, via),
		)
		download.OnUpdate(func(prog int64) {
			diff := prog - progress
			rater.Add(diff)
			// thread unsafe, but we don't care since it's not meant to be precise
			rootRater.Add(diff)
			progress = prog
			con.EditRow(id, fprog(download.File.Name(), float64(prog), float64(download.File.Length()), float64(rater.Rate()), via))
		})
		download.OnDone(func(dur time.Duration, err error) {
			if err != nil {
				con.EditRow(id, fmt.Sprintf("%s: error: %v", download.File.Name(), err))
			} else {
				con.EditRow(id, fmt.Sprintf("%s: done. Duration: %v", download.File.Name(), dur))
			}
		})
	})
	downloader.OnSkip(func(download *core.Download) {
		con.AddRow(fmt.Sprintf("%s: skipped...", download.File.Name()))
	})
	downloader.OnDeadend(func(f core.File) {
		exit = 1
		con.AddRow(fmt.Sprintf("%v: Reached deadend.", f.URL()))
	})
	downloader.OnError(func(f core.File, err error) {
		exit = 1
		con.AddRow(fmt.Sprintf("%v.", err))
	})
	downloader.Start()
	wg.Wait()
	fmt.Println()
	return exit
}

func cmdServer(args []string, opts *options) int {
	server := &api.Server{}
	server.BindAddr = opts.Server.BindAddr
	server.Port = opts.Server.Port
	if server.Port != 9666 {
		fmt.Fprintln(os.Stderr, "Click'n'Load v2 will only work for port 9666!")
	}
	server.Run()
	return 1
}

func cmdDaemon(args []string, opts *options) int {
	cmd := exec.Command(os.Args[0], append([]string{"server"}, os.Args[2:]...)...)
	fi, err := os.OpenFile("server.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		logrus.Error(err)
	}
	cmd.Stdout, cmd.Stderr = fi, fi
	err = cmd.Start()
	if err != nil {
		logrus.Error("Error starting the daemon: ", err, ".")
		return 1
	}
	logrus.Info("Daemon running with pid ", cmd.Process.Pid)
	return 0
}

func cmdPush(args []string, opts *options) int {
	logrus.Error("Not implemented yet.")
	return 3
}
