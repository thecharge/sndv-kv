package storage

import (
	"bufio"
	"encoding/binary"
	"io"
	"os"
	"sync"
)

type WAL struct {
	file *os.File
	mu   sync.Mutex
	sync bool
	path string
}

func OpenWAL(path string, syncWrites bool) (*WAL, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	return &WAL{file: f, sync: syncWrites, path: path}, nil
}

func (w *WAL) AppendBatch(entries []Entry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	totalSize := 0
	for i := range entries {
		totalSize += 4 + len(entries[i].Key) + 4 + len(entries[i].Value) + 9
	}

	buf := make([]byte, totalSize)
	offset := 0

	for i := range entries {
		e := &entries[i]
		kLen := len(e.Key)
		vLen := len(e.Value)

		binary.LittleEndian.PutUint32(buf[offset:], uint32(kLen))
		offset += 4
		copy(buf[offset:], e.Key)
		offset += kLen

		binary.LittleEndian.PutUint32(buf[offset:], uint32(vLen))
		offset += 4
		copy(buf[offset:], e.Value)
		offset += vLen

		binary.LittleEndian.PutUint64(buf[offset:], uint64(e.ExpiresAt))
		offset += 8

		if e.Deleted {
			buf[offset] = 1
		} else {
			buf[offset] = 0
		}
		offset += 1
	}

	if _, err := w.file.Write(buf); err != nil {
		return err
	}

	if w.sync {
		return w.file.Sync()
	}
	return nil
}

func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}

func (w *WAL) Delete() error {
	w.Close()
	return os.Remove(w.path)
}

func (w *WAL) Replay(onEntry func(string, []byte, int64, bool)) error {
	w.mu.Lock()
	defer w.mu.Unlock()

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

		keyBuf := make([]byte, kLen)
		if _, err := io.ReadFull(reader, keyBuf); err != nil {
			return err
		}

		if _, err := io.ReadFull(reader, header); err != nil {
			return err
		}
		vLen := binary.LittleEndian.Uint32(header)

		valBuf := make([]byte, vLen)
		if _, err := io.ReadFull(reader, valBuf); err != nil {
			return err
		}

		metaBuf := make([]byte, 9)
		if _, err := io.ReadFull(reader, metaBuf); err != nil {
			return err
		}

		expiry := int64(binary.LittleEndian.Uint64(metaBuf[:8]))
		deleted := metaBuf[8] == 1

		onEntry(string(keyBuf), valBuf, expiry, deleted)
	}
	w.file.Seek(0, 2)
	return nil
}
