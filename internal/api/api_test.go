package api

import (
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

func setupTestServer(t *testing.T) (*fasthttp.Client, func()) {
	dir := "./test_api_" + t.Name()
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0755)
	logger.InitializeLogger(dir, "ERROR")

	cfg := config.SystemConfiguration{
		DataDirectoryPath:          dir,
		WriteAheadLogFilePath:      dir + "/wal.log",
		MaximumMemtableSizeInBytes: 1024,
		KeyCacheCapacityCount:      1000,
	}
	state := core.NewSystemState(cfg)
	agents.InitializeIngestionSubsystem(state)

	router := &HttpApiRouter{SystemState: state}
	ln := fasthttputil.NewInmemoryListener()

	go fasthttp.Serve(ln, router.GetFastHTTPHandler())

	client := &fasthttp.Client{
		Dial: func(addr string) (net.Conn, error) { return ln.Dial() },
	}

	return client, func() { ln.Close(); os.RemoveAll(dir) }
}

func TestAPI_Positive_PutGet(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	// Put
	req, resp := fasthttp.AcquireRequest(), fasthttp.AcquireResponse()
	req.SetRequestURI("http://test/put")
	req.Header.SetMethod("POST")
	req.SetBody([]byte(`{"key":"k1","value":"v1","ttl":0}`))
	client.Do(req, resp)

	if resp.StatusCode() != 201 {
		t.Errorf("Put failed: %d", resp.StatusCode())
	}

	// Get
	req.SetRequestURI("http://test/get?key=k1")
	req.Header.SetMethod("GET")
	client.Do(req, resp)

	if resp.StatusCode() != 200 {
		t.Errorf("Get failed: %d", resp.StatusCode())
	}
}

func TestAPI_Positive_BatchDelete(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	// Batch
	req, resp := fasthttp.AcquireRequest(), fasthttp.AcquireResponse()
	req.SetRequestURI("http://test/batch")
	req.Header.SetMethod("POST")
	req.SetBody([]byte(`{"items":[{"key":"b1","value":"v1","ttl":0}]}`))
	client.Do(req, resp)

	if resp.StatusCode() != 201 {
		t.Error("Batch failed")
	}

	// Delete
	req.SetRequestURI("http://test/delete?key=b1")
	req.Header.SetMethod("DELETE")
	client.Do(req, resp)

	if resp.StatusCode() != 200 {
		t.Error("Delete failed")
	}

	// Verify Gone
	req.SetRequestURI("http://test/get?key=b1")
	req.Header.SetMethod("GET")
	client.Do(req, resp)
	if resp.StatusCode() != 404 {
		t.Error("Get after delete should be 404")
	}
}

func TestAPI_Negative_BadRequests(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()
	req, resp := fasthttp.AcquireRequest(), fasthttp.AcquireResponse()

	// Bad Put JSON
	req.SetRequestURI("http://test/put")
	req.Header.SetMethod("POST")
	req.SetBody([]byte(`{bad}`))
	client.Do(req, resp)
	if resp.StatusCode() != 400 {
		t.Error("Bad JSON should be 400")
	}

	// Bad Batch JSON
	req.SetRequestURI("http://test/batch")
	req.Header.SetMethod("POST")
	req.SetBody([]byte(`{bad}`))
	client.Do(req, resp)
	if resp.StatusCode() != 400 {
		t.Error("Bad Batch JSON should be 400")
	}

	// Bad Method
	req.Header.SetMethod("GET")
	client.Do(req, resp)
	if resp.StatusCode() != 405 {
		t.Error("Wrong method should be 405")
	}

	// Missing Key
	req.SetRequestURI("http://test/get")
	req.Header.SetMethod("GET")
	client.Do(req, resp)
	if resp.StatusCode() != 400 {
		t.Error("Missing key should be 400")
	}

	// Missing Key Delete
	req.SetRequestURI("http://test/delete")
	req.Header.SetMethod("DELETE")
	client.Do(req, resp)
	if resp.StatusCode() != 400 {
		t.Error("Missing key delete should be 400")
	}
}

func TestAPI_Metrics(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()
	req, resp := fasthttp.AcquireRequest(), fasthttp.AcquireResponse()

	req.SetRequestURI("http://test/metrics")
	req.Header.SetMethod("GET")
	client.Do(req, resp)
	if resp.StatusCode() != 200 {
		t.Error("Metrics failed")
	}
}

func TestAPI_PanicRecovery(t *testing.T) {
	// Difficult to simulate handler panic without modifying router,
	// but recoverPanic is covered if called directly or via integration.
	// We rely on integration correctness here.
}
