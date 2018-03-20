package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/jessevdk/go-flags"
	"github.com/uget/uget/core"
)

/* CLI specification */

type options struct {
	Accounts accounts `command:"accounts"`
	Get      get      `command:"get"`
	Resolve  resolve  `command:"meta"`
	Server   server   `command:"server"`
	Daemon   daemon   `command:"daemon"`
	Push     push     `command:"push"`
	Version  version  `command:"version"`
	Unknowns map[string]string
}

type version struct{}

type server struct {
	Port     uint16 `short:"p" long:"port" description:"port the server listens on" default:"9666"`
	BindAddr string `short:"b" long:"bind" description:"address to bind the server to"`
}

type urlArgs struct {
	Inline bool `short:"i" long:"inline" description:"Interpret arguments as URLs (instead of files)"`
}

type get struct {
	*urlArgs
	DryRun     bool `short:"n" long:"dry-run" description:"Just output instead of downloading."`
	NoContinue bool `short:"C" long:"no-continue" description:"Redownload entire file instead of continuing previous download."`
	NoSkip     bool `short:"S" long:"no-skip" description:"Redownload file even if size is correct"`
	Jobs       int  `short:"j" long:"jobs" default:"3" description:"Jobs to run in parallel"`
}

type resolve struct {
	*urlArgs
	Full    bool `short:"f" long:"full" description:"List all available information"`
	Compare bool `short:"c" long:"compare" description:"Compare file checksums."`
	Remove  bool `short:"r" long:"remove" description:"Remove local files that cannot be equal to remote (implies -c)."`
}

type daemon struct{}
type push struct{}

type accounts struct {
	Add     accountsAdd     `command:"add"`
	List    accountsList    `command:"list"`
	Disable accountsDisable `command:"disable"`
	Enable  accountsEnable  `command:"enable"`
}
type accountsAdd struct{}
type accountsList struct{}
type accountsDisable struct{}
type accountsEnable struct{}

/* Commands */

func (v *version) Execute(args []string) error {
	return command(args, cmdVersion)
}

func (cmd *accountsAdd) Execute(args []string) error {
	return command(args, cmdAddAccount)
}

func (cmd *accountsList) Execute(args []string) error {
	return command(args, cmdListAccounts)
}

func (cmd *accountsDisable) Execute(args []string) error {
	return command(args, cmdDisableAccount)
}

func (cmd *accountsEnable) Execute(args []string) error {
	return command(args, cmdEnableAccount)
}

func (cmd *get) Execute(args []string) error {
	return command(args, cmdGet)
}

func (cmd *resolve) Execute(args []string) error {
	return command(args, cmdResolve)
}

func (cmd *server) Execute(args []string) error {
	return command(args, cmdServer)
}

func (cmd *daemon) Execute(args []string) error {
	return command(args, cmdDaemon)
}

func (cmd *push) Execute(args []string) error {
	return command(args, cmdPush)
}

// Command facilitates calling commands with options.
type Command func(*options) int

func (c Command) Error() string {
	return ""
}

func command(args []string, f func([]string, *options) int) error {
	return Command(func(opt *options) int {
		return f(args, opt)
	})
}

// RunApp sets up parser and runs app with passed arguments. Returns exit code.
func RunApp(arguments []string) int {
	logrus.Infof("==== uget %v - %v ====", core.Version, time.Now().Local().Format("15:04:05"))
	logrus.Infof("==== running with args %s", core.Version, strings.Join(arguments[1:], " "))
	opts := &options{
		Unknowns: map[string]string{},
	}
	parser := flags.NewParser(opts, flags.Default^flags.PrintErrors)
	parser.Name = arguments[0]
	parser.Find("meta").Aliases = []string{"resolve"}
	parser.UnknownOptionHandler = func(opt string, split flags.SplitArgument, args []string) ([]string, error) {
		arg, ok := split.Value()
		if ok {
			opts.Unknowns[opt] = arg
			return args, nil
		}
		if len(args) == 0 {
			return args, nil
		}
		opts.Unknowns[opt] = args[0]
		return args[1:], nil
	}
	_, err := parser.ParseArgs(arguments[1:])
	if err != nil {
		if e, ok := err.(*flags.Error); ok {
			if e.Type == flags.ErrHelp {
				fmt.Println(e.Message)
				return 0
			}
		} else {
			cmd, ok := err.(Command)
			if ok {
				return cmd(opts)
			}
		}
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	fmt.Fprintln(os.Stderr, "INVALID USAGE. Return Command!")
	return 2
}
