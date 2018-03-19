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
	File      *os.File
	jobs      chan func()
	rows      map[int]func() string
	length    int
	neglength int
}

// Updating initializes an UpdatingConsole object that updates
// the console every `update`
func Updating(update time.Duration) *UpdatingConsole {
	c := &UpdatingConsole{
		os.Stdout,
		make(chan func()),
		map[int]func() string{},
		0,
		0,
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
		if at == math.MaxInt64 {
			at = c.length
		}
		c.insert(at, text)
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

func (c *UpdatingConsole) insert(at int, text func() string) {
	c.ensure(at)
	if at < 0 {
		at += c.length
	}
	for i := at; i < c.length; i++ {
		c.rows[i], text = text, c.rows[i]
		c.update(i)
	}
	c.rows[c.length] = text
	c.update(c.length)
}

func (c *UpdatingConsole) SetFixed(at int, text func() string) {
	c.jobs <- func() {
		c.rows[at] = text
		c.update(at)
	}
}

func (c *UpdatingConsole) ensure(at int) {
	if at >= c.length {
		fmt.Println(strings.Repeat("\n", at-c.length))
		c.length = at + 1
	} else if -at > c.neglength {
		fmt.Println(strings.Repeat("\n", -at-c.neglength-1))
		c.neglength = -at
	}
}

func (c *UpdatingConsole) update(lineNo int) {
	c.ensure(lineNo)
	textF := c.rows[lineNo]
	if textF == nil {
		return
	}
	if lineNo < 0 {
		lineNo += c.length + c.neglength + 1
	}
	text := textF()
	diff := (c.length + c.neglength) - lineNo
	h, _, err := GetWinSize()
	if err == nil && diff >= int(h) {
		return
	}
	fmt.Fprintf(c.File, "%c[%dA", 27, diff)
	fmt.Fprintf(c.File, "\r%c[2K", 27)
	fmt.Fprintf(c.File, "%s\n", strings.TrimSpace(text))
	fmt.Fprintf(c.File, "%c[%dB", 27, diff)
}

func (c *UpdatingConsole) updateAll() {
	for line := range c.rows {
		c.update(line)
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
