package util

import (
	"net"
	"strconv"
	"strings"
	"unicode/utf8"
)

func IsASCIINoSpace(s string) bool {
	if strings.ContainsRune(s, ' ') {
		return false
	}
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}

func IsValidPlayerRemark(s string) bool {
	s = strings.TrimSpace(s)
	return s != "" && utf8.RuneCountInString(s) <= 30
}

func IsValidPlayerKey(s string) bool {
	return len(s) >= 2 && len(s) <= 64 && IsASCIINoSpace(s)
}

func IsValidDomain(domain string) bool {
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		if strings.HasPrefix(part, "-") || strings.HasSuffix(part, "-") {
			return false
		}
		for _, r := range part {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-') {
				return false
			}
		}
	}
	return true
}

func IsValidTunnelSourceAddress(addr string) bool {
	host, _, ok := splitHostPort(addr)
	if !ok {
		return false
	}
	return isValidTunnelHost(host, true)
}

func IsValidTunnelEndpointAddress(addr string) bool {
	host, _, ok := splitHostPort(addr)
	if !ok {
		return false
	}
	return isValidTunnelHost(host, false)
}

func GetTunnelPort(addr string) (uint16, bool) {
	_, port, ok := splitHostPort(addr)
	if !ok {
		return 0, false
	}
	value, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return 0, false
	}
	return uint16(value), true
}

func splitHostPort(addr string) (string, string, bool) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", "", false
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", "", false
	}
	if port == "" {
		return "", "", false
	}
	if _, err := strconv.ParseUint(port, 10, 16); err != nil {
		return "", "", false
	}
	return host, port, true
}

func isValidTunnelHost(host string, allowEmpty bool) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return allowEmpty
	}
	// 中文注释：localhost 和域名在 Go 的监听/拨号侧都可以正常工作，这里直接放行。
	if strings.EqualFold(host, "localhost") || IsValidDomain(host) {
		return true
	}
	return net.ParseIP(host) != nil
}
