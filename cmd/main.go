package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	// _ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	// "os/signal"
	// "ratelimiter/control"
	"ratelimiter/internal"
	"ratelimiter/internal/app"
	"ratelimiter/internal/closer"
	"ratelimiter/internal/config"
	"ratelimiter/internal/logger"
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
		fmt.Printf("ratelimiter version %s, date %s, commit %s\n", version, date, commit) // nolint
		os.Exit(0)
	}

	if *inf {
		internal.DisplayBanner()
		os.Exit(0)
	}

	if *configPath == "" {
		fmt.Println("config file not defined") // nolint
		flag.Usage()
		os.Exit(2)
	}

	err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("config load failed: %v", err) // nolint
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
