package log

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
)

func TestSimpleFormatter(t *testing.T) {

	buff := bytes.NewBuffer([]byte{})

	jsonFormatter(buff, logEntry{
		Level:   "DEBUG",
		Message: "Message",
		Fields: map[string]interface{}{
			"key": "val",
		},
	})

	loggedJSON := buff.Bytes()
	logged := map[string]interface{}{}
	if err := json.Unmarshal(loggedJSON, &logged); err != nil {
		t.Fatalf("Error decoding log message: %s", err.Error())
	}

	if logged["message"] != "Message" || logged["level"] != "DEBUG" {
		t.Errorf("Bad log message: %s", string(loggedJSON))
	}

}

type nojson struct{}

func (nojson) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("Error")
}

func TestSimpleFormatterError(t *testing.T) {

	buff := bytes.NewBuffer([]byte{})

	jsonFormatter(buff, logEntry{
		Level:   "DEBUG",
		Message: "Message",
		Fields: map[string]interface{}{
			"key": nojson{},
		},
	})

	loggedJSON := buff.Bytes()
	logged := map[string]interface{}{}
	if err := json.Unmarshal(loggedJSON, &logged); err != nil {
		t.Fatalf("Error decoding log message: %s", err.Error())
	}

	if logged["message"] != "Message" || logged["level"] != "DEBUG" {
		t.Errorf("Bad log message: %s", string(loggedJSON))
	}

}
