package storage

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sndv-kv/internal/common"
	"sync"
)

type DiskWAL struct {
	file       *os.File
	mutex      sync.Mutex
	path       string
	shouldSync bool
}

func NewDiskWAL(path string, shouldSync bool) (*DiskWAL, error) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL: %w", err)
	}
	return &DiskWAL{
		file:       file,
		path:       path,
		shouldSync: shouldSync,
	}, nil
}

func (w *DiskWAL) WriteBatch(entries []common.Entry) error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	totalSize := 0
	for _, e := range entries {
		totalSize += 4 + len(e.Key) + 4 + len(e.Value) + 9
	}

	buffer := make([]byte, totalSize)
	offset := 0

	for _, e := range entries {
		kLen := len(e.Key)
		vLen := len(e.Value)

		binary.LittleEndian.PutUint32(buffer[offset:], uint32(kLen))
		offset += 4
		copy(buffer[offset:], e.Key)
		offset += kLen

		binary.LittleEndian.PutUint32(buffer[offset:], uint32(vLen))
		offset += 4
		copy(buffer[offset:], e.Value)
		offset += vLen

		binary.LittleEndian.PutUint64(buffer[offset:], uint64(e.ExpiryTimestamp))
		offset += 8

		if e.IsDeleted {
			buffer[offset] = 1
		} else {
			buffer[offset] = 0
		}
		offset += 1
	}

	if _, err := w.file.Write(buffer); err != nil {
		return err
	}

	if w.shouldSync {
		return w.file.Sync()
	}
	return nil
}

func (w *DiskWAL) Replay(callback func(common.Entry)) error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if _, err := w.file.Seek(0, 0); err != nil {
		return err
	}

	reader := bufio.NewReader(w.file)
	header := make([]byte, 4)

	for {
		if _, err := io.ReadFull(reader, header); err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		kLen := binary.LittleEndian.Uint32(header)
		key := make([]byte, kLen)
		if _, err := io.ReadFull(reader, key); err != nil {
			return err
		}

		if _, err := io.ReadFull(reader, header); err != nil {
			return err
		}
		vLen := binary.LittleEndian.Uint32(header)
		val := make([]byte, vLen)
		if _, err := io.ReadFull(reader, val); err != nil {
			return err
		}

		meta := make([]byte, 9)
		if _, err := io.ReadFull(reader, meta); err != nil {
			return err
		}

		expiry := int64(binary.LittleEndian.Uint64(meta[:8]))
		isDeleted := meta[8] == 1

		callback(common.Entry{
			Key:             string(key),
			Value:           val,
			ExpiryTimestamp: expiry,
			IsDeleted:       isDeleted,
		})
	}

	w.file.Seek(0, 2)
	return nil
}

func (w *DiskWAL) Close() error {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	return w.file.Close()
}

func (w *DiskWAL) Delete() error {
	w.Close()
	return os.Remove(w.path)
}
