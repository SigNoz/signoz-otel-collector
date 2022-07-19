package main

import (
	"fmt"
	"log"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/service"
)

func main() {

	factories, err := components()
	if err != nil {
		log.Fatalf("failed to build default components: %v", err)
	}

	info := component.BuildInfo{
		Command:     "signoz-otel-collector",
		Description: "SigNoz OTEL Collector",
		Version:     "latest",
	}

	params := service.CollectorSettings{
		Factories: factories,
		BuildInfo: info,
	}

	if err := run(params); err != nil {
		log.Fatal(err)
	}
}

func runInteractive(params service.CollectorSettings) error {
	cmd := service.NewCommand(params)
	err := cmd.Execute()
	if err != nil {
		return fmt.Errorf("application run finished with error: %w", err)
	}

	return nil
}
