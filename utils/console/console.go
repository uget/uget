package console

import (
	"fmt"
	"os"
	"strings"
)

// Row - line number / ID in console
type Row int

// Console represents the TTY
type Console struct {
	File     *os.File
	jobs     chan func()
	rowCount int
}

// NewConsole initializes a Console object
func NewConsole() *Console {
	c := &Console{
		os.Stdout,
		make(chan func()),
		0,
	}
	go c.dispatch()
	return c
}

func (c *Console) dispatch() {
	for job := range c.jobs {
		job()
	}
}

// AddRow adds a row at the end of the console
func (c *Console) AddRow(text string) Row {
	idch := make(chan Row)
	c.jobs <- func() {
		idch <- c.addRow(text)
	}
	return <-idch
}

// AddRows adds multiple rows at the end of the console
func (c *Console) AddRows(texts ...string) []Row {
	idsch := make(chan []Row)
	c.jobs <- func() {
		ids := make([]Row, len(texts))
		for i, text := range texts {
			ids[i] = c.addRow(text)
		}
		idsch <- ids
	}
	return <-idsch
}

func (c *Console) addRow(text string) Row {
	id := Row(c.rowCount)
	c.rowCount++
	fmt.Fprintf(c.File, "%s\n", strings.TrimSpace(text))
	return id
}

// EditRow replaces the given row with the given string
func (c *Console) EditRow(id Row, text string) {
	ch := make(chan struct{})
	c.jobs <- func() {
		diff := c.rowCount - int(id)
		fmt.Fprintf(c.File, "%c[%dA", 27, diff)
		fmt.Fprintf(c.File, "\r%c[2K", 27)
		fmt.Fprintf(c.File, "%s\n", strings.TrimSpace(text))
		fmt.Fprintf(c.File, "%c[%dB", 27, diff)
		close(ch)
	}
	<-ch
}
