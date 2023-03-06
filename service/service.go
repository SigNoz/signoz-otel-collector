package service

import (
	"context"

	"github.com/SigNoz/signoz-otel-collector/opamp"
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
}

// Start starts the Opamp connection and collector
func (s *service) Start(ctx context.Context) error {
	return nil
}

// Shutdown stops the Opamp connection and collector
func (s *service) Shutdown(ctx context.Context) error {
	return nil
}
