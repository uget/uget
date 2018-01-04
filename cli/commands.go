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
		if f.Size() != -1 {
			totalLength += f.Size()
			length := units.BytesSize(float64(f.Size()))
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
	downloader.NoContinue = opts.Get.NoContinue
	wg := downloader.AddURLs(urls)
	if opts.Get.DryRun {
		logrus.SetOutput(os.Stderr)
		downloader.DryRun()
		wg.Wait()
		return 0
	}
	exit := 0
	con := console.NewConsole()
	fprog := func(name string, progress, total, speed float64, via string) string {
		s := fmt.Sprintf("%s: %5.2f%% of %9s @ %9s/s%s", name, progress/total*100, units.BytesSize(total), units.BytesSize(speed), via)
		return s
	}
	type info struct {
		dl     *core.Download
		row    console.Row
		rater  rate.Rater
		via    string
		prog   int64
		start  time.Time
		ignore bool
	}

	dlChan := make(chan *core.Download)
	done := make(chan struct{})
	update := func(rootRater rate.Rater, downloads []*info) {
		if len(downloads) > 0 {
			for _, inf := range downloads {
				if inf.ignore {
					continue
				}
				diff := inf.dl.Progress() - inf.prog
				inf.rater.Add(diff)
				rootRater.Add(diff)
				inf.prog = inf.dl.Progress()
				if inf.dl.Done() {
					inf.ignore = true
					if err := inf.dl.Err(); err != nil {
						con.EditRow(inf.row, fmt.Sprintf("%s: error: %v", inf.dl.File.Name(), err))
					} else {
						name := inf.dl.File.Name()
						dl := units.BytesSize(float64(inf.dl.Progress()))
						pt := prettyTime(time.Since(inf.start))
						con.EditRow(inf.row, fmt.Sprintf("%s: downloaded %s in %s%s", name, dl, pt, inf.via))
					}
				} else {
					con.EditRow(inf.row, fprog(inf.dl.File.Name(), float64(inf.prog), float64(inf.dl.File.Size()), float64(inf.rater.Rate()), inf.via))
				}
			}
			con.Summary(fmt.Sprintf("TOTAL %9s/s", units.BytesSize(float64(rootRater.Rate()))))
		}
	}
	go func() {
		defer close(done)
		downloads := make([]*info, 0, len(urls))
		rootRater := rate.SmoothRate(10)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case download, ok := <-dlChan:
				if !ok {
					update(rootRater, downloads)
					fmt.Println() // newline after summary
					return
				}
				inf := &info{dl: download, prog: download.Progress(), start: time.Now()}
				inf.rater = rate.SmoothRate(10)
				if download.Provider != download.File.Provider() {
					inf.via = fmt.Sprintf(" (via %s)", download.Provider.Name())
				}
				inf.row = con.AddRow(
					fprog(download.File.Name(), float64(inf.prog), float64(download.File.Size()), 0, inf.via),
				)
				downloads = append(downloads, inf)
			case <-ticker.C:
				update(rootRater, downloads)
			}
		}
	}()
	downloader.OnDownload(func(download *core.Download) {
		dlChan <- download
	})
	downloader.OnSkip(func(file core.File) {
		con.AddRow(fmt.Sprintf("%s: skipped...", file.Name()))
	})
	downloader.OnDeadend(func(f core.File) {
		exit = 1
		con.AddRow(fmt.Sprintf("%v: Reached deadend.", f.URL()))
	})
	downloader.OnError(func(f core.File, err error) {
		exit = 1
		con.AddRow(fmt.Sprintf("%v: error: %v.", f.Name(), err))
	})
	downloader.Start()
	wg.Wait()
	close(dlChan)
	<-done
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
