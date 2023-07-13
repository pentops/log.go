package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
)

func main() {

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 1 {
			continue
		}
		if line[0] != '{' {
			fmt.Println(line)
			continue
		}

		fields := map[string]interface{}{}
		err := json.Unmarshal([]byte(line), &fields)
		if err != nil {
			fmt.Printf("<invalid JSON> %s\n", line)
			continue
		}

		if _, ok := fields["time"]; ok {
			delete(fields, "time")
		}

		level, hasLevel := fields["level"].(string)
		message, hasMessage := fields["message"].(string)
		innerFields, hasFields := fields["fields"].(map[string]interface{})

		if hasLevel && hasMessage && hasFields && len(fields) == 3 {
			delete(fields, "level")
			delete(fields, "message")
			delete(fields, "fields")
			fields = innerFields
			whichColor, ok := levelColors[strings.ToLower(level)]
			if !ok {
				whichColor = color.FgWhite
			}

			levelColor := color.New(whichColor).SprintFunc()
			fmt.Printf("%s: %s\n", levelColor(level), message)
		}

		nice, _ := json.MarshalIndent(fields, "|  ", "  ")
		fmt.Printf("| %s\n", string(nice))

	}

}

var levelColors = map[string]color.Attribute{
	"debug": color.FgWhite,
	"info":  color.FgGreen,
	"warn":  color.FgYellow,
	"error": color.FgRed,
}
