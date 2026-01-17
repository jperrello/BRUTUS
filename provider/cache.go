package provider

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"
)

var globalServiceCache = NewServiceCache(30 * time.Second)

type Discoverer interface {
	Discover(ctx context.Context, timeout time.Duration) ([]SaturnService, error)
	DiscoverFiltered(ctx context.Context, timeout time.Duration, filter DiscoveryFilter) ([]SaturnService, error)
}

func CreateDiscoverer(cache *ServiceCache) Discoverer {
	return NewZeroconfDiscoverer(cache)
}

func createPooledTransport() *http.Transport {
	return &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
}

type CachedService struct {
	Service   SaturnService
	CachedAt  time.Time
	ExpiresAt time.Time
}

type RefreshFunc func(ctx context.Context) ([]SaturnService, error)

type ServiceCache struct {
	mu       sync.RWMutex
	services map[string]CachedService
	ttl      time.Duration

	refreshFn     RefreshFunc
	refreshCtx    context.Context
	refreshCancel context.CancelFunc
	refreshWg     sync.WaitGroup
}

func NewServiceCache(ttl time.Duration) *ServiceCache {
	if ttl == 0 {
		ttl = 30 * time.Second
	}
	return &ServiceCache{
		services: make(map[string]CachedService),
		ttl:      ttl,
	}
}

func (c *ServiceCache) Get(name string) (SaturnService, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cached, ok := c.services[name]
	if !ok {
		return SaturnService{}, false
	}

	if c.isExpired(cached) {
		return SaturnService{}, false
	}

	return cached.Service, true
}

func (c *ServiceCache) GetAll() []SaturnService {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var services []SaturnService
	for _, cached := range c.services {
		if !c.isExpired(cached) {
			services = append(services, cached.Service)
		}
	}
	return services
}

func (c *ServiceCache) Set(svc SaturnService) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	c.services[svc.Name] = CachedService{
		Service:   svc,
		CachedAt:  now,
		ExpiresAt: now.Add(c.ttl),
	}
}

func (c *ServiceCache) SetAll(services []SaturnService) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for _, svc := range services {
		c.services[svc.Name] = CachedService{
			Service:   svc,
			CachedAt:  now,
			ExpiresAt: now.Add(c.ttl),
		}
	}
}

func (c *ServiceCache) Remove(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.services, name)
}

func (c *ServiceCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.services = make(map[string]CachedService)
}

func (c *ServiceCache) isExpired(cached CachedService) bool {
	return time.Now().After(cached.ExpiresAt)
}

func (c *ServiceCache) PruneExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for name, cached := range c.services {
		if now.After(cached.ExpiresAt) {
			delete(c.services, name)
		}
	}
}

func (c *ServiceCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.services)
}

func (c *ServiceCache) StartBackgroundRefresh(refreshFn RefreshFunc) {
	c.mu.Lock()
	if c.refreshCancel != nil {
		c.mu.Unlock()
		return
	}
	c.refreshFn = refreshFn
	c.refreshCtx, c.refreshCancel = context.WithCancel(context.Background())
	c.mu.Unlock()

	c.refreshWg.Add(1)
	go c.backgroundRefreshLoop()
}

func (c *ServiceCache) StopBackgroundRefresh() {
	c.mu.Lock()
	if c.refreshCancel != nil {
		c.refreshCancel()
		c.refreshCancel = nil
	}
	c.mu.Unlock()
	c.refreshWg.Wait()
}

func (c *ServiceCache) backgroundRefreshLoop() {
	defer c.refreshWg.Done()

	checkInterval := c.ttl / 4
	minInterval := time.Second
	if c.ttl < 10*time.Second {
		minInterval = 10 * time.Millisecond
	}
	if checkInterval < minInterval {
		checkInterval = minInterval
	}

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.refreshCtx.Done():
			return
		case <-ticker.C:
			c.refreshIfNeeded()
		}
	}
}

func (c *ServiceCache) refreshIfNeeded() {
	c.mu.RLock()
	refreshFn := c.refreshFn
	ctx := c.refreshCtx
	if refreshFn == nil || ctx == nil {
		c.mu.RUnlock()
		return
	}

	refreshThreshold := c.ttl * 80 / 100
	now := time.Now()
	needsRefresh := false
	hasValidEntries := false

	for _, cached := range c.services {
		timeUntilExpiry := cached.ExpiresAt.Sub(now)
		if timeUntilExpiry > 0 {
			hasValidEntries = true
			if timeUntilExpiry <= refreshThreshold {
				needsRefresh = true
				break
			}
		}
	}

	if !hasValidEntries && len(c.services) > 0 {
		needsRefresh = true
	}
	c.mu.RUnlock()

	if !needsRefresh {
		return
	}

	services, err := refreshFn(ctx)
	if err != nil {
		return
	}

	c.SetAll(services)
}
