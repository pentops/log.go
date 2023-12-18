package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/pentops/log.go/pretty"
)

func main() {
	fmt.Printf("LogCat Begin\n")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Split(bufio.ScanLines)

	printer := pretty.NewPrinter(os.Stdout)

	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 1 {
			continue
		}
		before, after, found := strings.Cut(line, " | ")
		if !found {
			after = before
			before = ""
		}
		printer.PrintRawLine(before, after)
	}
}
