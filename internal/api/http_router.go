package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"sndv-kv/internal/agents"
	"sndv-kv/internal/common"
	"sndv-kv/internal/core"
	"sndv-kv/internal/logger"
	"sndv-kv/internal/metrics"
	"sndv-kv/internal/storage"
	"time"

	"github.com/o1egl/paseto"
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

func (router *HttpApiRouter) ApplyAuthenticationMiddleware(nextHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		defer func() {
			logger.LogAccessEvent("%s %s %s %v", r.Method, r.URL.Path, r.RemoteAddr, time.Since(startTime))
		}()

		if isAuthDisabled(router.SystemState.Configuration.AuthenticationToken, r.Header.Get("Authorization")) {
			nextHandler(w, r)
			return
		}

		if err := verifyToken(r.Header.Get("Authorization"), router.SystemState.Configuration.AuthenticationSecret); err != nil {
			http.Error(w, "Unauthorized: Invalid Token", 401)
			return
		}
		nextHandler(w, r)
	}
}

func isAuthDisabled(configToken, headerToken string) bool {
	return configToken == "" && headerToken == ""
}

func verifyToken(tokenString, secret string) error {
	var footer string
	var tokenClaims paseto.JSONToken
	secretKey := []byte(fmt.Sprintf("%-32s", secret))[:32]
	return paseto.NewV2().Decrypt(tokenString, secretKey, &tokenClaims, &footer)
}

func (router *HttpApiRouter) HandleSinglePutRequest(w http.ResponseWriter, r *http.Request) {
	var payload SinglePutRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Bad Request: Invalid JSON", 400)
		return
	}

	if err := agents.SubmitIngestionRequest(payload.Key, []byte(payload.Value), payload.TimeToLive, false); err != nil {
		logger.LogErrorEvent("PUT Operation Failed: %v", err)
		http.Error(w, fmt.Sprintf("Internal Error: %v", err), 500)
		return
	}
	w.WriteHeader(201)
}

func (router *HttpApiRouter) HandleGetRequest(w http.ResponseWriter, r *http.Request) {
	defer recoverPanic(w)

	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Bad Request: Missing key parameter", 400)
		return
	}

	if serveFromCache(w, router.SystemState, key) {
		return
	}

	if serveFromMemory(w, router.SystemState, key) {
		return
	}

	if serveFromDisk(w, router.SystemState, key) {
		return
	}

	http.Error(w, "Not Found", 404)
}

func recoverPanic(w http.ResponseWriter) {
	if r := recover(); r != nil {
		logger.LogErrorEvent("PANIC in HandleGetRequest: %v\n%s", r, debug.Stack())
		http.Error(w, "Internal Server Error", 500)
	}
}

func serveFromCache(w http.ResponseWriter, state *core.SystemState, key string) bool {
	if state.KeyCache == nil {
		return false
	}
	if value, hit := state.KeyCache.Retrieve(key); hit {
		metrics.IncrementCacheHitCount()
		metrics.IncrementReadOperationsCount()
		writeJsonResponse(w, key, string(value))
		return true
	}
	return false
}

func serveFromMemory(w http.ResponseWriter, state *core.SystemState, key string) bool {
	state.Mutex.RLock()
	defer state.Mutex.RUnlock()

	// 1. Active MemTable
	if entry, found := state.MemTable.Get(key); found {
		return processFoundEntry(w, state, entry)
	}

	// 2. Immutable MemTables
	for i := len(state.ImmutableMem) - 1; i >= 0; i-- {
		if entry, found := state.ImmutableMem[i].Get(key); found {
			return processFoundEntry(w, state, entry)
		}
	}
	return false
}

func serveFromDisk(w http.ResponseWriter, state *core.SystemState, key string) bool {
	// Snapshot state to avoid holding lock during IO
	state.Mutex.RLock()
	levels := state.SSTables
	bloom := state.BloomFilter
	state.Mutex.RUnlock()

	for _, levelTables := range levels {
		if found := searchLevel(w, state, levelTables, bloom, key); found {
			return true
		}
	}
	return false
}

func searchLevel(w http.ResponseWriter, state *core.SystemState, tables []storage.SSTableMetadata, bloom common.BloomFilter, key string) bool {
	for i := len(tables) - 1; i >= 0; i-- {
		meta := tables[i]
		if bloom != nil && !bloom.Contains(meta.FileID, []byte(key)) {
			continue
		}

		entry, found := storage.FindInSSTable(meta, key)
		if found {
			return processFoundEntry(w, state, entry)
		}
	}
	return false
}

func processFoundEntry(w http.ResponseWriter, state *core.SystemState, entry common.Entry) bool {
	if entry.IsDeleted {
		http.Error(w, "Not Found (Deleted)", 404)
		return true
	}
	if entry.ExpiryTimestamp > 0 && time.Now().UnixNano() > entry.ExpiryTimestamp {
		http.Error(w, "Not Found (Expired)", 404)
		return true
	}

	if state.KeyCache != nil {
		state.KeyCache.Insert(entry.Key, entry.Value)
	}
	writeJsonResponse(w, entry.Key, string(entry.Value))
	return true
}

func writeJsonResponse(w http.ResponseWriter, key, value string) {
	json.NewEncoder(w).Encode(map[string]string{"key": key, "val": value})
}

func (router *HttpApiRouter) HandleBatchPutRequest(w http.ResponseWriter, r *http.Request) {
	var request BatchPutRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Bad Request", 400)
		return
	}

	keys := make([]string, len(request.Items))
	vals := make([][]byte, len(request.Items))
	ttls := make([]int, len(request.Items))

	for i, item := range request.Items {
		keys[i] = item.Key
		vals[i] = []byte(item.Value)
		ttls[i] = item.TimeToLive
	}

	if err := agents.SubmitBatchIngestion(keys, vals, ttls); err != nil {
		http.Error(w, fmt.Sprintf("Batch Failure: %v", err), 500)
		return
	}
	w.WriteHeader(201)
}

func (router *HttpApiRouter) HandleDeleteRequest(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Missing key", 400)
		return
	}
	if err := agents.SubmitIngestionRequest(key, nil, 0, true); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(200)
}

func (router *HttpApiRouter) HandleMetricsRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics.Global)
}
