package console

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"
)

// UpdatingConsole manages output to the TTY in a fixed-interval
type UpdatingConsole struct {
	File *os.File
	jobs chan func()
	rows map[uint]func() string
}

// Updating initializes an UpdatingConsole object that updates
// the console every `update`
func Updating(update time.Duration) *UpdatingConsole {
	c := &UpdatingConsole{
		os.Stdout,
		make(chan func()),
		map[uint]func() string{},
	}
	go c.dispatch(update)
	return c
}

// AddConst is a convenience method for Add with an invariable string.
func (c *UpdatingConsole) AddConst(text string) {
	c.InsertConst(math.MaxInt64, text)
}

// AddConst is a convenience method for Add with an invariable string.
func (c *UpdatingConsole) InsertConst(at int, text string) {
	c.Insert(at, func() string { return text })
}

// Add adds a row at the end of the console.
func (c *UpdatingConsole) Add(text func() string) {
	c.Insert(math.MaxInt64, text)
}

// Insert adds the specified string at the given line number.
// Can be a negative number for negative indexing
// if line number exceeds total amount of lines, row will be inserted at the end instead.
func (c *UpdatingConsole) Insert(at int, text func() string) {
	c.jobs <- func() {
		if at < 0 {
			at += len(c.rows)
		} else if at > len(c.rows) {
			at = len(c.rows)
		}
		c.insert(uint(at), text)
	}
}

// Close releases this console's goroutines. Updates if true is passed.
func (c *UpdatingConsole) Close(update bool) {
	if update {
		done := make(chan interface{})
		c.jobs <- func() {
			c.updateAll()
			close(done)
		}
		<-done
	}
	close(c.jobs)
}

func (c *UpdatingConsole) insert(at uint, text func() string) {
	length := uint(len(c.rows))
	for i := at; i < length; i++ {
		c.rows[i], text = text, c.rows[at]
		c.update(at)
	}
	c.rows[length] = text
	fmt.Println()
	c.update(length)
}

func (c *UpdatingConsole) update(lineNo uint) {
	text := c.rows[lineNo]()
	diff := uint(len(c.rows)) - lineNo
	fmt.Fprintf(c.File, "%c[%dA", 27, diff)
	fmt.Fprintf(c.File, "\r%c[2K", 27)
	fmt.Fprintf(c.File, "%s\n", strings.TrimSpace(text))
	fmt.Fprintf(c.File, "%c[%dB", 27, diff)
}

func (c *UpdatingConsole) updateAll() {
	length := uint(len(c.rows))
	for lineNo := uint(0); lineNo < length; lineNo++ {
		c.update(lineNo)
	}
}

func (c *UpdatingConsole) dispatch(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case job, ok := <-c.jobs:
			if !ok {
				return
			}
			job()
		case <-ticker.C:
			c.updateAll()
		}
	}
}
