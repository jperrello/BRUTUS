package provider

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os/exec"
	"sort"
	"strings"
	"time"
)

type SaturnService struct {
	Name           string
	Host           string
	Port           int
	Priority       int
	APIType        string
	EphemeralKey   string
	Features       []string
	APIBase        string // Remote API URL (e.g., "https://openrouter.ai/api/v1")
	SaturnVersion  string
	MaxConcurrent  int
	CurrentLoad    int
	Security       string
	HealthEndpoint string
	Models         []string
	GPU            string
	VRAMGb         int
	HealthStatus   string
}

func (s SaturnService) AvailableCapacity() int {
	if s.MaxConcurrent == 0 {
		return 0
	}
	avail := s.MaxConcurrent - s.CurrentLoad
	if avail < 0 {
		return 0
	}
	return avail
}

func (s SaturnService) LoadFraction() float64 {
	if s.MaxConcurrent == 0 {
		return 1.0
	}
	return float64(s.CurrentLoad) / float64(s.MaxConcurrent)
}

func SelectBestService(services []SaturnService) *SaturnService {
	if len(services) == 0 {
		return nil
	}

	var best *SaturnService
	bestScore := -1.0

	for i := range services {
		svc := &services[i]

		if svc.HealthStatus == "unhealthy" {
			continue
		}

		priorityScore := float64(100-svc.Priority) / 100.0
		loadScore := 1.0 - svc.LoadFraction()
		if loadScore < 0 {
			loadScore = 0
		}

		score := priorityScore*0.6 + loadScore*0.4

		if score > bestScore {
			bestScore = score
			best = svc
		}
	}

	return best
}

func (s SaturnService) URL() string {
	if s.APIBase != "" {
		return strings.TrimSuffix(s.APIBase, "/v1")
	}
	return fmt.Sprintf("http://%s:%d", s.Host, s.Port)
}

func DiscoverSaturn(ctx context.Context, timeout time.Duration) ([]SaturnService, error) {
	browseCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(browseCtx, "dns-sd", "-B", "_saturn._tcp", "local.")
	hideWindow(cmd)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Run()

	instances := parseBrowseOutput(stdout.String())
	if len(instances) == 0 {
		return nil, fmt.Errorf("no Saturn services found")
	}

	var services []SaturnService
	for _, instance := range instances {
		svc, err := resolveInstance(ctx, instance)
		if err != nil {
			continue
		}
		services = append(services, svc)
	}

	sort.Slice(services, func(i, j int) bool {
		return services[i].Priority < services[j].Priority
	})

	return services, nil
}


func parseTXTRecords(line string) map[string]string {
	result := make(map[string]string)
	pairs := strings.Fields(line)
	for _, pair := range pairs {
		if idx := strings.Index(pair, "="); idx > 0 {
			key := strings.TrimSpace(pair[:idx])
			value := strings.TrimSpace(pair[idx+1:])
			result[key] = value
		}
	}
	return result
}

func resolveHostname(hostname string) string {
	addrs, err := net.LookupHost(hostname)
	if err != nil || len(addrs) == 0 {
		return strings.TrimSuffix(hostname, ".local.")
	}

	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip != nil && !ip.IsLoopback() && ip.To4() != nil {
			return addr
		}
	}

	return addrs[0]
}
