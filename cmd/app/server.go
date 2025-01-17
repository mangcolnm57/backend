package app

import (
	"backend/pkg/config"
	"backend/pkg/identity/auth/auth"
	"backend/pkg/identity/storage"
	"backend/pkg/identity/user/userimpl"
	"backend/pkg/infra/registry"
	"backend/pkg/infra/storage/postgres"
	"backend/pkg/protocol"
	"context"
	"errors"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

type Server struct {
	postgresDB postgres.DB
	services   []registry.RunFunc
	log        *zap.Logger
}

func NewServer(isStandaloneMode bool) (*Server, error) {
	cfg, err := config.FromEnv()
	if err != nil {
		return nil, err
	}

	postgresDB, err := postgres.New(storage.New(), cfg.Postgres.ConnectionString())
	if err != nil {
		return nil, err
	}

	userSvc := userimpl.NewService(postgresDB, cfg)
	authSvc := auth.NewService(userSvc)

	restServer := protocol.NewServer(&protocol.Dependencies{
		Postgres: postgresDB,
		Cfg:      cfg,

		UserSvc: userSvc,
		AuthSvc: authSvc,
	}, cfg)

	services := registry.NewServiceRegistry(
		restServer.Run,
	)

	if isStandaloneMode {
		services = registry.NewServiceRegistry(
			restServer.Run,
		)
	}

	return &Server{
		postgresDB: postgresDB,
		services:   services.GetServices(),
		log:        zap.L().Named("apiserver"),
	}, nil
}

func (s *Server) Run(ctx context.Context) {
	defer func() {
		s.postgresDB.Close()
	}()

	var wg sync.WaitGroup
	wg.Add(len(s.services))

	for _, svc := range s.services {
		go func(svc registry.RunFunc) error {
			defer wg.Done()
			err := svc(ctx)
			if err != nil && !errors.Is(err, context.Canceled) {
				s.log.Error("stopped server", zap.String("service", serviceName), zap.Error(err))
				return fmt.Errorf("%s run error: %w", serviceName, err)
			}

			return nil
		}(svc)
	}

	wg.Wait()
}
