package service

import (
	"context"
	"fmt"

	"github.com/SigNoz/signoz-otel-collector/opamp"
	"github.com/SigNoz/signoz-otel-collector/signozcol"
	"go.uber.org/zap"
)

// Service is the interface for the Opamp service
// which manages the Opamp connection and collector
// lifecycle
//
// main function will create a new service and call
// service.Start(ctx) and service.Shutdown(ctx)
// on SIGINT and SIGTERM
type Service interface {
	Start(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

type service struct {
	l      *zap.Logger
	client opamp.Client

	managerConfigPath   string
	collectorConfigPath string
}

func New(wrappedCollector *signozcol.WrappedCollector, logger *zap.Logger, managerConfigPath string, collectorConfigPath string) (*service, error) {

	var client opamp.Client
	var err error

	// Running without Opamp
	if managerConfigPath == "" {
		client = opamp.NewSimpleClient(wrappedCollector)
	} else {
		managerConfig, err := opamp.ParseAgentManagerConfig(managerConfigPath)
		// Invalid config file
		if err != nil {
			return nil, fmt.Errorf("failed to parse manager config: %w", err)
		}
		serverClientOpts := &opamp.NewServerClientOpts{
			Logger:             logger,
			Config:             managerConfig,
			WrappedCollector:   wrappedCollector,
			CollectorConfgPath: collectorConfigPath,
		}
		client, err = opamp.NewServerClient(serverClientOpts)
	}

	return &service{
		client:              client,
		l:                   logger,
		managerConfigPath:   managerConfigPath,
		collectorConfigPath: collectorConfigPath,
	}, err
}

// Start starts the Opamp connection and collector
func (s *service) Start(ctx context.Context) error {
	if err := s.client.Start(ctx); err != nil {
		return fmt.Errorf("failed to start : %w", err)
	}
	return nil
}

// Shutdown stops the Opamp connection and collector
func (s *service) Shutdown(ctx context.Context) error {
	if err := s.client.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop: %w", err)
	}
	return nil
}
