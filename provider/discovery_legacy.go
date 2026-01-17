package provider

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type LegacyDiscoverer struct {
	cache *ServiceCache
}

func NewLegacyDiscoverer(cache *ServiceCache) *LegacyDiscoverer {
	return &LegacyDiscoverer{cache: cache}
}

func (d *LegacyDiscoverer) Discover(ctx context.Context, timeout time.Duration) ([]SaturnService, error) {
	if d.cache != nil {
		if cached := d.cache.GetAll(); len(cached) > 0 {
			return cached, nil
		}
	}

	services, err := discoverSaturnDNSSD(ctx, timeout)
	if err != nil {
		return nil, err
	}

	if d.cache != nil {
		d.cache.SetAll(services)
	}

	return services, nil
}

func (d *LegacyDiscoverer) DiscoverFiltered(ctx context.Context, timeout time.Duration, filter DiscoveryFilter) ([]SaturnService, error) {
	services, err := d.Discover(ctx, timeout)
	if err != nil {
		return nil, err
	}

	return FilterServices(services, filter), nil
}

func discoverSaturnDNSSD(ctx context.Context, timeout time.Duration) ([]SaturnService, error) {
	browseCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(browseCtx, "dns-sd", "-B", "_saturn._tcp", "local.")
	hideCommandWindow(cmd)
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

func parseBrowseOutput(output string) []string {
	var instances []string
	seen := make(map[string]bool)

	output = strings.ReplaceAll(output, "\r\n", "\n")
	output = strings.ReplaceAll(output, "\r", "\n")
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "_saturn._tcp.") {
			continue
		}
		if strings.Contains(line, "Service Type") || strings.HasPrefix(line, "Browsing for") {
			continue
		}

		idx := strings.Index(line, "_saturn._tcp.")
		if idx == -1 {
			continue
		}
		remainder := strings.TrimSpace(line[idx+len("_saturn._tcp."):])
		if remainder != "" && !seen[remainder] {
			instances = append(instances, remainder)
			seen[remainder] = true
		}
	}

	return instances
}

func resolveInstance(ctx context.Context, instance string) (SaturnService, error) {
	resolveCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(resolveCtx, "dns-sd", "-L", instance, "_saturn._tcp", "local.")
	hideCommandWindow(cmd)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Run()

	return parseResolveOutput(instance, stdout.String())
}

func parseResolveOutput(instance, output string) (SaturnService, error) {
	svc := SaturnService{
		Name:     instance,
		Priority: 100,
		APIType:  "openai",
	}

	output = strings.ReplaceAll(output, "\r\n", "\n")
	output = strings.ReplaceAll(output, "\r", "\n")
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if hostPortRe := regexp.MustCompile(`(\S+\.local\.):(\d+)`); hostPortRe.MatchString(line) {
			matches := hostPortRe.FindStringSubmatch(line)
			if len(matches) >= 3 {
				svc.Host = resolveHostname(matches[1])
				svc.Port, _ = strconv.Atoi(matches[2])
			}
		}

		if strings.Contains(line, "=") {
			pairs := parseTXTRecords(line)
			for k, v := range pairs {
				switch k {
				case "priority":
					svc.Priority, _ = strconv.Atoi(v)
				case "api":
					svc.APIType = v
				case "api_base":
					svc.APIBase = v
				case "ephemeral_key":
					svc.EphemeralKey = v
				case "features":
					svc.Features = strings.Split(v, ",")
				case "version":
					svc.SaturnVersion = v
				case "max_concurrent":
					svc.MaxConcurrent, _ = strconv.Atoi(v)
				case "current_load":
					svc.CurrentLoad, _ = strconv.Atoi(v)
				case "security":
					svc.Security = v
				case "health_endpoint":
					svc.HealthEndpoint = v
				case "models":
					svc.Models = strings.Split(v, ",")
				case "gpu":
					svc.GPU = v
				case "vram_gb":
					svc.VRAMGb, _ = strconv.Atoi(v)
				}
			}
		}
	}

	if svc.APIBase == "" && (svc.Host == "" || svc.Port == 0) {
		return SaturnService{}, fmt.Errorf("could not resolve service")
	}

	return svc, nil
}
