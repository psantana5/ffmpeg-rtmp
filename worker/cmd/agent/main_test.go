package main

import (
	"testing"
)

func TestIsLocalhostURL(t *testing.T) {
	tests := []struct {
		url      string
		expected bool
		desc     string
	}{
		{"https://localhost:8080", true, "localhost with port"},
		{"https://localhost", true, "localhost without port"},
		{"https://127.0.0.1:8080", true, "IPv4 localhost with port"},
		{"https://127.0.0.1", true, "IPv4 localhost without port"},
		{"https://[::1]:8080", true, "IPv6 localhost with port"},
		{"https://[::1]", true, "IPv6 localhost without port"},
		{"http://localhost:8080", true, "HTTP localhost"},
		{"https://malicious-localhost.com", false, "malicious localhost variant"},
		{"https://evil.com/127.0.0.1/path", false, "localhost in path"},
		{"https://localhost.evil.com", false, "localhost as subdomain"},
		{"https://example.com", false, "normal domain"},
		{"https://192.168.1.1", false, "private IP"},
		{"invalid-url", false, "invalid URL format"},
		{"", false, "empty URL"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result := isLocalhostURL(tt.url)
			if result != tt.expected {
				t.Errorf("isLocalhostURL(%q) = %v, expected %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestIsMasterAsWorker(t *testing.T) {
	hostname := "myhost"

	tests := []struct {
		masterURL string
		hostname  string
		expected  bool
		desc      string
	}{
		{"https://localhost:8080", hostname, true, "localhost URL"},
		{"https://127.0.0.1:8080", hostname, true, "localhost IP"},
		{"https://[::1]:8080", hostname, true, "localhost IPv6"},
		{"https://myhost:8080", hostname, true, "matching hostname"},
		{"https://example.com:8080", hostname, false, "different hostname"},
		{"https://myhost.example.com:8080", hostname, false, "hostname as subdomain"},
		{"https://192.168.1.1:8080", hostname, false, "different IP"},
		{"invalid-url", hostname, false, "invalid URL"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result := isMasterAsWorker(tt.masterURL, tt.hostname)
			if result != tt.expected {
				t.Errorf("isMasterAsWorker(%q, %q) = %v, expected %v", 
					tt.masterURL, tt.hostname, result, tt.expected)
			}
		})
	}
}
