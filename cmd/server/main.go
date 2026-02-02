package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
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

	if err := Run(*cfgPath); err != nil {
		log.Fatal(err)
	}
}

func Run(configPath string) error {
	cfg, err := config.LoadConfigurationFromFile(configPath)
	if err != nil {
		return err
	}

	// Start pprof server
	// @TODO: need to test first
	if cfg.EnablePprofProfiling {
		go func() {
			http.ListenAndServe(":6060", nil)
		}()
	}

	if err := logger.InitializeLogger(cfg.LogDirectoryPath, cfg.LogSeverityLevel); err != nil {
		return err
	}

	configureRuntime(cfg)
	os.MkdirAll(cfg.DataDirectoryPath, 0755)

	system := core.NewSystemState(cfg)

	if err := recoverWal(system); err != nil {
		return err
	}

	startAgents(system)
	printAdminToken(cfg)

	return startHttpServer(system, cfg.ServerPort)
}

func configureRuntime(cfg config.SystemConfiguration) {
	if cfg.MaximumCpuCount > 0 {
		runtime.GOMAXPROCS(cfg.MaximumCpuCount)
	}
	// High Throughput Tuning: Less frequent GC
	debug.SetGCPercent(200)
}

func recoverWal(system *core.SystemState) error {
	if !system.Configuration.EnableDiskDurability {
		return nil
	}

	wal, err := storage.NewDiskWAL(system.Configuration.WriteAheadLogFilePath, true)
	if err != nil {
		return err
	}
	system.ActiveWal = wal

	return system.ActiveWal.Replay(func(e common.Entry) {
		system.MemTable.Put(e.Key, e.Value, e.ExpiryTimestamp, e.IsDeleted)
	})
}

func startAgents(system *core.SystemState) {
	metrics.Global = metrics.SystemMetricsRegistry{}
	agents.InitializeIngestionSubsystem(system)
	agents.StartFlushAgentInBackground(system)
	agents.StartCompactionAgentInBackground(system)
}

func printAdminToken(cfg config.SystemConfiguration) {
	if cfg.AuthenticationToken == "" {
		key := []byte(fmt.Sprintf("%-32s", cfg.AuthenticationSecret))[:32]
		token, _ := paseto.NewV2().Encrypt(key, paseto.JSONToken{
			Subject: "admin", Expiration: time.Now().Add(24 * time.Hour),
		}, "")
		fmt.Printf("ADMIN TOKEN: %s\n", token)
	}
}

func startHttpServer(system *core.SystemState, port int) error {
	router := &api.HttpApiRouter{SystemState: system}
	handler := router.GetFastHTTPHandler()

	addr := fmt.Sprintf(":%d", port)
	logger.LogInfoEvent("Listening on %s (fasthttp)", addr)

	return fasthttp.ListenAndServe(addr, handler)
}
