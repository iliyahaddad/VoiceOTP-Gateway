package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"voiceotp/gateway/internal/logging"
)

const (
	defaultConfigPath = "/etc/voiceotp/config.json"
	envConfigPath     = "VOICEOTP_CONFIG"
)

func main() {
	cfgPath := defaultConfigPath
	if v := os.Getenv(envConfigPath); v != "" {
		cfgPath = v
	}

	logger := logging.Stdout(logging.LevelInfo)

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		logger.Error(map[string]interface{}{"event": "startup_failed", "reason": err.Error()})
		os.Exit(1)
	}
	if err := cfg.Validate(); err != nil {
		logger.Error(map[string]interface{}{"event": "config_invalid", "reason": err.Error()})
		os.Exit(1)
	}
	logger.Info(map[string]interface{}{"event": "startup", "listen": cfg.Server.Listen, "tls": cfg.Server.TLSEnabled})

	srv, err := NewServer(cfg, logger)
	if err != nil {
		logger.Error(map[string]interface{}{"event": "server_init_failed", "reason": err.Error()})
		os.Exit(1)
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			logger.Error(map[string]interface{}{"event": "server_stopped", "reason": err.Error()})
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	logger.Info(map[string]interface{}{"event": "shutdown"})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
