package app

import (
	"context"
	"fmt"
	"ratelimiter/control"
	"ratelimiter/internal/closer"
	"ratelimiter/internal/config"
	"ratelimiter/internal/logger"
	"ratelimiter/service"
)

type diContainer struct {
	// ...

	timeProvider service.ITimeProvider

	serviceManager *service.Manager

	controlMethods *control.ControlMethods

	requestHandler service.IRequestHandler
}

func NewDIContainer() *diContainer {
	return &diContainer{}
}

func (s *diContainer) RequestHandler() service.IRequestHandler {
	if s.requestHandler == nil {
		handler := service.NewRequestHandler(
			s.ServiceManager(),
			logger.GetLogger(),
		)
		s.requestHandler = handler
	}

	return s.requestHandler
}

func (s *diContainer) ControlMethods() control.ControlMethods {

	// panic("not implr ControlMethods()")
	if s.controlMethods == nil {
		s.controlMethods = control.NewControlMethods(s.ServiceManager())
	}

	return *s.controlMethods
}

func (s *diContainer) GetTimeProvider() service.ITimeProvider {

	if s.timeProvider == nil {
		s.timeProvider = service.NewTimeProvider()
	}
	return s.timeProvider
}

func (s *diContainer) ServiceManager() *service.Manager {
	//
	if s.serviceManager == nil {

		manager, err := service.NewManager(
			config.AppConfig().Groups,
			config.AppConfig().TTL,
			config.AppConfig().CleanupInterval,
			s.GetTimeProvider(),
		)

		if err != nil {
			panic(fmt.Sprintf("failed to create service manager: %s", err))
		}

		closer.AddNamed("cleanup service manager", func(ctx context.Context) error {
			s.serviceManager.CleanUp()
			return nil
		})

		s.serviceManager = manager
	}

	return s.serviceManager
}
