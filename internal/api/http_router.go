package api

import (
	"encoding/json"
	"fmt"
	"runtime/debug"
	"sndv-kv/internal/agents"
	"sndv-kv/internal/common"
	"sndv-kv/internal/core"
	"sndv-kv/internal/logger"
	"sndv-kv/internal/metrics"
	"sndv-kv/internal/storage"
	"time"

	"github.com/o1egl/paseto"
	"github.com/valyala/fasthttp"
)

type HttpApiRouter struct {
	SystemState *core.SystemState
}

type SinglePutRequestPayload struct {
	Key        string `json:"key"`
	Value      string `json:"value"`
	TimeToLive int    `json:"ttl"`
}

type BatchPutRequestPayload struct {
	Items []struct {
		Key        string `json:"key"`
		Value      string `json:"value"`
		TimeToLive int    `json:"ttl"`
	} `json:"items"`
}

// GetFastHTTPHandler returns the main entry point for fasthttp
func (router *HttpApiRouter) GetFastHTTPHandler() fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		router.handleRequest(ctx)
	}
}

func (router *HttpApiRouter) handleRequest(ctx *fasthttp.RequestCtx) {
	// 1. Logging & Panic Recovery
	startTime := time.Now()
	defer func() {
		if r := recover(); r != nil {
			logger.LogErrorEvent("PANIC: %v\n%s", r, debug.Stack())
			ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		}
		logger.LogAccessEvent("%s %s %s %v", string(ctx.Method()), string(ctx.Path()), ctx.RemoteAddr(), time.Since(startTime))
	}()

	// 2. Auth Middleware
	if !router.checkAuth(ctx) {
		ctx.Error("Unauthorized", fasthttp.StatusUnauthorized)
		return
	}

	// 3. Routing
	path := string(ctx.Path())
	// used after testing the routes with switch/case were with magnitude 2.5 faster than regular early return statements
	// will be evaluated in future
	switch path {
	case "/put":
		if ctx.IsPost() || ctx.IsPut() {
			router.HandleSinglePutRequest(ctx)
		} else {
			ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
		}
	case "/get":
		if ctx.IsGet() {
			router.HandleGetRequest(ctx)
		} else {
			ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
		}
	case "/batch":
		if ctx.IsPost() {
			router.HandleBatchPutRequest(ctx)
		} else {
			ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
		}
	case "/delete":
		if ctx.IsDelete() || ctx.IsPost() { // Allow POST for easier client use
			router.HandleDeleteRequest(ctx)
		} else {
			ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
		}
	case "/metrics":
		if ctx.IsGet() {
			router.HandleMetricsRequest(ctx)
		} else {
			ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
		}
	default:
		ctx.Error("Not Found", fasthttp.StatusNotFound)
	}
}

func (router *HttpApiRouter) checkAuth(ctx *fasthttp.RequestCtx) bool {
	configToken := router.SystemState.Configuration.AuthenticationToken
	headerToken := string(ctx.Request.Header.Peek("Authorization"))

	if configToken == "" && headerToken == "" {
		return true
	}

	var footer string
	var claims paseto.JSONToken
	secretKey := []byte(fmt.Sprintf("%-32s", router.SystemState.Configuration.AuthenticationSecret))[:32]

	return paseto.NewV2().Decrypt(headerToken, secretKey, &claims, &footer) == nil
}

func (router *HttpApiRouter) HandleSinglePutRequest(ctx *fasthttp.RequestCtx) {
	var payload SinglePutRequestPayload
	if err := json.Unmarshal(ctx.PostBody(), &payload); err != nil {
		ctx.Error("Bad Request: Invalid JSON", fasthttp.StatusBadRequest)
		return
	}

	if err := agents.SubmitIngestionRequest(payload.Key, []byte(payload.Value), payload.TimeToLive, false); err != nil {
		logger.LogErrorEvent("PUT Failed: %v", err)
		ctx.Error(err.Error(), fasthttp.StatusInternalServerError)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusCreated)
}

func (router *HttpApiRouter) HandleGetRequest(ctx *fasthttp.RequestCtx) {
	keyBytes := ctx.QueryArgs().Peek("key")
	if len(keyBytes) == 0 {
		ctx.Error("Missing key", fasthttp.StatusBadRequest)
		return
	}
	key := string(keyBytes)

	if tryCache(ctx, router.SystemState, key) {
		return
	}
	if tryMemory(ctx, router.SystemState, key) {
		return
	}
	if tryDisk(ctx, router.SystemState, key) {
		return
	}

	ctx.Error("Not Found", fasthttp.StatusNotFound)
}

func tryCache(ctx *fasthttp.RequestCtx, state *core.SystemState, key string) bool {
	if state.KeyCache != nil {
		if val, hit := state.KeyCache.Retrieve(key); hit {
			updateMetrics()
			writeJSON(ctx, key, val)
			return true
		}
	}
	return false
}

func tryMemory(ctx *fasthttp.RequestCtx, state *core.SystemState, key string) bool {
	state.Mutex.RLock()
	defer state.Mutex.RUnlock()

	if e, ok := state.MemTable.Get(key); ok {
		return sendEntry(ctx, state, e)
	}
	for i := len(state.ImmutableMem) - 1; i >= 0; i-- {
		if e, ok := state.ImmutableMem[i].Get(key); ok {
			return sendEntry(ctx, state, e)
		}
	}
	return false
}

func tryDisk(ctx *fasthttp.RequestCtx, state *core.SystemState, key string) bool {
	state.Mutex.RLock()
	tables := state.SSTables
	bloom := state.BloomFilter
	state.Mutex.RUnlock()

	for _, level := range tables {
		if searchLevel(ctx, state, level, bloom, key) {
			return true
		}
	}
	return false
}

func searchLevel(ctx *fasthttp.RequestCtx, state *core.SystemState, level []storage.SSTableMetadata, bloom common.BloomFilter, key string) bool {
	for i := len(level) - 1; i >= 0; i-- {
		meta := level[i]
		if bloom != nil && !bloom.Contains(meta.FileID, []byte(key)) {
			continue
		}
		if e, found := storage.FindInSSTable(meta, key); found {
			return sendEntry(ctx, state, e)
		}
	}
	return false
}

func sendEntry(ctx *fasthttp.RequestCtx, state *core.SystemState, e common.Entry) bool {
	if e.IsDeleted {
		return false
	}
	if state.KeyCache != nil {
		state.KeyCache.Insert(e.Key, e.Value)
	}
	writeJSON(ctx, e.Key, e.Value)
	return true
}

func writeJSON(ctx *fasthttp.RequestCtx, key string, val []byte) {
	ctx.SetContentType("application/json")
	// Use fmt.Fprintf directly to ctx which implements Writer
	fmt.Fprintf(ctx, `{"key":"%s","val":"%s"}`, key, val)
}

func updateMetrics() {
	metrics.IncrementCacheHitCount()
	metrics.IncrementReadOperationsCount()
}

func (router *HttpApiRouter) HandleBatchPutRequest(ctx *fasthttp.RequestCtx) {
	var req BatchPutRequestPayload
	if err := json.Unmarshal(ctx.PostBody(), &req); err != nil {
		ctx.Error("Bad Request", fasthttp.StatusBadRequest)
		return
	}

	keys, vals, ttls := unpackBatch(&req)
	if err := agents.SubmitBatchIngestion(keys, vals, ttls); err != nil {
		ctx.Error(err.Error(), fasthttp.StatusInternalServerError)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusCreated)
}

func unpackBatch(req *BatchPutRequestPayload) ([]string, [][]byte, []int) {
	count := len(req.Items)
	keys := make([]string, count)
	vals := make([][]byte, count)
	ttls := make([]int, count)
	for i, item := range req.Items {
		keys[i] = item.Key
		vals[i] = []byte(item.Value)
		ttls[i] = item.TimeToLive
	}
	return keys, vals, ttls
}

func (router *HttpApiRouter) HandleDeleteRequest(ctx *fasthttp.RequestCtx) {
	keyBytes := ctx.QueryArgs().Peek("key")
	if len(keyBytes) == 0 {
		ctx.Error("Missing key", fasthttp.StatusBadRequest)
		return
	}
	key := string(keyBytes)

	if err := agents.SubmitIngestionRequest(key, nil, 0, true); err != nil {
		ctx.Error(err.Error(), fasthttp.StatusInternalServerError)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusOK)
}

func (router *HttpApiRouter) HandleMetricsRequest(ctx *fasthttp.RequestCtx) {
	ctx.SetContentType("application/json")
	json.NewEncoder(ctx).Encode(metrics.Global)
}
