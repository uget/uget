package cli

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/uget/uget/app"
	"github.com/uget/uget/core"
	api "github.com/uget/uget/server"
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
	if err := tryAddAccount(provider, prompter); err != nil {
		prompter.Error(err.Error())
		return 1
	}
	prompter.Success()
	return 0
}

func cmdListAccounts(args []string, opt *options) int {
	var providers []core.Provider
	if len(args) == 0 {
		providers = core.RegisteredProviders()
	} else {
		prov := core.RegisteredProviders().GetProvider(args[0])
		if prov == nil {
			fmt.Fprintf(os.Stderr, "No provider named %s\n", args[0])
			return 1
		}
		if _, ok := prov.(core.Accountant); !ok {
			fmt.Fprintf(os.Stderr, "Provider %v does not support accounts.\n", args[0])
			return 1
		}
		providers = []core.Provider{prov}
	}
	for _, p := range providers {
		pp, ok := p.(core.Accountant)
		if ok {
			accs := app.AccountManagerFor("", pp).Metadata()
			fmt.Printf("%s:\n", p.Name())
			for _, acc := range accs {
				fmt.Printf("    %v", acc.Data)
				if acc.Disabled {
					fmt.Printf(" (disabled)")
				}
				fmt.Println()
			}
		}
	}
	return 0
}

func cmdDisableAccount(args []string, opt *options) int {
	var arg string
	if len(args) != 0 {
		arg = args[0]
	}
	provider := selectPProvider(arg)
	if provider == nil {
		return 1
	}
	mgr := app.AccountManagerFor("", provider.(core.Accountant))
	accounts := mgr.Accounts()
	ids := make([]string, len(accounts))
	for i, acc := range accounts {
		ids[i] = acc.ID()
	}
	i, err := userSelection(ids, "Select an account", 2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	mgr.DisableAccount(ids[i])
	return 0
}

func cmdEnableAccount(args []string, opt *options) int {
	var arg string
	if len(args) != 0 {
		arg = args[0]
	}
	provider := selectPProvider(arg)
	if provider == nil {
		return 1
	}
	mgr := app.AccountManagerFor("", provider.(core.Accountant))
	accounts := mgr.Metadata()
	ids := make([]string, 0, len(accounts))
	for _, acc := range accounts {
		if acc.Disabled {
			ids = append(ids, acc.Data.ID())
		}
	}
	i, err := userSelection(ids, "Select an account", 2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	mgr.EnableAccount(ids[i])
	return 0
}

func cmdResolve(args []string, opts *options) int {
	urls := grabURLs(args, opts.Resolve.urlArgs)
	if urls == nil {
		return 1
	}
	client := core.NewClient()
	useAccounts(client)
	wg := client.AddURLs(urls)
	client.Resolve()
	wg.Wait()
	client.Finalize()
	var totalLength int64
	var unknownFactor bool
	ret := 0
	for file := range client.ResolvedQueue.Dequeue() {
		if file.Err() != nil {
			fmt.Printf("errored     %s - %v\n", file.URL(), file.Err().Error())
			unknownFactor = true
			ret = 1
		} else if file.Offline() {
			fmt.Printf("offline     %s\n", file.URL())
			unknownFactor = true
		} else if file.LengthUnknown() {
			fmt.Printf("???????     %s\n", file.URL())
		} else {
			totalLength += file.Size()
			length := units.BytesSize(float64(file.Size()))
			fmt.Printf("%9s   %s", length, file.Name())
			sum, algo, _ := file.Checksum()
			if opts.Resolve.Full {
				if sum == "" {
					fmt.Printf(" (%s)", file.URL())
				} else {
					fmt.Printf(" (%s, %s: %s)", file.URL(), algo, sum)
				}
			}
			fmt.Println()
		}
	}
	size, unit := units.Bytes(float64(totalLength))
	format := "TOTAL %s %s\n"
	if unknownFactor {
		format = "TOTAL %s+ %s\n"
	}
	fmt.Printf(format, size, unit)
	return ret
}

func cmdVersion(args []string, opts *options) int {
	fmt.Println("uget v" + core.Version)
	return 0
}

func cmdGet(args []string, opts *options) int {
	urls := grabURLs(args, opts.Get.urlArgs)
	if urls == nil {
		return 1
	}
	if opts.Get.Jobs < 1 {
		opts.Get.Jobs = 1
	}
	downloader := core.NewClientWith(opts.Get.Jobs)
	useAccounts(downloader)
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
