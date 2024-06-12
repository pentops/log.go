package pretty

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/fatih/color"
	"github.com/pentops/log.go/log"
)

var levelColors = map[string]color.Attribute{
	"debug": color.FgBlue,
	"info":  color.FgGreen,
	"warn":  color.FgYellow,
	"error": color.FgRed,
}

type Printer struct {
	prefix   string
	output   io.Writer
	didDots  bool
	lastLine map[string]interface{}
}

func WithPrefix(prefix string) func(*Printer) {
	return func(p *Printer) {
		p.prefix = prefix
	}
}

func NewPrinter(output io.Writer, opts ...func(*Printer)) *Printer {
	pp := &Printer{
		output: output,
	}

	for _, opt := range opts {
		opt(pp)
	}

	return pp
}

func (p *Printer) writef(namePrefix, line string, args ...interface{}) {
	if p.didDots {
		fmt.Fprintf(p.output, "\n")
		p.didDots = false
	}
	fmt.Fprintf(p.output, "========\n")
	if len(args) > 0 {
		line = fmt.Sprintf(line, args...)
	}
	if p.prefix != "" {
		namePrefix = p.prefix + " " + namePrefix
	}

	if namePrefix != "" {
		fmt.Fprintf(p.output, "%s: %s\n", namePrefix, line)
	} else {
		fmt.Fprintf(p.output, "%s\n", line)
	}
}

func (p *Printer) CallbackWithPrefix(prefix string) log.LogFunc {
	return log.LogFunc(func(level string, message string, fields map[string]interface{}) {
		p.PrintStandardLine(prefix, level, message, fields)
	})
}

func (p *Printer) PrintStandardLine(namePrefix, level, message string, fields map[string]interface{}) {
	whichColor, ok := levelColors[strings.ToLower(level)]
	if !ok {
		whichColor = color.FgWhite
	}

	levelColor := color.New(whichColor).SprintFunc()
	p.writef(namePrefix, "%s: %s", levelColor(level), message)

	for k, v := range fields {
		switch v.(type) {
		case string, int, int64, int32, float64, bool:
			fmt.Fprintf(p.output, "| %s: %v\n", k, v)
		default:
			nice, _ := json.MarshalIndent(v, "|  ", "  ")
			fmt.Fprintf(p.output, "| %s: %s\n", k, string(nice))
		}
	}
}

type writeBuffer struct {
	buffer  []byte
	printer *Printer
	prefix  string
}

func (p *writeBuffer) Write(data []byte) (int, error) {
	p.buffer = append(p.buffer, data...)

	if strings.Contains(string(p.buffer), "\n") {
		lines := strings.Split(string(p.buffer), "\n")
		for _, line := range lines[:len(lines)-1] {
			if line == "" {
				continue
			}
			p.printer.PrintRawLine(p.prefix, line)
		}
		p.buffer = []byte(lines[len(lines)-1])
	}

	return len(data), nil
}

func (p *Printer) WriterInterceptor(prefix string) io.Writer {
	return &writeBuffer{
		buffer:  []byte{},
		prefix:  prefix,
		printer: p,
	}
}

func (p *Printer) PrintRawLine(namePrefix, line string) {

	if line[0] != '{' {
		p.didDots = false
		p.lastLine = map[string]interface{}{}
		p.writef(namePrefix, line)
		return
	}

	fields := map[string]interface{}{}
	err := json.Unmarshal([]byte(line), &fields)
	if err != nil {
		p.writef(namePrefix, "<invalid JSON> %s", line)
		return
	}

	delete(fields, "time")

	if reflect.DeepEqual(fields, p.lastLine) {
		p.output.Write([]byte(".")) // nolint: errcheck
		p.didDots = true
		return
	}
	p.lastLine = fields
	if p.didDots {
		fmt.Printf("\n")
	}
	p.didDots = false

	level, hasLevel := fields["level"].(string)
	message, hasMessage := fields["message"].(string)
	innerFields, hasFields := fields["fields"].(map[string]interface{})

	if hasLevel && hasMessage && hasFields && len(fields) == 3 {
		p.PrintStandardLine(namePrefix, level, message, innerFields)
	} else {
		p.writef(namePrefix, line)
	}
}
