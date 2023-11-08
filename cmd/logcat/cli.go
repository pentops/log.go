package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/fatih/color"
)

func main() {
	fmt.Printf("LogCat Begin\n")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Split(bufio.ScanLines)

	lastLine := map[string]interface{}{}
	didDots := false
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 1 {
			continue
		}
		before, after, found := strings.Cut(line, " | ")
		if !found {
			after = before
		}
		if after[0] != '{' {
			if didDots {
				fmt.Printf("\n")
			}
			didDots = false
			lastLine = map[string]interface{}{}
			fmt.Println(line)
			continue
		}

		if found {
			fmt.Printf("%s\n", before)
		}

		fields := map[string]interface{}{}
		err := json.Unmarshal([]byte(after), &fields)
		if err != nil {
			fmt.Printf("<invalid JSON> %s\n", line)
			continue
		}

		if _, ok := fields["time"]; ok {
			delete(fields, "time")
		}

		if reflect.DeepEqual(fields, lastLine) {
			fmt.Printf(".")
			didDots = true
			continue
		}
		lastLine = fields
		if didDots {
			fmt.Printf("\n")
		}
		didDots = false

		level, hasLevel := fields["level"].(string)
		message, hasMessage := fields["message"].(string)
		innerFields, hasFields := fields["fields"].(map[string]interface{})

		if hasLevel && hasMessage && hasFields && len(fields) == 3 {
			fields = innerFields
			whichColor, ok := levelColors[strings.ToLower(level)]
			if !ok {
				whichColor = color.FgWhite
			}

			levelColor := color.New(whichColor).SprintFunc()
			fmt.Printf("%s: %s\n", levelColor(level), message)
		}

		for k, v := range fields {
			switch v.(type) {
			case string, int, int64, int32, float64, bool:
				fmt.Printf("| %s: %v\n", k, v)
			default:
				nice, _ := json.MarshalIndent(v, "|  ", "  ")
				fmt.Printf("| %s: %s\n", k, string(nice))
			}
		}

	}

}

var levelColors = map[string]color.Attribute{
	"debug": color.FgBlue,
	"info":  color.FgGreen,
	"warn":  color.FgYellow,
	"error": color.FgRed,
}
