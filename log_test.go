package log

import (
	"context"
	"io"
	"testing"
)

type testLogger chan logEntry

func assertEntry(t *testing.T, want logEntry, got logEntry) {
	t.Helper()
	if want.Level != got.Level {
		t.Errorf("Want level %s got %s", want.Level, got.Level)
	}
	if want.Message != got.Message {
		t.Errorf(`Want message: "%s" got "%s"`, want.Message, got.Message)
	}
	for key, wantVal := range want.Fields {
		gotVal, ok := got.Fields[key]
		if !ok {
			t.Errorf("No key %s in fields", key)
		}
		if gotVal != wantVal {
			t.Errorf(
				"In key %s, want %#v (%T) got %#v (%T)",
				key,
				wantVal, wantVal,
				gotVal, gotVal,
			)
		}
	}

}

func captureLogger() (simpleLogger, chan logEntry) {
	entries := make(chan logEntry, 1)
	format := func(out io.Writer, entry logEntry) {
		entries <- entry
	}
	return simpleLogger{
		format: format,
	}, entries
}

func TestDefaultLogger(t *testing.T) {
	logger, entries := captureLogger()
	DefaultLogger = logger

	ctx := context.Background()

	Debug(ctx, "Message")
	assertEntry(t, logEntry{Message: "Message", Level: debugLevel}, <-entries)

	Debugf(ctx, "Message %s", "string")
	assertEntry(t, logEntry{Message: "Message string", Level: debugLevel}, <-entries)

	Info(ctx, "Message")
	assertEntry(t, logEntry{Message: "Message", Level: infoLevel}, <-entries)

	Infof(ctx, "Message %s", "string")
	assertEntry(t, logEntry{Message: "Message string", Level: infoLevel}, <-entries)

	Error(ctx, "Message")
	assertEntry(t, logEntry{Message: "Message", Level: errorLevel}, <-entries)

	Errorf(ctx, "Message %s", "string")
	assertEntry(t, logEntry{Message: "Message string", Level: errorLevel}, <-entries)

}

func TestContext(t *testing.T) {
	logger, entries := captureLogger()

	ctx := context.Background()

	t.Run("TestWithField", func(t *testing.T) {
		logger.Debug(WithField(ctx, "key", "value"), "Message")
		assertEntry(t, logEntry{
			Message: "Message",
			Level:   debugLevel,
			Fields:  map[string]interface{}{"key": "value"},
		}, <-entries)
	})

	t.Run("TestWithFields", func(t *testing.T) {
		ctx := WithFields(ctx, map[string]interface{}{"key": "value"})
		logger.Debug(ctx, "Message")
		assertEntry(t, logEntry{
			Message: "Message",
			Level:   debugLevel,
			Fields: map[string]interface{}{
				"key": "value",
			},
		}, <-entries)
	})

	t.Run("TestOverrideMerge", func(t *testing.T) {
		ctx := WithFields(ctx, map[string]interface{}{
			"1": "A",
			"2": "A",
		})
		ctx = WithFields(ctx, map[string]interface{}{
			"2": "B",
			"3": "B",
		})
		logger.Debug(ctx, "Message")
		assertEntry(t, logEntry{
			Message: "Message",
			Level:   debugLevel,
			Fields: map[string]interface{}{
				"1": "A",
				"2": "B",
				"3": "B",
			},
		}, <-entries)
	})
}
