package provider

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"
)

type ZeroconfDiscoverer struct {
	cache    *ServiceCache
	fallback *LegacyDiscoverer
}

func NewZeroconfDiscoverer(cache *ServiceCache) *ZeroconfDiscoverer {
	return &ZeroconfDiscoverer{
		cache:    cache,
		fallback: NewLegacyDiscoverer(cache),
	}
}

func (d *ZeroconfDiscoverer) Discover(ctx context.Context, timeout time.Duration) ([]SaturnService, error) {
	if d.cache != nil {
		if cached := d.cache.GetAll(); len(cached) > 0 {
			return cached, nil
		}
	}

	services, err := d.discoverZeroconf(ctx, timeout)
	if err != nil {
		services, err = d.fallback.Discover(ctx, timeout)
		if err != nil {
			return nil, err
		}
	}

	if d.cache != nil && len(services) > 0 {
		d.cache.SetAll(services)
	}

	return services, nil
}

func (d *ZeroconfDiscoverer) DiscoverFiltered(ctx context.Context, timeout time.Duration, filter DiscoveryFilter) ([]SaturnService, error) {
	services, err := d.Discover(ctx, timeout)
	if err != nil {
		return nil, err
	}

	return FilterServices(services, filter), nil
}

func (d *ZeroconfDiscoverer) discoverZeroconf(ctx context.Context, timeout time.Duration) ([]SaturnService, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create zeroconf resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry, 10)
	var services []SaturnService

	browseCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		for entry := range entries {
			if svc := parseZeroconfEntry(entry); svc.Name != "" {
				services = append(services, svc)
			}
		}
		close(done)
	}()

	err = resolver.Browse(browseCtx, "_saturn._tcp", "local.", entries)
	if err != nil {
		return nil, fmt.Errorf("zeroconf browse failed: %w", err)
	}

	<-browseCtx.Done()
	close(entries)
	<-done

	if len(services) == 0 {
		return nil, fmt.Errorf("no Saturn services found via zeroconf")
	}

	sort.Slice(services, func(i, j int) bool {
		return services[i].Priority < services[j].Priority
	})

	return services, nil
}

func parseZeroconfEntry(entry *zeroconf.ServiceEntry) SaturnService {
	svc := SaturnService{
		Name:     entry.Instance,
		Port:     entry.Port,
		Priority: 100,
		APIType:  "openai",
	}

	if len(entry.AddrIPv4) > 0 {
		svc.Host = entry.AddrIPv4[0].String()
	} else if len(entry.AddrIPv6) > 0 {
		svc.Host = entry.AddrIPv6[0].String()
	} else if entry.HostName != "" {
		svc.Host = resolveHostname(entry.HostName)
	}

	for _, txt := range entry.Text {
		if idx := strings.Index(txt, "="); idx > 0 {
			key := txt[:idx]
			value := txt[idx+1:]

			switch key {
			case "priority":
				svc.Priority, _ = strconv.Atoi(value)
			case "api":
				svc.APIType = value
			case "api_base":
				svc.APIBase = value
			case "ephemeral_key":
				svc.EphemeralKey = value
			case "features":
				svc.Features = strings.Split(value, ",")
			case "version":
				svc.SaturnVersion = value
			case "max_concurrent":
				svc.MaxConcurrent, _ = strconv.Atoi(value)
			case "current_load":
				svc.CurrentLoad, _ = strconv.Atoi(value)
			case "security":
				svc.Security = value
			case "health_endpoint":
				svc.HealthEndpoint = value
			case "models":
				svc.Models = strings.Split(value, ",")
			case "gpu":
				svc.GPU = value
			case "vram_gb":
				svc.VRAMGb, _ = strconv.Atoi(value)
			}
		}
	}

	return svc
}
