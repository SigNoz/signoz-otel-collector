package opamp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
)

type wrappedCollectorMock struct {
	runCalled     bool
	stopCalled    bool
	restartCalled bool
	mock.Mock
}

func (m *wrappedCollectorMock) Run(ctx context.Context) error {
	m.runCalled = true
	return nil
}

func (m *wrappedCollectorMock) Shutdown() {
	m.stopCalled = true
}

func (m *wrappedCollectorMock) Restart(ctx context.Context) error {
	m.restartCalled = true
	return nil
}

func TestNopClientWithCollector(t *testing.T) {
	coll := &wrappedCollectorMock{}
	client := NewSimpleClient(coll)

	err := client.Start(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !coll.runCalled {
		t.Errorf("expected collector to be run")
	}

	err = client.Stop(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !coll.stopCalled {
		t.Errorf("expected collector to be stopped")
	}
}
