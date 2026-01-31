package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"sndv-kv/internal/agents"
	"sndv-kv/internal/core"
	"sndv-kv/internal/logger"
	"sndv-kv/internal/metrics"
	"sndv-kv/internal/storage"
	"time"

	"github.com/o1egl/paseto"
)

type Router struct {
	BB *core.Blackboard
}

type Payload struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	TTL   int    `json:"ttl"`
}

type BatchPutReq struct {
	Items []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
		TTL   int    `json:"ttl"`
	} `json:"items"`
}

func (api *Router) MiddlewareAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			logger.Access("%s %s %s %v", r.Method, r.URL.Path, r.RemoteAddr, time.Since(start))
		}()

		token := r.Header.Get("Authorization")
		if api.BB.Config.AuthToken == "" && token == "" {
			next(w, r)
			return
		}

		var footer string
		var claims paseto.JSONToken
		key := []byte(fmt.Sprintf("%-32s", api.BB.Config.AuthSecret))[:32]

		if err := paseto.NewV2().Decrypt(token, key, &claims, &footer); err != nil {
			http.Error(w, "Unauthorized", 401)
			return
		}
		next(w, r)
	}
}

func (api *Router) HandlePut(w http.ResponseWriter, r *http.Request) {
	var p Payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "Bad Request", 400)
		return
	}
	if err := agents.SubmitIngest(p.Key, []byte(p.Value), p.TTL, false); err != nil {
		logger.Error("Put Error: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(201)
}

func (api *Router) HandleDelete(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if err := agents.SubmitIngest(key, nil, 0, true); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(200)
}

func (api *Router) HandleGet(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("PANIC in Get: %v\n%s", r, debug.Stack())
			http.Error(w, "Internal Server Error", 500)
		}
	}()

	key := r.URL.Query().Get("key")

	// 1. Cache
	if api.BB.KeyCache != nil {
		if val, hit := api.BB.KeyCache.Get(key); hit {
			metrics.IncCacheHit()
			metrics.IncRead()
			json.NewEncoder(w).Encode(map[string]string{"key": key, "val": string(val)})
			return
		}
	}

	api.BB.Mu.RLock()
	// 2. MemTable
	if val, ok := api.BB.MemTable.Get(key); ok {
		api.BB.Mu.RUnlock()
		if val.Deleted {
			http.Error(w, "Not Found", 404)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"key": key, "val": string(val.Value)})
		return
	}

	// 3. Immutable MemTables
	for i := len(api.BB.ImmutableMem) - 1; i >= 0; i-- {
		if v, found := api.BB.ImmutableMem[i].Get(key); found {
			api.BB.Mu.RUnlock()
			if v.Deleted {
				http.Error(w, "Not Found", 404)
				return
			}
			if api.BB.KeyCache != nil {
				api.BB.KeyCache.Put(key, v.Value)
			}
			json.NewEncoder(w).Encode(map[string]string{"key": key, "val": string(v.Value)})
			return
		}
	}

	sstables := api.BB.SSTables
	sharedBloom := api.BB.SharedBloom
	api.BB.Mu.RUnlock()

	// 4. Disk Check
	for _, level := range sstables {
		for i := len(level) - 1; i >= 0; i-- {
			meta := level[i]

			// 1. Correct Bloom Access: Use the Blackboard's SharedBloom + FileID
			if sharedBloom != nil && !sharedBloom.MayContain(meta.FileID, []byte(key)) {
				continue
			}

			// 2. Disk Seek
			entry, found := storage.FindInSSTable(meta, key)
			if found {
				if entry.Deleted {
					http.Error(w, "Not Found", 404)
					return
				}

				if entry.ExpiresAt > 0 && time.Now().UnixNano() > entry.ExpiresAt {
					http.Error(w, "Not Found", 404)
					return
				}

				if api.BB.KeyCache != nil {
					api.BB.KeyCache.Put(key, entry.Value)
				}
				json.NewEncoder(w).Encode(map[string]string{"key": key, "val": string(entry.Value)})
				return
			}
		}
	}

	http.Error(w, "Not Found", 404)
}

func (api *Router) HandleBatchPut(w http.ResponseWriter, r *http.Request) {
	var req BatchPutReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", 400)
		return
	}

	keys := make([]string, len(req.Items))
	vals := make([][]byte, len(req.Items))
	ttls := make([]int, len(req.Items))

	for i, item := range req.Items {
		keys[i] = item.Key
		vals[i] = []byte(item.Value)
		ttls[i] = item.TTL
	}

	if err := agents.SubmitBatch(keys, vals, ttls); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(201)
}

func (api *Router) HandleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics.Global)
}
