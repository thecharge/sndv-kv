package config

import (
	"encoding/json"
	"fmt"
	"os"
)

const ConfigurationTemplate = `{
  "data_directory_path": "./data",
  "write_ahead_log_file_path": "./data/wal.log",
  "log_directory_path": "./logs",
  "server_port": 8080,
  "maximum_memtable_size_in_bytes": 67108864,
  "level_zero_compaction_trigger_count": 4,
  "sstable_block_size_in_bytes": 4096,
  "bloom_filter_false_positive_rate": 0.01,
  "compaction_interval_in_seconds": 5,
  "authentication_secret": "CHANGE_ME",
  "enable_disk_durability": true,
  "maximum_cpu_count": 0,
  "maximum_system_memory_in_bytes": 0,
  "enable_pprof_profiling": false,
  "key_cache_capacity_count": 40000,
  "log_severity_level": "INFO"
}`

const (
	DefaultServerPort                   = 8080
	DefaultMaximumMemtableSizeInBytes   = 64 * 1024 * 1024
	DefaultKeyCacheCapacityCount        = 40000
	DefaultCompactionIntervalInSeconds  = 5
	DefaultBloomFilterFalsePositiveRate = 0.01
)

type SystemConfiguration struct {
	DataDirectoryPath               string  `json:"data_directory_path"`
	WriteAheadLogFilePath           string  `json:"write_ahead_log_file_path"`
	LogDirectoryPath                string  `json:"log_directory_path"`
	ServerPort                      int     `json:"server_port"`
	MaximumMemtableSizeInBytes      int64   `json:"maximum_memtable_size_in_bytes"`
	LevelZeroCompactionTriggerCount int     `json:"level_zero_compaction_trigger_count"`
	SSTableBlockSizeInBytes         int     `json:"sstable_block_size_in_bytes"`
	BloomFilterFalsePositiveRate    float64 `json:"bloom_filter_false_positive_rate"`
	CompactionIntervalInSeconds     int     `json:"compaction_interval_in_seconds"`
	AuthenticationToken             string  `json:"authentication_token"`
	AuthenticationSecret            string  `json:"authentication_secret"`
	EnableDiskDurability            bool    `json:"enable_disk_durability"`
	MaximumCpuCount                 int     `json:"maximum_cpu_count"`
	MaximumSystemMemoryInBytes      int64   `json:"maximum_system_memory_in_bytes"`
	EnablePprofProfiling            bool    `json:"enable_pprof_profiling"`
	LogSeverityLevel                string  `json:"log_severity_level"`
	KeyCacheCapacityCount           int     `json:"key_cache_capacity_count"`
}

func LoadConfigurationFromFile(filePath string) (SystemConfiguration, error) {
	config := SystemConfiguration{
		DataDirectoryPath:               "./data",
		WriteAheadLogFilePath:           "./data/wal.log",
		LogDirectoryPath:                "./logs",
		ServerPort:                      DefaultServerPort,
		MaximumMemtableSizeInBytes:      DefaultMaximumMemtableSizeInBytes,
		LevelZeroCompactionTriggerCount: 4,
		SSTableBlockSizeInBytes:         4096,
		BloomFilterFalsePositiveRate:    DefaultBloomFilterFalsePositiveRate,
		CompactionIntervalInSeconds:     DefaultCompactionIntervalInSeconds,
		AuthenticationSecret:            "DEFAULT_SECRET_CHANGE_ME_IN_PROD",
		EnableDiskDurability:            true,
		MaximumCpuCount:                 0,
		MaximumSystemMemoryInBytes:      0,
		EnablePprofProfiling:            false,
		LogSeverityLevel:                "INFO",
		KeyCacheCapacityCount:           DefaultKeyCacheCapacityCount,
	}

	if filePath != "" {
		file, err := os.Open(filePath)
		if err != nil {
			return config, fmt.Errorf("failed to open configuration file: %w", err)
		}
		defer file.Close()

		if err := json.NewDecoder(file).Decode(&config); err != nil {
			return config, fmt.Errorf("failed to decode configuration json: %w", err)
		}
	}
	return config, nil
}
