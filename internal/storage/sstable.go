package storage

import (
	"bufio"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type SSTableMetadata struct {
	Level    int
	Filename string
	FileID   int64
	Index    map[string]int64
	MinKey   string
	MaxKey   string
}

type SSTableIterator struct {
	file    *os.File
	reader  *bufio.Reader
	Current Entry
	Valid   bool
	Error   error
	buf     []byte
}

func NewIterator(filename string) (*SSTableIterator, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	return &SSTableIterator{
		file:   f,
		reader: bufio.NewReader(f),
		Valid:  false,
		buf:    make([]byte, 17),
	}, nil
}

func (it *SSTableIterator) Next() bool {
	if _, err := io.ReadFull(it.reader, it.buf); err != nil {
		if err != io.EOF {
			it.Error = err
		}
		it.Valid = false
		return false
	}

	kLen := binary.LittleEndian.Uint32(it.buf[0:4])
	vLen := binary.LittleEndian.Uint32(it.buf[4:8])
	expiry := int64(binary.LittleEndian.Uint64(it.buf[8:16]))
	deleted := it.buf[16] == 1

	keyBytes := make([]byte, kLen)
	io.ReadFull(it.reader, keyBytes)
	valBytes := make([]byte, vLen)
	io.ReadFull(it.reader, valBytes)

	it.Current = Entry{Key: string(keyBytes), Value: valBytes, ExpiresAt: expiry, Deleted: deleted}
	it.Valid = true
	return true
}

func (it *SSTableIterator) Close() {
	if it.file != nil {
		it.file.Close()
	}
}

func WriteSSTable(entries []Entry, filename string, level int, bloom *SharedBloom) (SSTableMetadata, error) {
	f, err := os.Create(filename)
	if err != nil {
		return SSTableMetadata{}, err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	index := make(map[string]int64)

	var fileID int64 = 0
	base := filepath.Base(filename)
	parts := strings.Split(base, "_")

	if len(parts) >= 2 {
		idPart := strings.TrimSuffix(parts[1], ".sst")
		if val, err := strconv.ParseInt(idPart, 10, 64); err == nil {
			fileID = val
		}
	}

	var offset int64 = 0
	minKey, maxKey := "", ""
	headerBuf := make([]byte, 17)

	for i, entry := range entries {
		if i == 0 {
			minKey = entry.Key
		}
		if i == len(entries)-1 {
			maxKey = entry.Key
		}

		if bloom != nil {
			bloom.Add(fileID, []byte(entry.Key))
		}
		index[entry.Key] = offset

		kLen := len(entry.Key)
		vLen := len(entry.Value)

		binary.LittleEndian.PutUint32(headerBuf[0:4], uint32(kLen))
		binary.LittleEndian.PutUint32(headerBuf[4:8], uint32(vLen))
		binary.LittleEndian.PutUint64(headerBuf[8:16], uint64(entry.ExpiresAt))
		if entry.Deleted {
			headerBuf[16] = 1
		} else {
			headerBuf[16] = 0
		}

		w.Write(headerBuf)
		w.WriteString(entry.Key)
		w.Write(entry.Value)

		offset += int64(17 + kLen + vLen)
	}
	w.Flush()

	return SSTableMetadata{
		Level: level, Filename: filename, FileID: fileID, Index: index, MinKey: minKey, MaxKey: maxKey,
	}, nil
}

func FindInSSTable(meta SSTableMetadata, key string) (Entry, bool) {
	offset, ok := meta.Index[key]
	if !ok {
		return Entry{}, false
	}

	f, err := os.Open(meta.Filename)
	if err != nil {
		return Entry{}, false
	}
	defer f.Close()

	f.Seek(offset, 0)
	header := make([]byte, 17)
	f.Read(header)

	kLen := binary.LittleEndian.Uint32(header[0:4])
	vLen := binary.LittleEndian.Uint32(header[4:8])
	expiry := int64(binary.LittleEndian.Uint64(header[8:16]))
	deleted := header[16] == 1

	f.Seek(int64(kLen), 1)
	valBytes := make([]byte, vLen)
	io.ReadFull(f, valBytes)

	return Entry{Key: key, Value: valBytes, ExpiresAt: expiry, Deleted: deleted}, true
}
