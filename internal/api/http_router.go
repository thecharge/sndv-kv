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
	"sync"
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

var encoderPool = sync.Pool{
	New: func() interface{} { return json.NewEncoder(nil) },
}

func (router *HttpApiRouter) GetFastHTTPHandler() fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		router.handleRequest(ctx)
	}
}

func (router *HttpApiRouter) handleRequest(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	defer func() {
		recoverPanic(ctx)
		logger.LogAccessEvent("%s %s %s %v", string(ctx.Method()), string(ctx.Path()), ctx.RemoteAddr(), time.Since(startTime))
	}()

	if !router.checkAuth(ctx) {
		ctx.Error("Unauthorized", fasthttp.StatusUnauthorized)
		return
	}

	router.routePath(ctx)
}

func (router *HttpApiRouter) routePath(ctx *fasthttp.RequestCtx) {
	switch string(ctx.Path()) {
	case "/put":
		router.HandleSinglePutRequest(ctx)
	case "/get":
		router.HandleGetRequest(ctx)
	case "/batch":
		router.HandleBatchPutRequest(ctx)
	case "/delete":
		router.HandleDeleteRequest(ctx)
	case "/metrics":
		router.HandleMetricsRequest(ctx)
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
	if !isMethodAllowed(ctx, "POST", "PUT") {
		return
	}

	var payload SinglePutRequestPayload
	if err := json.Unmarshal(ctx.PostBody(), &payload); err != nil {
		ctx.Error("Bad Request", fasthttp.StatusBadRequest)
		return
	}

	if err := agents.SubmitIngestionRequest(payload.Key, []byte(payload.Value), payload.TimeToLive, false); err != nil {
		ctx.Error(err.Error(), fasthttp.StatusInternalServerError)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusCreated)
}

func (router *HttpApiRouter) HandleGetRequest(ctx *fasthttp.RequestCtx) {
	if !isMethodAllowed(ctx, "GET") {
		return
	}

	key := string(ctx.QueryArgs().Peek("key"))
	if key == "" {
		ctx.Error("Missing key", fasthttp.StatusBadRequest)
		return
	}

	if router.findAndServe(ctx, key) {
		return
	}

	ctx.Error("Not Found", fasthttp.StatusNotFound)
}

func (router *HttpApiRouter) findAndServe(ctx *fasthttp.RequestCtx, key string) bool {
	if tryServeFromCache(ctx, router.SystemState, key) {
		return true
	}
	if tryServeFromMemory(ctx, router.SystemState, key) {
		return true
	}
	return tryServeFromDisk(ctx, router.SystemState, key)
}

func tryServeFromCache(ctx *fasthttp.RequestCtx, state *core.SystemState, key string) bool {
	if state.KeyCache == nil {
		return false
	}
	if val, hit := state.KeyCache.RetrieveFromCache(key); hit {
		updateMetrics()
		writeJSON(ctx, key, val)
		return true
	}
	return false
}

func tryServeFromMemory(ctx *fasthttp.RequestCtx, state *core.SystemState, key string) bool {
	state.Mutex.RLock()
	defer state.Mutex.RUnlock()

	if e, ok := state.MemTable.Get(key); ok {
		return processEntry(ctx, state, e)
	}
	for i := len(state.ImmutableMem) - 1; i >= 0; i-- {
		if e, ok := state.ImmutableMem[i].Get(key); ok {
			return processEntry(ctx, state, e)
		}
	}
	return false
}

func tryServeFromDisk(ctx *fasthttp.RequestCtx, state *core.SystemState, key string) bool {
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
			return processEntry(ctx, state, e)
		}
	}
	return false
}

func processEntry(ctx *fasthttp.RequestCtx, state *core.SystemState, e common.Entry) bool {
	if e.IsDeleted {
		ctx.Error("Not Found", fasthttp.StatusNotFound)
		return true
	}
	if e.ExpiryTimestamp > 0 && time.Now().UnixNano() > e.ExpiryTimestamp {
		ctx.Error("Not Found", fasthttp.StatusNotFound)
		return true
	}

	if state.KeyCache != nil {
		state.KeyCache.InsertIntoCache(e.Key, e.Value)
	}
	writeJSON(ctx, e.Key, e.Value)
	return true
}

func (router *HttpApiRouter) HandleBatchPutRequest(ctx *fasthttp.RequestCtx) {
	if !isMethodAllowed(ctx, "POST") {
		return
	}

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

func (router *HttpApiRouter) HandleDeleteRequest(ctx *fasthttp.RequestCtx) {
	if !isMethodAllowed(ctx, "DELETE", "POST") {
		return
	}

	key := string(ctx.QueryArgs().Peek("key"))
	if key == "" {
		ctx.Error("Missing key", fasthttp.StatusBadRequest)
		return
	}
	if err := agents.SubmitIngestionRequest(key, nil, 0, true); err != nil {
		ctx.Error(err.Error(), fasthttp.StatusInternalServerError)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusOK)
}

func (router *HttpApiRouter) HandleMetricsRequest(ctx *fasthttp.RequestCtx) {
	if !isMethodAllowed(ctx, "GET") {
		return
	}
	ctx.SetContentType("application/json")
	json.NewEncoder(ctx).Encode(metrics.Global)
}

func isMethodAllowed(ctx *fasthttp.RequestCtx, methods ...string) bool {
	reqMethod := string(ctx.Method())
	for _, m := range methods {
		if reqMethod == m {
			return true
		}
	}
	ctx.Error("Method Not Allowed", fasthttp.StatusMethodNotAllowed)
	return false
}

func recoverPanic(ctx *fasthttp.RequestCtx) {
	if r := recover(); r != nil {
		logger.LogErrorEvent("PANIC: %v\n%s", r, debug.Stack())
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
	}
}

func unpackBatch(req *BatchPutRequestPayload) ([]string, [][]byte, []int) {
	count := len(req.Items)
	k, v, t := make([]string, count), make([][]byte, count), make([]int, count)
	for i, item := range req.Items {
		k[i], v[i], t[i] = item.Key, []byte(item.Value), item.TimeToLive
	}
	return k, v, t
}

func writeJSON(ctx *fasthttp.RequestCtx, key string, val []byte) {
	ctx.SetContentType("application/json")
	fmt.Fprintf(ctx, `{"key":"%s","val":"%s"}`, key, val)
}

func updateMetrics() {
	metrics.IncrementCacheHitCount()
	metrics.IncrementReadOperationsCount()
}
