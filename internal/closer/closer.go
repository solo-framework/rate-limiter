package closer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"time"

	"ratelimiter/internal/logger"
)

// shutdownTimeout по умолчанию, можно сделать параметром
const shutdownTimeout = 2 * time.Second

// Closer управляет процессом graceful shutdown приложения
type Closer struct {
	mu     sync.Mutex                    // Защита от гонки при добавлении функций
	once   sync.Once                     // Гарантия однократного вызова CloseAll
	done   chan struct{}                 // Канал для оповещения о завершении
	funcs  []func(context.Context) error // Зарегистрированные функции закрытия
	logger logger.ILogger                // slog.Logger                   // Используемый логгер
}

// var globalCloser = create(logger.DummyLogger())
var globalCloser *Closer = create()

// global functions

// handleSignals обрабатывает системные сигналы и вызывает CloseAll с fresh shutdown context
func (s *Closer) handleSignals(signals ...os.Signal) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, signals...)
	defer signal.Stop(ch)

	select {
	case <-ch:

		s.logger.Info("🛑 Получен системный сигнал, начинаем graceful shutdown...")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()

		if err := s.CloseAll(shutdownCtx); err != nil {
			s.logger.Error("❌ Ошибка при закрытии ресурсов", slog.Any("error", err))
		}

	case <-s.done:
		// CloseAll уже был вызван вручную, просто выходим
		s.logger.Info("CloseAll уже был вызван вручную")
	}
}

// handleContext обрабатыает системные сигналы, полученные из контекста и вызывает CloseAll с fresh shutdown context
func (s *Closer) handleContext(appContext context.Context) {
	select {
	case <-appContext.Done():
		s.logger.Info("🛑 Получен системный сигнал, начинаем graceful shutdown...")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()

		if err := s.CloseAll(shutdownCtx); err != nil {
			s.logger.Error("❌ Ошибка при закрытии ресурсов", slog.Any("error", err))
		}

	case <-s.done:
		// CloseAll уже был вызван вручную, просто выходим
	}
}

// CloseAll вызывает все зарегистрированные функции закрытия.
// Возвращает первую возникшую ошибку, если таковая была.
func (s *Closer) CloseAll(ctx context.Context) error {
	var result error

	s.once.Do(func() {
		defer close(s.done)

		s.mu.Lock()
		funcs := s.funcs
		s.funcs = nil // освободим память
		s.mu.Unlock()

		if len(funcs) == 0 {
			s.logger.Info("ℹ️ Нет функций для закрытия.")
			return
		}

		s.logger.Info("🚦 Начинаем процесс graceful shutdown...")

		errCh := make(chan error, len(funcs))
		var wg sync.WaitGroup

		// Выполняем в обратном порядке добавления
		for i := len(funcs) - 1; i >= 0; i-- {
			f := funcs[i]
			wg.Add(1)
			go func(f func(context.Context) error) {
				defer wg.Done()

				// Защита от паники
				defer func() {
					if r := recover(); r != nil {
						errCh <- errors.New("panic recovered in closer")
						s.logger.Error("⚠️ Panic в функции закрытия", slog.Any("error", r))
					}
				}()

				if err := f(ctx); err != nil {
					errCh <- err
				}
			}(f)
		}

		// Закрываем канал ошибок, когда все функции завершатся
		go func() {
			wg.Wait()
			close(errCh)
		}()

		// Читаем ошибки или отмену контекста
		for {
			select {
			case <-ctx.Done():
				s.logger.Info("⚠️ Контекст отменён во время закрытия", slog.Any("error", ctx.Err()))
				if result == nil {
					result = ctx.Err()
				}
				return
			case err, ok := <-errCh:
				if !ok {
					s.logger.Info("✅ Все ресурсы успешно закрыты")
					return
				}
				s.logger.Error("❌ Ошибка при закрытии", slog.Any("error", err))
				if result == nil {
					result = err
				}
			}
		}
	})

	return result
}

// Add добавляет одну или несколько функций закрытия
func (s *Closer) Add(f ...func(context.Context) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.funcs = append(s.funcs, f...)
}

// AddNamed добавляет функцию закрытия с именем зависимости для логирования
func (s *Closer) AddNamed(name string, f func(context.Context) error) {
	s.Add(func(ctx context.Context) error {
		start := time.Now()
		s.logger.Info(fmt.Sprintf("🧩 Закрываем %s...", name))

		err := f(ctx)

		duration := time.Since(start)
		if err != nil {
			s.logger.Error(fmt.Sprintf("❌ Ошибка при закрытии %s: %v (заняло %s)", name, err, duration))
		} else {
			s.logger.Info(fmt.Sprintf("✅ %s успешно закрыт за %s", name, duration))
		}
		return err
	})
}

// SetLogger устанавливает логгер
func SetLogger(logger logger.ILogger) {
	globalCloser.SetLogger(logger)
}

// func createWithLogger(log *slog.Logger) *Closer {
// 	return &Closer{
// 		done:   make(chan struct{}),
// 		logger: log,
// 	}
// }

func create() *Closer {
	return &Closer{
		done: make(chan struct{}),
	}
}

func (s *Closer) SetLogger(logger logger.ILogger) {
	s.logger = logger
}

func ConfigureWithContext(appContext context.Context, logger logger.ILogger) {
	SetLogger(logger)
	go globalCloser.handleContext(appContext)
}

func ConfigureWithSignals(signals ...os.Signal) {
	go globalCloser.handleSignals(signals...)
}

// AddNamed добавляет функцию закрытия с именем зависимости для логирования в глобальный closer
func AddNamed(name string, f func(context.Context) error) {
	globalCloser.AddNamed(name, f)
}

// Add добавляет функции закрытия в глобальный closer
func Add(f ...func(context.Context) error) {
	globalCloser.Add(f...)
}

func CloseAll(ctx context.Context) error {
	return globalCloser.CloseAll(ctx)
}
