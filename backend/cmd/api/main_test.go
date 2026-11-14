package main

import "testing"

func TestListenAddress(t *testing.T) {
	for _, tc := range []struct {
		port string
		want string
	}{
		{"", "127.0.0.1:8080"},
		{"3000", ":3000"},
		{"65535", ":65535"},
	} {
		got, err := listenAddress("127.0.0.1:8080", tc.port)
		if err != nil || got != tc.want {
			t.Fatalf("listenAddress(%q) = %q, %v; want %q", tc.port, got, err, tc.want)
		}
	}
}

func TestListenAddressRejectsInvalidPlatformPort(t *testing.T) {
	for _, port := range []string{"0", "-1", "65536", "3000/path", "localhost:3000", " 3000", "3000\n", "not-a-port"} {
		if _, err := listenAddress("127.0.0.1:8080", port); err == nil {
			t.Fatalf("accepted invalid PORT %q", port)
		}
	}
}
