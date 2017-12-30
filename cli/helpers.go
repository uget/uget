package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/uget/uget/core"
)

func linksFromFile(links *[]string, f string) error {
	file, err := os.Open(f)
	if err != nil {
		log.WithField("file", f).Error("could not open")
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		*links = append(*links, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func selectPProvider(arg string) core.PersistentProvider {
	if arg == "" {
		ps := make([]string, 0)
		for _, p := range core.AllProviders() {
			if pp, ok := p.(core.PersistentProvider); ok {
				ps = append(ps, pp.Name())
			}
		}
		i := userSelection(ps, "Choose a provider")
		if i < 0 {
			fmt.Fprintln(os.Stderr, "Invalid selection.")
			os.Exit(1)
		}
		arg = ps[i]
	}
	provider := core.GetProvider(arg).(core.PersistentProvider)
	if provider == nil {
		fmt.Printf("No provider found for %s\n", arg)
	}
	return provider
}

func userSelection(arr []string, prompt string) int {
	for i, x := range arr {
		fmt.Printf("- %s (%v)\n", x, i+1)
	}
	i := -1
	fmt.Printf("%s: ", prompt)
	buf := make([]byte, 256)
	read, err := os.Stdin.Read(buf)
	if err != nil {
		panic(err)
	}
	str := strings.TrimSpace(string(buf[:read]))
	if len(str) > 0 {
		if _, err := fmt.Sscanf(str, "%d", &i); err != nil {
			for index, s := range arr {
				if s == str {
					i = index + 1
					break
				}
			}
		}
	}
	return i - 1
}
