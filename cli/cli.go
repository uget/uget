package cli

import (
	"fmt"
	"github.com/jessevdk/go-flags"
	"os"
)

/* CLI specification */

type Options struct {
	Accounts Accounts `command:"accounts"`
	Get      Get      `command:"get"`
	Server   Server   `command:"server"`
	Daemon   Daemon   `command:"daemon"`
	Push     Push     `command:"push"`
	Unknowns map[string]string
}

type Server struct {
	Port     uint16 `short:"p" long:"port" description:"port the server listens on" default:"9666"`
	BindAddr string `short:"b" long:"bind" description:"address to bind the server to"`
}

type Get struct {
	NoSkip bool `short:"S" long:"no-skip" description:"Don't skip files that already exist"`
	Inline bool `short:"i" long:"inline" description:"Interpret arguments as URLs (instead of files)"`
}
type Daemon struct{}
type Push struct{}

type Accounts struct {
	Add    AccountsAdd    `command:"add"`
	List   AccountsList   `command:"list"`
	Select AccountsSelect `command:"select"`
}
type AccountsAdd struct{}
type AccountsList struct{}
type AccountsSelect struct{}

/* Commands */

func (cmd *AccountsAdd) Execute(args []string) error {
	return command(args, CmdAddAccount)
}

func (cmd *AccountsList) Execute(args []string) error {
	return command(args, CmdListAccounts)
}

func (cmd *AccountsSelect) Execute(args []string) error {
	return command(args, CmdSelectAccounts)
}

func (cmd *Get) Execute(args []string) error {
	return command(args, CmdGet)
}

func (cmd *Server) Execute(args []string) error {
	return command(args, CmdServer)
}

func (cmd *Daemon) Execute(args []string) error {
	return command(args, CmdDaemon)
}

func (cmd *Push) Execute(args []string) error {
	return command(args, CmdPush)
}

/* Hack to facilitate calling commands with options. */
type Command func(*Options) int

func (c Command) Error() string {
	return ""
}

func command(args []string, f func([]string, *Options) int) error {
	return Command(func(opt *Options) int {
		return f(args, opt)
	})
}

// Sets up parser and runs app with passed arguments. Returns exit code.
func RunApp(arguments []string) int {
	opts := &Options{
		Unknowns: map[string]string{},
	}
	parser := flags.NewParser(opts, flags.Default^flags.PrintErrors)
	parser.Name = arguments[0]
	parser.UnknownOptionHandler = func(opt string, split flags.SplitArgument, args []string) ([]string, error) {
		arg, ok := split.Value()
		if ok {
			opts.Unknowns[opt] = arg
			return args, nil
		} else {
			opts.Unknowns[opt] = args[0]
			return args[1:], nil
		}
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
