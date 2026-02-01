package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"sndv-kv/internal/agents"
	"sndv-kv/internal/config"
	"sndv-kv/internal/core"
	"sndv-kv/internal/logger"
	"testing"
)

func setupApiTest(t *testing.T) (*HttpApiRouter, func()) {
	dir := "./test_api_" + t.Name()
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0755)

	logger.InitializeLogger(dir, "ERROR")

	cfg := config.SystemConfiguration{
		DataDirectoryPath:            dir,
		WriteAheadLogFilePath:        dir + "/wal.log",
		MaximumMemtableSizeInBytes:   1024 * 1024,
		EnableDiskDurability:         false,
		KeyCacheCapacityCount:        1000,
		BloomFilterFalsePositiveRate: 0.01,
	}

	state := core.NewSystemState(cfg)
	agents.InitializeIngestionSubsystem(state)

	return &HttpApiRouter{SystemState: state}, func() { os.RemoveAll(dir) }
}

func TestCriticalPath_PutGet(t *testing.T) {
	router, cleanup := setupApiTest(t)
	defer cleanup()

	// 1. PUT
	putBody := []byte(`{"key":"user:123", "value":"John Doe", "ttl":0}`)
	reqPut := httptest.NewRequest("PUT", "/put", bytes.NewBuffer(putBody))
	wPut := httptest.NewRecorder()

	router.HandleSinglePutRequest(wPut, reqPut)

	if wPut.Code != 201 {
		t.Fatalf("PUT failed. Code: %d, Body: %s", wPut.Code, wPut.Body.String())
	}

	// 2. GET
	reqGet := httptest.NewRequest("GET", "/get?key=user:123", nil)
	wGet := httptest.NewRecorder()

	router.HandleGetRequest(wGet, reqGet)

	if wGet.Code != 200 {
		t.Fatalf("GET failed. Code: %d", wGet.Code)
	}

	var resp map[string]string
	json.NewDecoder(wGet.Body).Decode(&resp)

	if resp["val"] != "John Doe" {
		t.Errorf("Critical Path Failure: Expected 'John Doe', got '%s'", resp["val"])
	}
}

func TestCriticalPath_Delete(t *testing.T) {
	router, cleanup := setupApiTest(t)
	defer cleanup()

	// Setup data
	agents.SubmitIngestionRequest("del_me", []byte("val"), 0, false)

	// DELETE
	reqDel := httptest.NewRequest("DELETE", "/delete?key=del_me", nil)
	wDel := httptest.NewRecorder()
	router.HandleDeleteRequest(wDel, reqDel)

	if wDel.Code != 200 {
		t.Fatalf("DELETE failed code: %d", wDel.Code)
	}

	// Verify GET returns 404
	reqGet := httptest.NewRequest("GET", "/get?key=del_me", nil)
	wGet := httptest.NewRecorder()
	router.HandleGetRequest(wGet, reqGet)

	if wGet.Code != 404 {
		t.Errorf("Expected 404 after delete, got %d", wGet.Code)
	}
}

func TestCriticalPath_Batch(t *testing.T) {
	router, cleanup := setupApiTest(t)
	defer cleanup()

	body := []byte(`{"items": [{"key":"b1", "value":"v1", "ttl":0}, {"key":"b2", "value":"v2", "ttl":0}]}`)
	req := httptest.NewRequest("POST", "/batch", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	router.HandleBatchPutRequest(w, req)

	if w.Code != 201 {
		t.Fatalf("Batch failed: %d", w.Code)
	}

	// Verify one item
	reqGet := httptest.NewRequest("GET", "/get?key=b2", nil)
	wGet := httptest.NewRecorder()
	router.HandleGetRequest(wGet, reqGet)

	if wGet.Code != 200 {
		t.Errorf("Batch write verification failed")
	}
}
