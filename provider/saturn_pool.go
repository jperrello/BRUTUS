package provider

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"brutus/tools"
)

type SaturnPool struct {
	services   []SaturnService
	httpClient *http.Client
	model      string
	maxTokens  int

	current atomic.Uint32
	mu      sync.RWMutex
}

type SaturnPoolConfig struct {
	DiscoveryTimeout time.Duration
	Model            string
	MaxTokens        int
	Filter           *DiscoveryFilter
	MinServices      int
}

func NewSaturnPool(ctx context.Context, cfg SaturnPoolConfig) (*SaturnPool, error) {
	if cfg.DiscoveryTimeout == 0 {
		cfg.DiscoveryTimeout = 3 * time.Second
	}

	discoverer := CreateDiscoverer(globalServiceCache)

	var services []SaturnService
	var err error

	if cfg.Filter != nil {
		services, err = discoverer.DiscoverFiltered(ctx, cfg.DiscoveryTimeout, *cfg.Filter)
	} else {
		services, err = discoverer.Discover(ctx, cfg.DiscoveryTimeout)
	}

	if err != nil {
		return nil, fmt.Errorf("saturn pool discovery failed: %w", err)
	}

	if len(services) == 0 {
		return nil, fmt.Errorf("no saturn services found on network")
	}

	if cfg.MinServices > 0 && len(services) < cfg.MinServices {
		return nil, fmt.Errorf("found %d services, need at least %d", len(services), cfg.MinServices)
	}

	var healthy []SaturnService
	for _, svc := range services {
		if healthCheck(svc) == nil {
			healthy = append(healthy, svc)
		}
	}

	if len(healthy) == 0 {
		healthy = services
	}

	return &SaturnPool{
		services: healthy,
		httpClient: &http.Client{
			Timeout:   120 * time.Second,
			Transport: createPooledTransport(),
		},
		model:     cfg.Model,
		maxTokens: cfg.MaxTokens,
	}, nil
}

func (p *SaturnPool) Name() string {
	return fmt.Sprintf("saturn-pool(%d services)", len(p.services))
}

func (p *SaturnPool) GetModel() string {
	return p.model
}

func (p *SaturnPool) SetModel(model string) {
	p.model = model
}

func (p *SaturnPool) GetServices() []SaturnService {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]SaturnService, len(p.services))
	copy(result, p.services)
	return result
}

func (p *SaturnPool) ServiceCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.services)
}

func (p *SaturnPool) next() *SaturnService {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if len(p.services) == 0 {
		return nil
	}
	idx := p.current.Add(1) - 1
	return &p.services[idx%uint32(len(p.services))]
}

func (p *SaturnPool) nextN(start int, count int) []*SaturnService {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if len(p.services) == 0 {
		return nil
	}

	result := make([]*SaturnService, 0, count)
	for i := 0; i < count && i < len(p.services); i++ {
		idx := (start + i) % len(p.services)
		result = append(result, &p.services[idx])
	}
	return result
}

func (p *SaturnPool) ListModels(ctx context.Context) ([]ModelInfo, error) {
	svc := p.next()
	if svc == nil {
		return nil, fmt.Errorf("no services available")
	}

	single := &Saturn{
		service:    svc,
		httpClient: p.httpClient,
		model:      p.model,
	}
	return single.ListModels(ctx)
}

func (p *SaturnPool) Chat(ctx context.Context, systemPrompt string, messages []Message, toolDefs []tools.Tool) (Message, error) {
	startIdx := int(p.current.Add(1) - 1)
	services := p.nextN(startIdx, len(p.services))

	var lastErr error
	for _, svc := range services {
		single := &Saturn{
			service:    svc,
			httpClient: p.httpClient,
			model:      p.model,
			maxTokens:  p.maxTokens,
		}

		msg, err := single.Chat(ctx, systemPrompt, messages, toolDefs)
		if err == nil {
			return msg, nil
		}
		lastErr = err
	}

	return Message{}, fmt.Errorf("all %d services failed, last error: %w", len(services), lastErr)
}

func (p *SaturnPool) ChatStream(ctx context.Context, systemPrompt string, messages []Message, toolDefs []tools.Tool) (<-chan StreamDelta, error) {
	startIdx := int(p.current.Add(1) - 1)
	services := p.nextN(startIdx, len(p.services))

	var lastErr error
	for _, svc := range services {
		single := &Saturn{
			service:    svc,
			httpClient: p.httpClient,
			model:      p.model,
			maxTokens:  p.maxTokens,
		}

		ch, err := single.ChatStream(ctx, systemPrompt, messages, toolDefs)
		if err == nil {
			return ch, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("all %d services failed, last error: %w", len(services), lastErr)
}
