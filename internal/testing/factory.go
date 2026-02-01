package testing

import (
	"os"
	"sndv-kv/internal/common"
	"sndv-kv/internal/config"
	"sndv-kv/internal/core"
	"sndv-kv/internal/storage"
	"testing"
)

type TestSystemFactory struct {
	t       *testing.T
	RootDir string
}

func NewTestFactory(t *testing.T) *TestSystemFactory {
	dir := "./test_data_factory_" + t.Name()
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0755)

	return &TestSystemFactory{
		t:       t,
		RootDir: dir,
	}
}

func (f *TestSystemFactory) Cleanup() {
	os.RemoveAll(f.RootDir)
}

func (f *TestSystemFactory) CreateSystem(opts ...func(*config.SystemConfiguration)) *core.SystemState {
	cfg := config.SystemConfiguration{
		DataDirectoryPath:               f.RootDir,
		WriteAheadLogFilePath:           f.RootDir + "/wal.log",
		MaximumMemtableSizeInBytes:      1024 * 1024,
		EnableDiskDurability:            true,
		BloomFilterFalsePositiveRate:    0.01,
		MaximumCpuCount:                 1,
		LevelZeroCompactionTriggerCount: 2,
		CompactionIntervalInSeconds:     1,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	state := core.NewSystemState(cfg)

	// Always init WAL if durability is on, unless disabled by opts
	if cfg.EnableDiskDurability {
		wal, err := storage.NewDiskWAL(cfg.WriteAheadLogFilePath, true)
		if err != nil {
			f.t.Fatalf("Factory failed to create WAL: %v", err)
		}
		state.ActiveWal = wal
	}

	return state
}

func (f *TestSystemFactory) CreateEntry(key, val string) common.Entry {
	return common.Entry{Key: key, Value: []byte(val)}
}
