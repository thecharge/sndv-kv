package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"sndv-kv/internal/agents"
	"sndv-kv/internal/config"
	"sndv-kv/internal/core"
	"testing"
)

func TestHandlePutAndGet(t *testing.T) {
	cfg, _ := config.Load("")
	bb := core.NewBlackboard(cfg)

	agents.InitIngest(bb)

	router := &Router{BB: bb}

	payload := map[string]interface{}{
		"key": "testk", "value": "testv", "ttl": 0,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/put", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	router.HandlePut(w, req)
	if w.Code != 201 {
		t.Errorf("Expected 201, got %d", w.Code)
	}

	reqGet := httptest.NewRequest("GET", "/get?key=testk", nil)
	wGet := httptest.NewRecorder()

	router.HandleGet(wGet, reqGet)
	if wGet.Code != 200 {
		t.Errorf("Expected 200, got %d", wGet.Code)
	}
}
