package provider

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestBackgroundRefresh(t *testing.T) {
	cache := NewServiceCache(200 * time.Millisecond)

	var refreshCount int32
	refreshFn := func(ctx context.Context) ([]SaturnService, error) {
		atomic.AddInt32(&refreshCount, 1)
		return []SaturnService{
			{Name: "test-service", Host: "localhost", Port: 8080},
		}, nil
	}

	cache.Set(SaturnService{Name: "initial", Host: "localhost", Port: 8080})
	cache.StartBackgroundRefresh(refreshFn)
	defer cache.StopBackgroundRefresh()

	time.Sleep(300 * time.Millisecond)

	count := atomic.LoadInt32(&refreshCount)
	if count == 0 {
		t.Errorf("expected background refresh to be called at least once, got %d", count)
	}

	svc, ok := cache.Get("test-service")
	if !ok {
		t.Error("expected refreshed service to be in cache")
	} else if svc.Name != "test-service" {
		t.Errorf("expected service name 'test-service', got '%s'", svc.Name)
	}
}

func TestBackgroundRefreshStops(t *testing.T) {
	cache := NewServiceCache(50 * time.Millisecond)

	var refreshCount int32
	refreshFn := func(ctx context.Context) ([]SaturnService, error) {
		atomic.AddInt32(&refreshCount, 1)
		return nil, nil
	}

	cache.Set(SaturnService{Name: "test", Host: "localhost", Port: 8080})
	cache.StartBackgroundRefresh(refreshFn)
	time.Sleep(30 * time.Millisecond)
	cache.StopBackgroundRefresh()

	countAfterStop := atomic.LoadInt32(&refreshCount)
	time.Sleep(100 * time.Millisecond)
	countLater := atomic.LoadInt32(&refreshCount)

	if countLater > countAfterStop {
		t.Errorf("refresh continued after stop: before=%d, after=%d", countAfterStop, countLater)
	}
}

func TestCacheExpiry(t *testing.T) {
	cache := NewServiceCache(50 * time.Millisecond)

	cache.Set(SaturnService{Name: "test", Host: "localhost", Port: 8080})

	_, ok := cache.Get("test")
	if !ok {
		t.Error("expected service to be available immediately after set")
	}

	time.Sleep(60 * time.Millisecond)

	_, ok = cache.Get("test")
	if ok {
		t.Error("expected service to be expired after TTL")
	}
}

func TestLoadAwareSelection(t *testing.T) {
	services := []SaturnService{
		{Name: "busy", Host: "localhost", Port: 8081, MaxConcurrent: 10, CurrentLoad: 9},
		{Name: "idle", Host: "localhost", Port: 8082, MaxConcurrent: 10, CurrentLoad: 1},
		{Name: "full", Host: "localhost", Port: 8083, MaxConcurrent: 10, CurrentLoad: 10},
	}

	best := SelectBestService(services)
	if best == nil {
		t.Fatal("expected a service to be selected")
	}
	if best.Name != "idle" {
		t.Errorf("expected 'idle' to be selected (lowest load), got '%s'", best.Name)
	}
}

func TestLoadAwareSelectionWithPriority(t *testing.T) {
	services := []SaturnService{
		{Name: "high-priority-idle", Host: "localhost", Port: 8081, MaxConcurrent: 10, CurrentLoad: 1, Priority: 1},
		{Name: "low-priority-busy", Host: "localhost", Port: 8082, MaxConcurrent: 10, CurrentLoad: 5, Priority: 50},
	}

	best := SelectBestService(services)
	if best == nil {
		t.Fatal("expected a service to be selected")
	}
	if best.Name != "high-priority-idle" {
		t.Errorf("expected 'high-priority-idle' to win due to priority (lower number = higher priority), got '%s'", best.Name)
	}
}

func TestLoadAwareSelectionWithHealth(t *testing.T) {
	services := []SaturnService{
		{Name: "healthy", Host: "localhost", Port: 8081, MaxConcurrent: 10, CurrentLoad: 5, HealthStatus: "healthy"},
		{Name: "unhealthy", Host: "localhost", Port: 8082, MaxConcurrent: 10, CurrentLoad: 1, HealthStatus: "unhealthy"},
	}

	best := SelectBestService(services)
	if best == nil {
		t.Fatal("expected a service to be selected")
	}
	if best.Name != "healthy" {
		t.Errorf("expected 'healthy' to win despite higher load, got '%s'", best.Name)
	}
}

func TestAvailableCapacity(t *testing.T) {
	tests := []struct {
		name          string
		svc           SaturnService
		wantCapacity  int
		wantFraction  float64
	}{
		{"full load", SaturnService{MaxConcurrent: 10, CurrentLoad: 10}, 0, 1.0},
		{"half load", SaturnService{MaxConcurrent: 10, CurrentLoad: 5}, 5, 0.5},
		{"no load", SaturnService{MaxConcurrent: 10, CurrentLoad: 0}, 10, 0.0},
		{"over load", SaturnService{MaxConcurrent: 10, CurrentLoad: 15}, 0, 1.5},
		{"no max", SaturnService{MaxConcurrent: 0, CurrentLoad: 5}, 0, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCap := tt.svc.AvailableCapacity()
			if gotCap != tt.wantCapacity {
				t.Errorf("AvailableCapacity() = %d, want %d", gotCap, tt.wantCapacity)
			}
			gotFrac := tt.svc.LoadFraction()
			if gotFrac != tt.wantFraction {
				t.Errorf("LoadFraction() = %f, want %f", gotFrac, tt.wantFraction)
			}
		})
	}
}
