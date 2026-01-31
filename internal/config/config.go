package config

import (
	"encoding/json"
	"os"
)

const ConfigTemplate = `{
  "data_dir": "./data",
  "wal_path": "./data/wal.log",
  "log_dir": "./logs",
  "port": 8080,
  "max_memtable_size": 67108864,
  "l0_compaction_trigger": 4,
  "sstable_block_size": 4096,
  "bloom_fpr": 0.01,
  "compaction_interval": 5,
  "auth_secret": "CHANGE_ME",
  "durability": true,
  "max_cpu": 0,
  "max_system_memory": 0,
  "enable_profiling": false,
  "key_cache_size": 40000,
  "log_level": "INFO"
}`

const (
	DefaultPort           = 8080
	DefaultMemtableSize   = 64 * 1024 * 1024
	DefaultCacheSize      = 40000
	DefaultCompactionTime = 5
	MinBloomFPR           = 0.01
)

type Config struct {
	DataDir             string  `json:"data_dir"`
	WalPath             string  `json:"wal_path"`
	LogDir              string  `json:"log_dir"`
	Port                int     `json:"port"`
	MaxMemTableSize     int64   `json:"max_memtable_size"`
	L0CompactionTrigger int     `json:"l0_compaction_trigger"`
	SSTableBlockSize    int     `json:"sstable_block_size"`
	BloomFPR            float64 `json:"bloom_fpr"`
	CompactionInterval  int     `json:"compaction_interval"`
	AuthToken           string  `json:"auth_token"`
	AuthSecret          string  `json:"auth_secret"`
	Durability          bool    `json:"durability"`
	MaxCPU              int     `json:"max_cpu"`
	MaxSystemMemory     int64   `json:"max_system_memory"`
	EnableProfiling     bool    `json:"enable_profiling"`
	LogLevel            string  `json:"log_level"`
	KeyCacheSize        int     `json:"key_cache_size"`
}

func Load(path string) (Config, error) {
	cfg := Config{
		DataDir:             "./data",
		WalPath:             "./data/wal.log",
		LogDir:              "./logs",
		Port:                DefaultPort,
		MaxMemTableSize:     DefaultMemtableSize,
		L0CompactionTrigger: 4,
		SSTableBlockSize:    4096,
		BloomFPR:            MinBloomFPR,
		CompactionInterval:  DefaultCompactionTime,
		AuthSecret:          "DEFAULT_SECRET_CHANGE_ME_IN_PROD",
		Durability:          true,
		MaxCPU:              0,
		MaxSystemMemory:     0,
		EnableProfiling:     false,
		LogLevel:            "INFO",
		KeyCacheSize:        DefaultCacheSize,
	}

	if path != "" {
		f, err := os.Open(path)
		if err != nil {
			return cfg, err
		}
		defer f.Close()
		if err := json.NewDecoder(f).Decode(&cfg); err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}
