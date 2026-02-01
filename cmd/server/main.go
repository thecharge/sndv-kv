package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"sndv-kv/internal/agents"
	"sndv-kv/internal/api"
	"sndv-kv/internal/common"
	"sndv-kv/internal/config"
	"sndv-kv/internal/core"
	"sndv-kv/internal/logger"
	"sndv-kv/internal/metrics"
	"sndv-kv/internal/storage"
	"time"

	"github.com/o1egl/paseto"
	"github.com/valyala/fasthttp"
)

func main() {
	cfgPath := flag.String("config", "", "Config path")
	flag.Parse()

	cfg, err := config.LoadConfigurationFromFile(*cfgPath)
	if err != nil {
		log.Fatalf("Config Error: %v", err)
	}

	if err := logger.InitializeLogger(cfg.LogDirectoryPath, cfg.LogSeverityLevel); err != nil {
		log.Fatal(err)
	}

	if cfg.MaximumCpuCount > 0 {
		runtime.GOMAXPROCS(cfg.MaximumCpuCount)
	}
	os.MkdirAll(cfg.DataDirectoryPath, 0755)

	system := core.NewSystemState(cfg)

	// Recovery
	if cfg.EnableDiskDurability {
		wal, err := storage.NewDiskWAL(cfg.WriteAheadLogFilePath, true)
		if err != nil {
			logger.LogErrorEvent("WAL Error: %v", err)
			os.Exit(1)
		}
		system.ActiveWal = wal
		system.ActiveWal.Replay(func(e common.Entry) {
			system.MemTable.Put(e.Key, e.Value, e.ExpiryTimestamp, e.IsDeleted)
		})
	}

	metrics.Global = metrics.SystemMetricsRegistry{}
	agents.InitializeIngestionSubsystem(system)
	agents.StartFlushAgentInBackground(system)
	agents.StartCompactionAgentInBackground(system)

	if cfg.AuthenticationToken == "" {
		key := []byte(fmt.Sprintf("%-32s", cfg.AuthenticationSecret))[:32]
		token, _ := paseto.NewV2().Encrypt(key, paseto.JSONToken{
			Subject: "admin", Expiration: time.Now().Add(24 * time.Hour),
		}, "")
		fmt.Printf("ADMIN TOKEN: %s\n", token)
	}

	router := &api.HttpApiRouter{SystemState: system}
	handler := router.GetFastHTTPHandler()

	addr := fmt.Sprintf(":%d", cfg.ServerPort)
	logger.LogInfoEvent("Listening on %s (fasthttp)", addr)

	if err := fasthttp.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}
