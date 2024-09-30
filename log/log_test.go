package log

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

type logLines struct {
	entries []logEntry
}

func assertEntry(t *testing.T, want logEntry, lines *logLines) {
	t.Helper()
	if len(lines.entries) == 0 {
		t.Fatalf("No log entries")
	}
	if len(lines.entries) > 1 {
		t.Fatalf("More than one log entry")
	}
	got := lines.entries[0]
	lines.entries = make([]logEntry, 0)

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

func captureLogger() (Logger, *logLines) {
	ll := &logLines{}
	format := func(level string, msg string, fields map[string]interface{}) {
		ll.entries = append(ll.entries, logEntry{
			Level:   level,
			Time:    time.Now(),
			Message: msg,
			Fields:  fields,
		})
	}
	return &CallbackLogger{
		Callback:   format,
		Collectors: []ContextCollector{DefaultContext},
	}, ll
}

const (
	debugLevel = "DEBUG"
	infoLevel  = "INFO"
	errorLevel = "ERROR"
)

func TestDefaultLogger(t *testing.T) {
	logger, entries := captureLogger()
	DefaultLogger = logger
	logger.SetLevel(slog.LevelDebug)

	ctx := context.Background()

	Debug(ctx, "Message")
	assertEntry(t, logEntry{Message: "Message", Level: debugLevel}, entries)

	Debugf(ctx, "Message %s", "string")
	assertEntry(t, logEntry{Message: "Message string", Level: debugLevel}, entries)

	Info(ctx, "Message")
	assertEntry(t, logEntry{Message: "Message", Level: infoLevel}, entries)

	Infof(ctx, "Message %s", "string")
	assertEntry(t, logEntry{Message: "Message string", Level: infoLevel}, entries)

	Error(ctx, "Message")
	assertEntry(t, logEntry{Message: "Message", Level: errorLevel}, entries)

	Errorf(ctx, "Message %s", "string")
	assertEntry(t, logEntry{Message: "Message string", Level: errorLevel}, entries)

}

func TestContext(t *testing.T) {
	logger, entries := captureLogger()
	logger.SetLevel(slog.LevelDebug)

	ctx := context.Background()

	t.Run("TestWithField", func(t *testing.T) {
		logger.Debug(WithField(ctx, "key", "value"), "Message")
		assertEntry(t, logEntry{
			Message: "Message",
			Level:   debugLevel,
			Fields:  map[string]interface{}{"key": "value"},
		}, entries)
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
		}, entries)
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
		}, entries)
	})
}
