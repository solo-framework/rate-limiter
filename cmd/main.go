package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os/signal"
	"syscall"
	"time"

	// _ "net/http/pprof"
	"os"
	// "os/signal"
	// "ratelimiter/control"
	"ratelimiter/internal"
	"ratelimiter/internal/app"
	"ratelimiter/internal/closer"
	"ratelimiter/internal/config"
	"ratelimiter/internal/logger"
	// "go.uber.org/zap"
	// "ratelimiter/service"
	// "syscall"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"

	exitCode = 0
)

func main() {

	env, ok := os.LookupEnv("RATE_LIMITER_ENV")
	if !ok {
		env = "prod"
	}

	v := flag.Bool("version", false, "app version")
	configPath := flag.String("config", "", "path to config file")
	inf := flag.Bool("info", false, "print info")
	flag.Parse()

	if *v {
		fmt.Printf("ratelimiter version %s, date %s, commit %s\n", version, date, commit)
		os.Exit(0)
	}

	if *inf {
		internal.DisplayBanner()
		os.Exit(0)
	}

	if *configPath == "" {
		fmt.Println("config file not defined")
		flag.Usage()
		os.Exit(2)
	}

	err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("config load failed: %v", err)
		os.Exit(1)
	}

	stopSignals := []os.Signal{syscall.SIGINT, syscall.SIGTERM}
	appCtx, stopFn := signal.NotifyContext(context.Background(), stopSignals...)

	logger.InitLogger(env)
	closer.ConfigureWithContext(appCtx, logger.GetLogger())

	defer func() {

		if r := recover(); r != nil {
			logger.GetLogger().Error("PANIC", slog.Any("error", r))
			os.Exit(1)
		}

		if exitCode > 0 {
			os.Exit(exitCode)
		}

		logger.GetLogger().Info("exit ok")
	}()

	defer stopFn()
	defer gracefulShutdown()

	application, err := app.New(appCtx, env)
	if err != nil {
		logger.GetLogger().Error("❌ Не удалось создать приложение", slog.Any("error", err))
		exitCode = 3
		return
	}

	err = application.Run(appCtx)
	if err != nil {
		logger.GetLogger().Error("❌ Не удалось запустить приложение", slog.Any("error", err))
		exitCode = 4
		return
	}

	// logger.GetLogger().Info("finish")

	// logger
	// logger.InitLogger(env)
	// logger := logger.GetLogger()
	// logger.Debug("config", "config", config.AppConfig())

	// // define stop signals
	// stopCh := make(chan os.Signal, 1)
	// signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)

	// // add default group
	// groups := config.AppConfig().Groups
	// // time provider
	// timeProvider := service.NewTimeProvider() // *

	// // create limit manager
	// manager, err := service.NewManager(groups, config.AppConfig().TTL, config.AppConfig().CleanupInterval, timeProvider) //
	// if err != nil {
	// 	logger.Error("can't create limit manager: ", "error", err)
	// 	os.Exit(1)
	// }

	// logger.Info("starting services...")

	// // load saved groups
	// ok, err = manager.LoadGroupsFromFile(config.AppConfig().StorePath)
	// if err != nil {
	// 	logger.Error("can't load groups from file", "error", err)
	// 	os.Exit(1)
	// }
	// if ok {
	// 	logger.Info("stored groups loaded")
	// }

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

	// logger.Info(fmt.Sprintf("control server started at %d", config.AppConfig().HttpPort))

	// // start limit cleanup service
	// go manager.StartCleanup()

	// // request processing
	// requestHandler := service.NewRequestHandler(manager, logger)

	// logger.Info(fmt.Sprintf("config: ratelimit port %d", config.AppConfig().Port))
	// logger.Info(fmt.Sprintf("config: ratelimit groups count %d", config.AppConfig().Groups.Count()))

	// // start rate limiter server
	// srvLogger := logger.With("app", "rate")
	// server := service.NewServer(srvLogger, config.AppConfig(), requestHandler)
	// err = server.Start()
	// if err != nil {
	// 	logger.Error("ratelimit service start failed: ", "error", err)
	// 	os.Exit(1)
	// }

	// logger.Info(fmt.Sprintf("ratelimit server started at %d", config.AppConfig().Port))
	// logger.Info("all services are started, waiting signals...")

	// // waiting a signal
	// code := <-stopCh
	// logger.Info(fmt.Sprintf("recieved signal '%s', starting shutdown...", code.String()))

	// // stop cleanup service
	// manager.StopCleanup()

	// // save groups settings into file
	// err = manager.SaveGoupsToFile(config.AppConfig().StorePath)
	// if err != nil {
	// 	logger.Error("manager: can't save groups to file", "error", err)
	// } else {
	// 	logger.Info(fmt.Sprintf("manager: groups saved into dir '%s'", config.AppConfig().StorePath))
	// }

	// // shutdown server
	// if err = server.Shutdown(); err != nil {
	// 	if errors.Is(err, service.ErrShutdown) {
	// 		logger.Warn("all connections are closed by timeout")
	// 	}
	// } else {
	// 	logger.Info("all connections are closed gracefully")
	// }

	// logger.Info("ratelimit service stopped")

	// // stop control server
	// err = controlServer.Shutdown()
	// if err != nil {
	// 	logger.Error("control server stopping error", "error", err)
	// }
	// logger.Info("control server stopped")
	// logger.Info("bye-bye!")

	// os.Exit(0)
}

func gracefulShutdown() {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := closer.CloseAll(ctx); err != nil {
		logger.GetLogger().Error("❌ Ошибка при завершении работы", slog.Any("error", err))
		return
	}

	logger.GetLogger().Info("graceful shutdown завершен")
}
