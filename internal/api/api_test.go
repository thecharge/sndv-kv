package api

import (
	"sndv-kv/internal/agents"
	"sndv-kv/internal/config"
	"sndv-kv/internal/core"
	"testing"

	"github.com/valyala/fasthttp"
)

func setupRouter() *HttpApiRouter {
	cfg := config.SystemConfiguration{
		MaximumMemtableSizeInBytes: 1024,
		KeyCacheCapacityCount:      100,
	}
	state := core.NewSystemState(cfg)
	agents.InitializeIngestionSubsystem(state)
	return &HttpApiRouter{SystemState: state}
}

func TestHandleSinglePut(t *testing.T) {
	router := setupRouter()
	ctx := &fasthttp.RequestCtx{}

	// Valid Put
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetBody([]byte(`{"key":"k1", "value":"v1", "ttl":0}`))
	router.HandleSinglePutRequest(ctx)
	if ctx.Response.StatusCode() != fasthttp.StatusCreated {
		t.Errorf("Expected 201, got %d", ctx.Response.StatusCode())
	}

	// Invalid Method
	ctx.Request.Header.SetMethod("GET")
	router.HandleSinglePutRequest(ctx)
	if ctx.Response.StatusCode() != fasthttp.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", ctx.Response.StatusCode())
	}
}

func TestHandleGet(t *testing.T) {
	router := setupRouter()
	agents.SubmitIngestionRequest("k1", []byte("v1"), 0, false)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")
	ctx.QueryArgs().Set("key", "k1")

	router.HandleGetRequest(ctx)
	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Errorf("Expected 200, got %d", ctx.Response.StatusCode())
	}

	// Missing Key
	ctx.QueryArgs().Del("key")
	router.HandleGetRequest(ctx)
	if ctx.Response.StatusCode() != fasthttp.StatusBadRequest {
		t.Errorf("Expected 400 for missing key")
	}
}

func TestHandleDelete(t *testing.T) {
	router := setupRouter()
	ctx := &fasthttp.RequestCtx{}

	ctx.Request.Header.SetMethod("DELETE")
	ctx.QueryArgs().Set("key", "del_me")

	router.HandleDeleteRequest(ctx)
	if ctx.Response.StatusCode() != fasthttp.StatusOK {
		t.Errorf("Expected 200, got %d", ctx.Response.StatusCode())
	}
}

func TestHandleBatch(t *testing.T) {
	router := setupRouter()
	ctx := &fasthttp.RequestCtx{}

	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetBody([]byte(`{"items":[{"key":"b1","value":"v1","ttl":0}]}`))

	router.HandleBatchPutRequest(ctx)
	if ctx.Response.StatusCode() != fasthttp.StatusCreated {
		t.Errorf("Expected 201, got %d", ctx.Response.StatusCode())
	}
}

func TestAuth(t *testing.T) {
	router := setupRouter()
	router.SystemState.Configuration.AuthenticationToken = "dummy" // Enable auth

	ctx := &fasthttp.RequestCtx{}
	// No header
	if router.checkAuth(ctx) {
		t.Error("Auth should fail without header")
	}
}
