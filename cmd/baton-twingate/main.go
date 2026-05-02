package main

import (
	"context"
	"fmt"
	"os"

	"github.com/conductorone/baton-sdk/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/connectorrunner"
	"github.com/conductorone/baton-sdk/pkg/field"
	"github.com/conductorone/baton-sdk/pkg/types"
	cfg "github.com/conductorone/baton-twingate/pkg/config"
	"github.com/conductorone/baton-twingate/pkg/connector"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

var version = "dev"

func main() {
	ctx := context.Background()

	_, cmd, err := config.DefineConfiguration(
		ctx,
		"baton-twingate",
		getConnector,
		cfg.Configuration,
		connectorrunner.WithDefaultCapabilitiesConnectorBuilder(&connector.Twingate{}),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	cmd.Version = version

	err = cmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func getConnector(ctx context.Context, tgc *cfg.Twingate) (types.ConnectorServer, error) {
	l := ctxzap.Extract(ctx)
	err := field.Validate(cfg.Config, tgc)
	if err != nil {
		return nil, err
	}

	domain := tgc.Domain
	if domain == "" {
		return nil, fmt.Errorf("domain field is required")
	}

	apiKey := tgc.ApiKey
	if apiKey == "" {
		return nil, fmt.Errorf("api key field is required")
	}

	connectorConfig := connector.Config{
		Domain:  domain,
		ApiKey:  apiKey,
		BaseURL: tgc.BaseUrl,
	}

	cb, err := connector.New(ctx, connectorConfig)
	if err != nil {
		l.Error("error creating connector", zap.Error(err))
		return nil, err
	}

	conn, err := connectorbuilder.NewConnector(ctx, cb)
	if err != nil {
		l.Error("error creating connector", zap.Error(err))
		return nil, err
	}
	return conn, nil
}
