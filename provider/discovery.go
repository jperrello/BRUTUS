package provider

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type SaturnService struct {
	Name         string
	Host         string
	Port         int
	Priority     int
	APIType      string
	EphemeralKey string
	Features     []string
	APIBase      string // Remote API URL (e.g., "https://openrouter.ai/api/v1")
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
				}
			}
		}
	}

	if svc.APIBase == "" && (svc.Host == "" || svc.Port == 0) {
		return SaturnService{}, fmt.Errorf("could not resolve service")
	}

	return svc, nil
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
