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

	// panic("FOOOOOOOOOOOO")

	return app, nil
	// return app, fmt.Errorf("FOOOOOOOOOOOO")
}

func (s *app) Run(ctx context.Context) error {

	// return fmt.Errorf("FOOOOOOOOOOOO")
	<-ctx.Done()
	return nil
}

func (s *app) initDeps(ctx context.Context) error {
	//
	inits := []dependencyFunc{
		s.initDI,
		// s.initLogger,
		// s.initCloser,
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
	logger.GetLogger().Info("DI initialized")
	return nil
}

// func (s *app) initLogger(_ context.Context) error {
// 	logger.InitLogger(s.mode)
// 	logger.GetLogger().Info("logger initialized")
// 	return nil
// }

// func (s *app) initCloser(_ context.Context) error {

// 	closer.SetLogger(logger.GetLogger())
// 	logger.GetLogger().Info("Closer initialized")
// 	return nil
// }
