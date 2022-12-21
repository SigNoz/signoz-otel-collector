package signozcol

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestCollectorNew(t *testing.T) {
	coll := New(WrappedCollectorSettings{
		ConfigPaths: []string{"testdata/config.yaml"},
		Version:     "0.0.1",
		Desc:        "test",
		LoggingOpts: []zap.Option{zap.AddStacktrace(zapcore.ErrorLevel)},
	})
	if coll == nil {
		t.Fatal("coll is nil")
	}
}

func TestCollectorNewInvalidPath(t *testing.T) {
	coll := New(WrappedCollectorSettings{
		ConfigPaths: []string{"testdata/invalid.yaml"},
		Version:     "0.0.1",
		Desc:        "test",
		LoggingOpts: []zap.Option{zap.AddStacktrace(zapcore.ErrorLevel)},
	})
	if coll == nil {
		t.Fatal("coll is nil")
	}

	err := coll.Run(context.TODO())
	defer coll.Shutdown()
	if err == nil {
		t.Fatal("expected error")
	}
	assert.Contains(t, err.Error(), "no such file or directory")
}

func TestCollectorNewValidPath(t *testing.T) {
	coll := New(WrappedCollectorSettings{
		ConfigPaths: []string{"testdata/config.yaml"},
		Version:     "0.0.1",
		Desc:        "test",
		LoggingOpts: []zap.Option{zap.AddStacktrace(zapcore.ErrorLevel)},
	})
	if coll == nil {
		t.Fatal("coll is nil")
	}

	err := coll.Run(context.TODO())
	defer coll.Shutdown()

	if err != nil {
		t.Fatal(err)
	}
}

func TestCollectorNewValidPathInvalidConfig(t *testing.T) {
	coll := New(WrappedCollectorSettings{
		ConfigPaths: []string{"testdata/invalid_config.yaml"},
		Version:     "0.0.1",
		Desc:        "test",
		LoggingOpts: []zap.Option{zap.AddStacktrace(zapcore.ErrorLevel)},
	})
	if coll == nil {
		t.Fatal("coll is nil")
	}

	err := coll.Run(context.TODO())
	defer coll.Shutdown()

	if err == nil {
		t.Fatal("expected error")
	}
	assert.Contains(t, err.Error(), "invalid configuration")
}

func TestCollectorShutdown(t *testing.T) {
	coll := New(WrappedCollectorSettings{
		ConfigPaths: []string{"testdata/config.yaml"},
		Version:     "0.0.1",
		Desc:        "test",
		LoggingOpts: []zap.Option{zap.AddStacktrace(zapcore.ErrorLevel)},
	})
	if coll == nil {
		t.Fatal("coll is nil")
	}

	err := coll.Run(context.TODO())
	defer coll.Shutdown()

	if err != nil {
		t.Fatal(err)
	}

	coll.Shutdown()
	// should evntually return no error
}

func TestCollectorRunMultipleTimes(t *testing.T) {
	coll := New(WrappedCollectorSettings{
		ConfigPaths: []string{"testdata/config.yaml"},
		Version:     "0.0.1",
		Desc:        "test",
		LoggingOpts: []zap.Option{zap.AddStacktrace(zapcore.ErrorLevel)},
	})
	if coll == nil {
		t.Fatal("coll is nil")
	}

	err := coll.Run(context.TODO())
	defer coll.Shutdown()

	if err != nil {
		t.Fatal(err)
	}

	err = coll.Run(context.TODO())
	if err == nil {
		t.Fatal("expected error")
	}
	assert.Contains(t, err.Error(), "already running")
}
