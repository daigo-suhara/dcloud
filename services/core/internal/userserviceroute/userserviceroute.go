package userserviceroute

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

func UserServiceNameFromHost(r *http.Request, publicServiceDomain string) string {
	domain := strings.TrimSpace(publicServiceDomain)
	if domain == "" {
		return ""
	}

	host := requestHost(r)
	if host == "" {
		return ""
	}
	host = strings.ToLower(host)
	domain = strings.ToLower(strings.TrimSuffix(domain, "."))
	if host == domain {
		return ""
	}
	if !strings.HasSuffix(host, "."+domain) {
		return ""
	}

	name := strings.TrimSuffix(host, "."+domain)
	if name == "" || strings.Contains(name, ".") || !isDNSLabel(name) {
		return ""
	}
	return name
}

func UserServiceURL(baseURL string, publicServiceDomain string, name string) string {
	domain := strings.TrimSpace(publicServiceDomain)
	if domain != "" {
		return fmt.Sprintf("https://%s.%s/", name, strings.TrimSuffix(domain, "."))
	}
	return fmt.Sprintf("%s/container-apps/%s/", baseURL, name)
}

func requestHost(r *http.Request) string {
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(r.Host)
	}
	if host == "" {
		return ""
	}
	if strings.Contains(host, ":") {
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
	}
	return strings.TrimSpace(host)
}

func isDNSLabel(s string) bool {
	if s == "" || len(s) > 63 {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-':
			if i == 0 || i == len(s)-1 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
