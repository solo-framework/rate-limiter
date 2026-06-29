package app

import (
	"context"
	"ratelimiter/internal/logger"
)

type app struct {
	di   *diContainer
	mode string
}
type dependencyFunc func(context.Context) error

func New(ctx context.Context, mode string) (*app, error) {
	app := &app{
		mode: mode,
	}

	err := app.initDeps(ctx)
	if err != nil {
		return nil, err
	}

	return app, nil
}

func (s *app) Run() error {
	return nil
}

func (s *app) initDeps(ctx context.Context) error {
	//
	inits := []dependencyFunc{
		s.initDI,
		s.initLogger,
		s.initCloser,
		// s.initListener,
		// s.initGRPCServer,
		// s.applyMigrations,
	}

	for _, f := range inits {
		err := f(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *app) initDI(_ context.Context) error {
	s.di = NewDIContainer()
	return nil
}

func (s *app) initLogger(_ context.Context) error {
	logger.InitLogger(s.mode)
	return nil
}

func (s *app) initCloser(_ context.Context) error {
	SetLogger(logger.GetLogger())
	return nil
}
