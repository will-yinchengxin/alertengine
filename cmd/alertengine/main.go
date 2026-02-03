package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"alertengine/config"
	"alertengine/engine"
	"alertengine/rule"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v2"
)

var (
	configFile  = flag.String("config", "config.yml", "Configuration file path")
	showVersion = flag.Bool("version", false, "Show version information")
)

const (
	version = "1.0.0"
	appName = "AlertEngine"
)

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Printf("%s version %s\n", appName, version)
		os.Exit(0)
	}

	cfg, err := loadConfig(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid config: %v\n", err)
		os.Exit(1)
	}

	logger, err := initLogger(cfg.Log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("starting alert engine",
		zap.String("version", version),
		zap.String("config", *configFile),
	)

	// 创建规则存储
	storage, err := rule.NewStorage(
		cfg.Storage.RuleDir,
		cfg.Storage.RetentionDays,
		cfg.Storage.EnableHistory,
		logger,
	)
	if err != nil {
		logger.Fatal("failed to create storage", zap.Error(err))
	}

	// 创建监控指标
	metrics := engine.NewMetrics()

	// 创建重载器
	reloader := engine.NewReloader(cfg, storage, logger, metrics)

	// 启动指标服务器
	go startMetricsServer(cfg.MetricsPort, logger)

	// 启动健康检查服务器
	go startHealthServer(reloader, logger)

	// 启动清理任务
	if cfg.Storage.EnableHistory {
		go startCleanupTask(storage, logger)
	}

	// 设置信号处理
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("received signal", zap.String("signal", sig.String()))
		cancel()
		reloader.Stop()
	}()

	// 运行重载器
	reloader.Run()
	reloader.Loop()

	logger.Info("alert engine stopped")
}

func loadConfig(path string) (*config.Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return config.DefaultConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := config.DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return cfg, nil
}

func initLogger(cfg config.LogConfig) (*zap.Logger, error) {
	var level zapcore.Level
	switch cfg.Level {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	default:
		level = zapcore.InfoLevel
	}

	zapConfig := zap.Config{
		Level:            zap.NewAtomicLevelAt(level),
		Development:      false,
		Encoding:         cfg.Format,
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{cfg.OutputPath, "stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	return zapConfig.Build()
}

func startMetricsServer(port int, logger *zap.Logger) {
	http.Handle("/metrics", promhttp.Handler())

	addr := fmt.Sprintf(":%d", port)
	logger.Info("starting metrics server", zap.String("addr", addr))

	if err := http.ListenAndServe(addr, nil); err != nil {
		logger.Error("metrics server failed", zap.Error(err))
	}
}

func startHealthServer(reloader *engine.Reloader, logger *zap.Logger) {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		if reloader.GetManagerCount() > 0 {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Ready"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Not Ready"))
		}
	})

	addr := ":8080"
	logger.Info("starting health server", zap.String("addr", addr))

	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Error("health server failed", zap.Error(err))
	}
}

func startCleanupTask(storage *rule.Storage, logger *zap.Logger) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		logger.Info("starting cleanup task")
		if err := storage.CleanupOldVersions(); err != nil {
			logger.Error("cleanup failed", zap.Error(err))
		} else {
			logger.Info("cleanup completed")
		}
	}
}
