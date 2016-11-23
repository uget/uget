package cli

import (
	"bufio"
	"fmt"
	"github.com/howeyc/gopass"
	"github.com/uget/uget/core"
	"os"
	"strings"
)

type CliPrompter struct {
	prefix    string
	overrides map[string]string
}

func NewCliPrompter(prefix string, overrides map[string]string) *CliPrompter {
	return &CliPrompter{prefix, overrides}
}

func (c CliPrompter) Get(fields []core.Field) map[string]string {
	reader := bufio.NewReader(os.Stdin)
	values := map[string]string{}
	for _, field := range fields {
		if value, ok := c.overrides[field.Key]; ok {
			values[field.Key] = value
		} else {
			var deftext string = ""
			if field.Value != "" {
				deftext = fmt.Sprintf(" (%s)", field.Value)
			}
			fmt.Printf("[%s] %s%s: ", c.prefix, field.Display, deftext)
			var entered string
			if field.Sensitive {
				t, err := gopass.GetPasswd()
				if err != nil {
					c.Error(err.Error())
					return nil
				}
				entered = string(t)
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
	fmt.Fprintf(os.Stderr, "[%s] Error: %s\n", c.prefix, display)
}

func (c CliPrompter) Success() {
	fmt.Fprintf(os.Stderr, "[%s] Success!\n", c.prefix)
}
