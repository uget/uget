package cli

import (
	"bufio"
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/howeyc/gopass"
	"github.com/uget/uget/core"
	"os"
	"strings"
)

type CliPrompter struct {
	Context *cli.Context
	prefix  string
}

func NewCliPrompter(c *cli.Context, prefix string) *CliPrompter {
	return &CliPrompter{
		Context: c,
		prefix:  prefix,
	}
}

func (c CliPrompter) Get(fields []core.Field) map[string]string {
	reader := bufio.NewReader(os.Stdin)
	values := map[string]string{}
	for _, field := range fields {
		if c.Context.IsSet("--" + field.Key) {
			values[field.Key] = c.Context.String("--" + field.Key)
		} else {
			var deftext string = ""
			if field.Value != "" {
				deftext = fmt.Sprintf(" (%s)", field.Value)
			}
			fmt.Printf("[%s] %s%s: ", c.prefix, field.Display, deftext)
			var entered string
			if field.Sensitive {
				entered = string(gopass.GetPasswd())
			} else {
				line, err := reader.ReadString('\n')
				if err != nil {
					c.Error(err.Error())
					return nil
				}
				entered = strings.TrimSpace(string(line))
			}
			if entered == "" {
				entered = field.Value
			}
			values[field.Key] = entered
		}
	}
	return values
}

func (c CliPrompter) Error(display string) {
	fmt.Fprintf(os.Stderr, "[%s] Error occurred: %s\n", c.prefix, display)
}

func (c CliPrompter) Success() {
	fmt.Fprintf(os.Stderr, "[%s] Success!\n", c.prefix)
}
