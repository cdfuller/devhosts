package state

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	hostPattern       = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)
	localhostPrefixes = []string{"http://localhost:", "http://127.0.0.1:"}
)

// Host describes a single managed hostname and its upstream target.
type Host struct {
	Name     string `json:"name"`
	Upstream string `json:"upstream"`
	TLS      bool   `json:"tls,omitempty"`
}

// Snapshot represents the desired configuration state persisted to disk.
type Snapshot struct {
	Version          int    `json:"version"`
	Hosts            []Host `json:"hosts"`
	BaseCaddyfile    string `json:"base_caddyfile"`
	IncludeCaddyfile string `json:"include_caddyfile"`
}

// NormalizeHostName trims and lowercases a hostname.
func NormalizeHostName(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// ValidateSnapshot ensures the snapshot adheres to product rules.
func ValidateSnapshot(s Snapshot) error {
	if s.Version != 1 {
		return fmt.Errorf("unsupported config version %d", s.Version)
	}
	if s.BaseCaddyfile == "" {
		return errors.New("base_caddyfile must be set")
	}
	if s.IncludeCaddyfile == "" {
		return errors.New("include_caddyfile must be set")
	}

	names := make(map[string]struct{}, len(s.Hosts))
	for i := range s.Hosts {
		h := &s.Hosts[i]
		h.Name = NormalizeHostName(h.Name)
		if err := validateHost(*h); err != nil {
			return fmt.Errorf("host %q invalid: %w", h.Name, err)
		}
		if _, exists := names[h.Name]; exists {
			return fmt.Errorf("duplicate host name %q", h.Name)
		}
		names[h.Name] = struct{}{}
	}
	sort.Slice(s.Hosts, func(i, j int) bool { return s.Hosts[i].Name < s.Hosts[j].Name })
	return nil
}

func validateHost(h Host) error {
	if h.Name == "" {
		return errors.New("name required")
	}
	if strings.Contains(h.Name, ".") {
		return errors.New("hostname must be bare (no dots)")
	}
	if !hostPattern.MatchString(h.Name) {
		return errors.New("hostname must match [a-z0-9-]+ and start/end alphanumeric")
	}
	if err := validateUpstream(h.Upstream); err != nil {
		return fmt.Errorf("upstream %q invalid: %w", h.Upstream, err)
	}
	return nil
}

func validateUpstream(raw string) error {
	if raw == "" {
		return errors.New("upstream required")
	}
	allowed := false
	for _, prefix := range localhostPrefixes {
		if strings.HasPrefix(strings.ToLower(raw), prefix) {
			allowed = true
			break
		}
	}
	if !allowed {
		return errors.New("must target http://localhost:<port> or http://127.0.0.1:<port>")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	host := strings.ToLower(u.Hostname())
	if host != "localhost" && host != "127.0.0.1" {
		return errors.New("hostname must be localhost or 127.0.0.1")
	}
	if u.Scheme != "http" {
		return errors.New("scheme must be http")
	}
	if u.Path != "" && u.Path != "/" {
		return errors.New("path segments are not supported")
	}
	port := u.Port()
	if port == "" {
		return errors.New("port required")
	}
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("invalid port: %w", err)
	}
	if portNum < 1 || portNum > 65535 {
		return fmt.Errorf("port out of range: %d", portNum)
	}
	return nil
}
