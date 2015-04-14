package console

import (
	"fmt"
	"os"
	"strings"
)

type Row int

type Console struct {
	File     *os.File
	jobs     chan func()
	rowCount int
}

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

func (c *Console) AddRow(text string) Row {
	idch := make(chan Row)
	c.jobs <- func() {
		idch <- c.addRow(text)
	}
	return <-idch
}

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
