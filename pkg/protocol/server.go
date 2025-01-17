package protocol

import (
	"backend/pkg/config"
	"backend/pkg/identity/auth"
	"backend/pkg/identity/user"
	"backend/pkg/infra/api/routing"
	"backend/pkg/infra/storage/db"
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

type Server struct {
	Dependencies    *Dependencies
	Router          *routing.Router
	log             *zap.Logger
	ShutdownTimeout time.Duration
}

type Dependencies struct {
	Postgres db.DB
	Cfg      *config.Config

	UserSvc user.Service
	AuthSvc auth.Service
}

func NewServer(deps *Dependencies, cfg *config.Config) *Server {
	r := routing.NewRouter()
	return &Server{
		Dependencies:    deps,
		Router:          r,
		log:             zap.L().Named("server"),
		ShutdownTimeout: time.Duration(30) * time.Second,
	}
}

func (s *Server) registerRoutes() {
	r := s.Router

	s.NewUserHandler(r)
	s.NewAuthHandler(r)
}

func (s *Server) Run(ctx context.Context) error {
	stopCh := ctx.Done()

	s.registerRoutes()

	addr := fmt.Sprintf(":%s", s.Dependencies.Cfg.Server.HTTPPort)

	s.log.Info("Starting HTTP server", zap.String("port", s.Dependencies.Cfg.Server.HTTPPort))
	err := s.Router.ListenToAddress(addr)
	if err != nil {
		return err
	}

	go func() {
		<-stopCh
		fmt.Println("Shutting down server...")
		s.Router.Shutdown(ctx)
	}()

	return nil
}
