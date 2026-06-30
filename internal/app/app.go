package app

import (
	"context"
	"fmt"
	"log/slog"
	"ratelimiter/control"
	"ratelimiter/internal/closer"
	"ratelimiter/internal/config"
	"ratelimiter/internal/logger"
	"ratelimiter/service"
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

func (s *app) Run(ctx context.Context) error {

	// httpLogger := logger.With("app", "http")
	// // start control server
	// methods := control.NewControlMethods(manager)
	// router := control.NewRouter(*methods, *config.AppConfig(), httpLogger)
	// controlServer := control.NewControlServer(*config.AppConfig(), httpLogger, router)

	// go func() {
	// 	err := controlServer.Start()
	// 	if err != nil {
	// 		logger.Error("can't start control server", "error", err)
	// 		os.Exit(1)
	// 	}
	// }()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 3)

	// 1. start ControlServer
	go func() {
		httpLogger := logger.GetLogger().With("app", "http")
		httpLogger.Info(fmt.Sprintf("Starting control server at port %v", config.AppConfig().HttpPort))

		methods := control.NewControlMethods(s.di.ServiceManager())
		router := control.NewRouter(*methods, *config.AppConfig(), httpLogger)
		controlServer := control.NewControlServer(*config.AppConfig(), httpLogger, router)

		closer.AddNamed("control server", func(ctx context.Context) error {
			err := controlServer.Shutdown()
			if err != nil {
				return fmt.Errorf("control server stopping error: %w", err)
			}
			return nil
		})

		if err := controlServer.Start(); err != nil {
			errCh <- fmt.Errorf("failed to start control server: %w", err)
		}
	}()

	// 2. start manager.StartCleanup()
	go func() {

		defer func() {
			if r := recover(); r != nil {
				errCh <- fmt.Errorf("ServiceManager StartCleanup failed: %s", r)
			}
		}()

		logger.GetLogger().Info("Starting cleanup manager...")
		manager := s.di.ServiceManager()

		closer.AddNamed("service manager", func(ctx context.Context) error {
			manager.StopCleanup()
			return nil
		})

		manager.StartCleanup()
	}()

	// 3. start limiter service

	go func() {

		defer func() {
			if r := recover(); r != nil {
				errCh <- fmt.Errorf("panic in limiter service: %s", r)
			}
		}()

		serviceLogger := logger.GetLogger().With("app", "rate")
		serviceLogger.Info(fmt.Sprintf("starting limiter service at port %v", config.AppConfig().Port))

		manager := s.di.ServiceManager()
		requestHandler := service.NewRequestHandler(manager, serviceLogger)

		server := service.NewServer(serviceLogger, config.AppConfig(), requestHandler)
		err := server.Start()
		if err != nil {
			errCh <- fmt.Errorf("failed to start ratelimit server: %w", err)
		}

		closer.AddNamed("rate service", func(ctx context.Context) error {
			err := server.Shutdown()
			return err
		})
	}()

	select {
	case <-ctx.Done():
		logger.GetLogger().Info("app recieved a shutdown signal")
	case e := <-errCh:
		logger.GetLogger().Error("app failed, shutting down", slog.Any("error", e))

		cancel()
		return fmt.Errorf("app failed, shutting down with: %w", e)
	}

	return nil
}

func (s *app) initDeps(ctx context.Context) error {
	//
	inits := []dependencyFunc{
		s.initDI,
		s.loadSavedGroups,
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

func (s *app) loadSavedGroups(_ context.Context) error {

	manager := s.di.ServiceManager()
	count, err := manager.LoadGroupsFromFile(config.AppConfig().StorePath)
	if err != nil {
		return fmt.Errorf("failed to load stored groups from file: %w", err)
	}
	logger.GetLogger().Info(fmt.Sprintf("ServiceManager: stored groups loaded [%d]", count))
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
