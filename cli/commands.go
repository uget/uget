package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/uget/uget/app"
	"github.com/uget/uget/core"
	"github.com/uget/uget/server"
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
	if cmd, _ := setupPager(client.ResolvedQueue.Len()); cmd != nil {
		defer cmd.Wait()
		defer os.Stdout.Close()
	}
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
				if sum == nil {
					fmt.Printf("   %s", file.URL())
				} else {
					fmt.Printf("   %s   %s %s", file.URL(), algo, fmt.Sprintf("%x", sum))
				}
			}
			if opts.Resolve.Compare {
				remove := false
				fmt.Print(", ")
				if stat, err := os.Stat(file.Name()); err != nil {
					if err.(*os.PathError).Err == syscall.ENOENT {
						fmt.Print("no local file.")
					} else {
						fmt.Printf("error reading local file: %v", err)
					}
				} else {
					if stat.Size() < file.Size() {
						fmt.Print("local is smaller")
					} else if stat.Size() > file.Size() {
						fmt.Print("local is bigger")
						remove = true
					} else {
						fmt.Print("sizes match. ")
						if cks, algo, h := file.Checksum(); h != nil {
							fmt.Printf("%s-checksum: ", algo)
							if f, err := os.Open(file.Name()); err != nil {
								fmt.Printf("error opening local: %v", err)
							} else {
								io.Copy(h, f)
								localCks := h.Sum(nil)
								if bytes.Equal(cks, localCks) {
									fmt.Print("match")
								} else {
									fmt.Printf("don't match (%s : %s)", localCks, cks)
									remove = true
								}
							}
						} else {
							fmt.Print("no checksum data available.")
						}
					}
				}
				if remove {
					fmt.Print(", deleting")
					if err := os.Remove(file.Name()); err != nil {
						fmt.Printf(", error: %v", err)
					}
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
	downloader.NoSkip = opts.Get.NoSkip
	downloader.NoContinue = opts.Get.NoContinue
	wg := downloader.AddURLs(urls)
	if opts.Get.DryRun {
		logrus.SetOutput(os.Stderr)
		downloader.DryRun()
		wg.Wait()
		return 0
	}
	exit := 0
	con := console.Updating(500 * time.Millisecond)
	defer con.Close(true)
	fprog := func(name string, progress, total, speed float64, via string) string {
		s := fmt.Sprintf("%s: %5.2f%% of %9s @ %9s/s%s", name, progress/total*100, units.BytesSize(total), units.BytesSize(speed), via)
		return s
	}
	rootRater := rate.SmoothRate(10)
	con.Add(func() string {
		return fmt.Sprintf("TOTAL %9s/s", units.BytesSize(float64(rootRater.Rate())))
	})
	downloader.OnDownload(func(download *core.Download) {
		prog := download.Progress()
		start := time.Now()
		rater := rate.SmoothRate(10)
		var via string
		if download.Provider != download.File.Provider() {
			via = fmt.Sprintf(" (via %s)", download.Provider.Name())
		}
		con.Insert(-1, func() string {
			if download.Done() {
				if download.Err() != nil {
					return fmt.Sprintf("%s: error: %v", download.File.Name(), download.Err())
				} else if download.Canceled() {
					return fmt.Sprintf("%s: stopped.", download.File.Name())
				} else {
					name := download.File.Name()
					download := units.BytesSize(float64(download.Progress()))
					pt := prettyTime(time.Since(start))
					return fmt.Sprintf("%s: downloaded %s in %s%s", name, download, pt, via)
				}
			} else {
				progress := download.Progress()
				diff := progress - prog
				prog = progress
				rater.Add(diff)
				rootRater.Add(diff)
				return fprog(download.File.Name(), float64(prog), float64(download.Size()), float64(rater.Rate()), via)
			}
		})
	})
	downloader.OnSkip(func(file core.File) {
		con.InsertConst(-1, fmt.Sprintf("%s: skipped...", file.Name()))
	})
	downloader.OnError(func(f core.File, err error) {
		exit = 1
		con.InsertConst(-1, fmt.Sprintf("%v: error: %v.", f.Name(), err))
	})
	downloader.Start()
	wg.Wait()
	return exit
}

func cmdServer(args []string, opts *options) int {
	logrus.SetOutput(os.Stderr)
	server := server.On(opts.Server.BindAddr, opts.Server.Port)
	useAccounts(server)
	if server.Port != 9666 {
		logrus.Warn("Click'n'Load v2 will only work for port 9666!")
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

func cmdPs(args []string, opts *options) int {
	if len(args) > 0 {
		fmt.Fprintln(os.Stderr, "What are you passing arguments for?")
		return 1
	}
	if files := requestMany("get", opts.Ps.Host, "/containers", nil); files != nil {
		if cmd, _ := setupPager(len(files)); cmd != nil {
			defer cmd.Wait()
			defer os.Stdout.Close()
		}
		for _, file := range files {
			fmt.Printf("%s    %s\n", file["id"].(string)[:12], file["name"])
		}
		return 0
	}
	return 1
}

func cmdPush(args []string, opts *options) int {
	urls := grabURLs(args, opts.Push.urlArgs)
	if urls == nil {
		return 1
	}
	links := make([]string, len(urls))
	for i, u := range urls {
		links[i] = u.String()
	}
	bs, _ := json.Marshal(links)
	fmt.Println(requestOne("post", opts.Push.Host, "/containers", bytes.NewReader(bs))["id"])
	return 0
}

func cmdRm(args []string, opts *options) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Only one argument 'id' permitted.")
		return 1
	}
	fmt.Println(requestOne("delete", opts.Rm.Host, "/containers/"+args[0], nil))
	return 0
}
