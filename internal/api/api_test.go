package api

import (
	"encoding/json"
	"net"
	"os"
	"sndv-kv/internal/agents"
	"sndv-kv/internal/config"
	"sndv-kv/internal/core"
	"sndv-kv/internal/logger"
	"testing"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

// setupTestServer creates an in-memory fasthttp server/client pair
func setupTestServer(t *testing.T) (*fasthttp.Client, func()) {
	dir := "./test_api_" + t.Name()
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0755)
	
	// Quiet logger
	logger.InitializeLogger(dir, "ERROR")

	cfg := config.SystemConfiguration{
		DataDirectoryPath:          dir,
		WriteAheadLogFilePath:      dir + "/wal.log",
		MaximumMemtableSizeInBytes: 1024 * 1024,
		EnableDiskDurability:       false,
		KeyCacheCapacityCount:      1000,
		BloomFilterFalsePositiveRate: 0.01,
	}

	state := core.NewSystemState(cfg)
	agents.InitializeIngestionSubsystem(state)
	
	router := &HttpApiRouter{SystemState: state}
	handler := router.GetFastHTTPHandler()

	// In-memory listener for testing without ports
	ln := fasthttputil.NewInmemoryListener()
	
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- fasthttp.Serve(ln, handler)
	}()

	client := &fasthttp.Client{
		Dial: func(addr string) (net.Conn, error) {
			return ln.Dial()
		},
	}

	cleanup := func() {
		ln.Close()
		os.RemoveAll(dir)
	}

	return client, cleanup
}

func TestCriticalPath_PutGet(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. PUT
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI("http://test/put")
	req.Header.SetMethod("POST")
	req.SetBody([]byte(`{"key":"user:123", "value":"John Doe", "ttl":0}`))

	if err := client.Do(req, resp); err != nil {
		t.Fatalf("PUT failed: %v", err)
	}
	if resp.StatusCode() != fasthttp.StatusCreated {
		t.Fatalf("PUT status %d != 201. Body: %s", resp.StatusCode(), resp.Body())
	}

	// 2. GET
	req.Reset()
	resp.Reset()
	req.SetRequestURI("http://test/get?key=user:123")
	req.Header.SetMethod("GET")

	if err := client.Do(req, resp); err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	if resp.StatusCode() != fasthttp.StatusOK {
		t.Fatalf("GET status %d != 200", resp.StatusCode())
	}

	var data map[string]string
	json.Unmarshal(resp.Body(), &data)
	if data["val"] != "John Doe" {
		t.Errorf("Expected 'John Doe', got '%s'", data["val"])
	}
}

func TestCriticalPath_Batch(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	// Batch PUT
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI("http://test/batch")
	req.Header.SetMethod("POST")
	req.SetBody([]byte(`{"items": [{"key":"b1", "value":"v1", "ttl":0}, {"key":"b2", "value":"v2", "ttl":0}]}`))

	if err := client.Do(req, resp); err != nil {
		t.Fatalf("Batch failed: %v", err)
	}
	if resp.StatusCode() != fasthttp.StatusCreated {
		t.Errorf("Batch status %d != 201. Body: %s", resp.StatusCode(), resp.Body())
	}

	// Verify Item 2
	req.Reset()
	resp.Reset()
	req.SetRequestURI("http://test/get?key=b2")
	
	if err := client.Do(req, resp); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Errorf("GET b2 failed status %d", resp.StatusCode())
	}
}

func TestCriticalPath_Delete(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	// Seed Data
	agents.SubmitIngestionRequest("del_key", []byte("val"), 0, false)

	// DELETE
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI("http://test/delete?key=del_key")
	req.Header.SetMethod("DELETE")

	if err := client.Do(req, resp); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != fasthttp.StatusOK {
		t.Errorf("DELETE failed status %d", resp.StatusCode())
	}

	// Verify Gone
	req.Reset()
	resp.Reset()
	req.SetRequestURI("http://test/get?key=del_key")
	
	client.Do(req, resp)
	if resp.StatusCode() != fasthttp.StatusNotFound {
		t.Errorf("Expected 404, got %d", resp.StatusCode())
	}
}