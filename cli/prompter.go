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
	fields  []*core.Field
	prefix  string
}

func NewCliPrompter(c *cli.Context, prefix string) *CliPrompter {
	return &CliPrompter{
		Context: c,
		fields:  make([]*core.Field, 0, 5),
		prefix:  prefix,
	}
}

func (c *CliPrompter) Define(f *core.Field) {
	c.fields = append(c.fields, f)
}

func (c CliPrompter) Get() map[string]string {
	reader := bufio.NewReader(os.Stdin)
	values := map[string]string{}
	// fmt.Printf("field: %+v\n", c.fields)
	for _, field := range c.fields {
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
