package mockprovider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestServer_Healthz(t *testing.T) {
	srv := httptest.NewServer(NewServer(NewStore(), &RealClock{}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %q", body["status"])
	}
}

func TestServer_InvokeThenStatusReflectsWarmingState(t *testing.T) {
	clock := NewVirtualClock(time.Now())
	srv := httptest.NewServer(NewServer(NewStore(), clock))
	defer srv.Close()

	invokeResp, err := http.Post(srv.URL+"/v1/endpoints/ep_demo/invoke", "application/json", nil)
	if err != nil {
		t.Fatalf("POST invoke: %v", err)
	}
	defer invokeResp.Body.Close()

	var invoked invokeResponse
	if err := json.NewDecoder(invokeResp.Body).Decode(&invoked); err != nil {
		t.Fatalf("decode invoke response: %v", err)
	}
	if !invoked.WasColdStart {
		t.Fatalf("expected first invoke on an unseen endpoint to be a cold start")
	}

	statusResp, err := http.Get(srv.URL + "/v1/endpoints/ep_demo")
	if err != nil {
		t.Fatalf("GET status: %v", err)
	}
	defer statusResp.Body.Close()

	var status statusResponse
	if err := json.NewDecoder(statusResp.Body).Decode(&status); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	if status.WorkerState != WarmingState {
		t.Fatalf("expected worker_state %q immediately after a cold invoke, got %q", WarmingState, status.WorkerState)
	}

	clock.Advance(ColdStartDuration + time.Second)

	statusResp2, err := http.Get(srv.URL + "/v1/endpoints/ep_demo")
	if err != nil {
		t.Fatalf("GET status (after warm-up): %v", err)
	}
	defer statusResp2.Body.Close()

	var status2 statusResponse
	if err := json.NewDecoder(statusResp2.Body).Decode(&status2); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	if status2.WorkerState != WarmState {
		t.Fatalf("expected worker_state %q after the clock advanced past ColdStartDuration, got %q", WarmState, status2.WorkerState)
	}
	if status2.WorkerCount != 1 {
		t.Fatalf("expected worker_count 1 once warm, got %d", status2.WorkerCount)
	}
}

// TestServer_ConcurrentInvokesDoNotRace fires many real, concurrent HTTP
// requests at the same endpoint. Run with -race to prove the store's
// locking actually holds up under real concurrent traffic, not just
// sequential test calls.
func TestServer_ConcurrentInvokesDoNotRace(t *testing.T) {
	clock := NewVirtualClock(time.Now())
	srv := httptest.NewServer(NewServer(NewStore(), clock))
	defer srv.Close()

	const concurrentRequests = 50
	var wg sync.WaitGroup
	wg.Add(concurrentRequests)

	for i := 0; i < concurrentRequests; i++ {
		go func() {
			defer wg.Done()

			resp, err := http.Post(srv.URL+"/v1/endpoints/ep_demo/invoke", "application/json", nil)
			if err != nil {
				t.Errorf("POST invoke: %v", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("expected 200, got %d", resp.StatusCode)
			}
		}()
	}

	wg.Wait()

	statusResp, err := http.Get(srv.URL + "/v1/endpoints/ep_demo")
	if err != nil {
		t.Fatalf("GET status: %v", err)
	}
	defer statusResp.Body.Close()

	var status statusResponse
	if err := json.NewDecoder(statusResp.Body).Decode(&status); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	if status.WorkerState != WarmingState {
		t.Fatalf("expected worker_state %q after concurrent invokes with no time advanced, got %q", WarmingState, status.WorkerState)
	}
}
