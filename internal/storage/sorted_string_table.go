package storage

import (
	"bufio"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"sndv-kv/internal/common"
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

type SSTableReader struct {
	file   *os.File
	reader *bufio.Reader
	buffer []byte
}

func NewSSTableReader(filename string) (*SSTableReader, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	return &SSTableReader{
		file:   f,
		reader: bufio.NewReader(f),
		buffer: make([]byte, 17),
	}, nil
}

func (r *SSTableReader) Next() (common.Entry, bool) {
	if _, err := io.ReadFull(r.reader, r.buffer); err != nil {
		return common.Entry{}, false
	}

	kLen := binary.LittleEndian.Uint32(r.buffer[0:4])
	vLen := binary.LittleEndian.Uint32(r.buffer[4:8])
	expiry := int64(binary.LittleEndian.Uint64(r.buffer[8:16]))
	isDeleted := r.buffer[16] == 1

	key := make([]byte, kLen)
	io.ReadFull(r.reader, key)
	val := make([]byte, vLen)
	io.ReadFull(r.reader, val)

	return common.Entry{
		Key:             string(key),
		Value:           val,
		ExpiryTimestamp: expiry,
		IsDeleted:       isDeleted,
	}, true
}

func (r *SSTableReader) Close() {
	if r.file != nil {
		r.file.Close()
	}
}

func WriteSortedStringTableToDisk(entries []common.Entry, filename string, level int, bloom common.BloomFilter) (SSTableMetadata, error) {
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
	var minKey, maxKey string
	header := make([]byte, 17)

	for i, e := range entries {
		if i == 0 {
			minKey = e.Key
		}
		if i == len(entries)-1 {
			maxKey = e.Key
		}

		if bloom != nil {
			bloom.Add(fileID, []byte(e.Key))
		}
		index[e.Key] = offset

		kLen := len(e.Key)
		vLen := len(e.Value)

		binary.LittleEndian.PutUint32(header[0:4], uint32(kLen))
		binary.LittleEndian.PutUint32(header[4:8], uint32(vLen))
		binary.LittleEndian.PutUint64(header[8:16], uint64(e.ExpiryTimestamp))

		if e.IsDeleted {
			header[16] = 1
		} else {
			header[16] = 0
		}

		w.Write(header)
		w.WriteString(e.Key)
		w.Write(e.Value)

		offset += int64(17 + kLen + vLen)
	}
	w.Flush()

	return SSTableMetadata{
		Level:    level,
		Filename: filename,
		FileID:   fileID,
		Index:    index,
		MinKey:   minKey,
		MaxKey:   maxKey,
	}, nil
}

func FindInSSTable(meta SSTableMetadata, key string) (common.Entry, bool) {
	offset, ok := meta.Index[key]
	if !ok {
		return common.Entry{}, false
	}

	f, err := os.Open(meta.Filename)
	if err != nil {
		return common.Entry{}, false
	}
	defer f.Close()

	f.Seek(offset, 0)
	header := make([]byte, 17)
	io.ReadFull(f, header)

	kLen := binary.LittleEndian.Uint32(header[0:4])
	vLen := binary.LittleEndian.Uint32(header[4:8])
	expiry := int64(binary.LittleEndian.Uint64(header[8:16]))
	isDeleted := header[16] == 1

	f.Seek(int64(kLen), 1)
	val := make([]byte, vLen)
	io.ReadFull(f, val)

	return common.Entry{
		Key:             key,
		Value:           val,
		ExpiryTimestamp: expiry,
		IsDeleted:       isDeleted,
	}, true
}
