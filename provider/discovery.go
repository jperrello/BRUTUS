package provider

import (
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

// SaturnService represents a discovered Saturn AI service.
type SaturnService struct {
	Name         string
	Host         string
	Port         int
	Priority     int    // Lower = preferred
	APIType      string // "openai", "anthropic", etc.
	EphemeralKey string // Beacon-provided credential (if any)
	Features     []string
}

// URL returns the base URL for this service.
func (s SaturnService) URL() string {
	return fmt.Sprintf("http://%s:%d", s.Host, s.Port)
}

// DiscoverSaturn finds Saturn services on the local network using dns-sd.
// Returns services sorted by priority (lowest first).
func DiscoverSaturn(ctx context.Context, timeout time.Duration) ([]SaturnService, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Use dns-sd to browse for Saturn services
	// Service type: _saturn._tcp.local.
	cmd := exec.CommandContext(ctx, "dns-sd", "-B", "_saturn._tcp", "local.")

	output, err := cmd.Output()
	if err != nil {
		// dns-sd not available or no services found
		return nil, fmt.Errorf("discovery failed: %w", err)
	}

	// Parse instance names from browse output
	instances := parseBrowseOutput(string(output))
	if len(instances) == 0 {
		return nil, fmt.Errorf("no Saturn services found")
	}

	// Resolve each instance
	var services []SaturnService
	for _, instance := range instances {
		svc, err := resolveInstance(ctx, instance)
		if err != nil {
			continue // Skip failed resolutions
		}
		services = append(services, svc)
	}

	// Sort by priority (lower = better)
	sort.Slice(services, func(i, j int) bool {
		return services[i].Priority < services[j].Priority
	})

	return services, nil
}

// parseBrowseOutput extracts service instance names from dns-sd browse output.
func parseBrowseOutput(output string) []string {
	var instances []string
	lines := strings.Split(output, "\n")

	// dns-sd -B output format:
	// Browsing for _saturn._tcp.local
	// DATE: ---
	// Add ... "_saturn._tcp." "local." "My Saturn Server"

	for _, line := range lines {
		if strings.Contains(line, "_saturn._tcp.") {
			// Extract the instance name (last quoted string)
			parts := strings.Split(line, "\"")
			if len(parts) >= 2 {
				instances = append(instances, parts[len(parts)-2])
			}
		}
	}

	return instances
}

// resolveInstance gets full details for a Saturn service instance.
func resolveInstance(ctx context.Context, instance string) (SaturnService, error) {
	// Use dns-sd to resolve the instance
	cmd := exec.CommandContext(ctx, "dns-sd", "-L", instance, "_saturn._tcp", "local.")

	output, err := cmd.Output()
	if err != nil {
		return SaturnService{}, err
	}

	return parseResolveOutput(instance, string(output))
}

// parseResolveOutput extracts service details from dns-sd resolve output.
func parseResolveOutput(instance, output string) (SaturnService, error) {
	svc := SaturnService{
		Name:     instance,
		Priority: 100, // Default priority
		APIType:  "openai",
	}

	lines := strings.Split(output, "\n")

	for _, line := range lines {
		// Parse host and port
		// Format: hostname.local.:PORT
		if hostPortRe := regexp.MustCompile(`(\S+\.local\.):(\d+)`); hostPortRe.MatchString(line) {
			matches := hostPortRe.FindStringSubmatch(line)
			if len(matches) >= 3 {
				svc.Host = resolveHostname(matches[1])
				svc.Port, _ = strconv.Atoi(matches[2])
			}
		}

		// Parse TXT records
		// Format: key=value
		if strings.Contains(line, "=") {
			pairs := parseTXTRecords(line)
			for k, v := range pairs {
				switch k {
				case "priority":
					svc.Priority, _ = strconv.Atoi(v)
				case "api":
					svc.APIType = v
				case "ephemeral_key":
					svc.EphemeralKey = v
				case "features":
					svc.Features = strings.Split(v, ",")
				}
			}
		}
	}

	if svc.Host == "" || svc.Port == 0 {
		return SaturnService{}, fmt.Errorf("could not resolve service")
	}

	return svc, nil
}

// parseTXTRecords extracts key=value pairs from a line.
func parseTXTRecords(line string) map[string]string {
	result := make(map[string]string)
	// TXT records are space-separated key=value pairs
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

// resolveHostname converts a .local hostname to an IP address.
func resolveHostname(hostname string) string {
	// Try to resolve the hostname
	addrs, err := net.LookupHost(hostname)
	if err != nil || len(addrs) == 0 {
		// Return hostname without .local. suffix as fallback
		return strings.TrimSuffix(hostname, ".local.")
	}

	// Prefer non-loopback addresses
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip != nil && !ip.IsLoopback() && ip.To4() != nil {
			return addr
		}
	}

	return addrs[0]
}
