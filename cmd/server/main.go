package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"sndv-kv/internal/agents"
	"sndv-kv/internal/api"
	"sndv-kv/internal/config"
	"sndv-kv/internal/core"
	"sndv-kv/internal/logger"
	"sndv-kv/internal/metrics"
	"sndv-kv/internal/storage"
	"time"

	"github.com/o1egl/paseto"
)

func main() {
	cfgPath := flag.String("config", "", "Config file path")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize Logger
	if err := logger.Init(cfg.LogDir, cfg.LogLevel); err != nil {
		log.Fatalf("Log Init Failed: %v", err)
	}

	// Runtime Config
	if cfg.MaxCPU > 0 {
		runtime.GOMAXPROCS(cfg.MaxCPU)
	}

	os.MkdirAll(cfg.DataDir, 0755)
	bb := core.NewBlackboard(cfg)

	// Recovery
	if cfg.Durability {
		bb.ActiveWal, err = storage.OpenWAL(cfg.WalPath, cfg.Durability)
		if err != nil {
			logger.Error("WAL Open Failed: %v", err)
			os.Exit(1)
		}

		fmt.Println("Restoring WAL...")
		bb.ActiveWal.Replay(func(k string, v []byte, exp int64, del bool) {
			bb.MemTable.Put(k, v, exp, del)
		})
	}

	// Start Subsystems
	metrics.StartSystemMonitor(cfg.DataDir, cfg.WalPath)
	agents.InitIngest(bb) // Sharded Init
	agents.StartFlushAgent(bb)
	agents.StartCompactionAgent(bb)

	// Auth
	if cfg.AuthToken == "" {
		key := []byte(fmt.Sprintf("%-32s", cfg.AuthSecret))[:32]
		token, _ := paseto.NewV2().Encrypt(key, paseto.JSONToken{
			Subject: "admin", Expiration: time.Now().Add(24 * time.Hour),
		}, "")
		fmt.Printf("ADMIN TOKEN: %s\n", token)
	}

	// Router
	r := &api.Router{BB: bb}
	http.HandleFunc("/batch", r.MiddlewareAuth(r.HandleBatchPut))
	http.HandleFunc("/put", r.MiddlewareAuth(r.HandlePut))
	http.HandleFunc("/get", r.MiddlewareAuth(r.HandleGet))

	addr := fmt.Sprintf(":%d", cfg.Port)
	logger.Info("Server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
