package logger

import "go.uber.org/zap"

// Log глобальный экземпляр логгера.
var Log = zap.NewNop().Sugar()

// Initialize инициализирует логгер с указанным уровнем логирования.
func Initialize(level string) error {
	lvl, err := zap.ParseAtomicLevel(level)
	if err != nil {
		return err
	}

	cfg := zap.NewDevelopmentConfig()
	cfg.Level = lvl

	zl, err := cfg.Build()
	if err != nil {
		return err
	}

	Log = zl.Sugar()

	return nil
}
