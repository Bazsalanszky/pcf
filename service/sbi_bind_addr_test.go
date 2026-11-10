// SPDX-FileCopyrightText: 2026 Balázs Toldi
//
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
)

func TestSbiBindAddr(t *testing.T) {
	const port = 29507
	tests := []struct {
		name string
		host string
		want string
	}{
		{name: "IPv4 wildcard", host: "0.0.0.0", want: "0.0.0.0:29507"},
		{name: "IPv4 address", host: "192.0.2.10", want: "192.0.2.10:29507"},
		{name: "hostname", host: "pcf.sdcore.svc.cluster.local", want: "pcf.sdcore.svc.cluster.local:29507"},
		{name: "empty host", host: "", want: ":29507"},
		{name: "IPv6 wildcard", host: "::", want: "[::]:29507"},
		{name: "IPv6 address", host: "2001:db8::10", want: "[2001:db8::10]:29507"},
		{name: "IPv6 link-local with zone", host: "fe80::1%eth0", want: "[fe80::1%eth0]:29507"},
		{name: "already bracketed IPv6", host: "[2001:db8::10]", want: "[2001:db8::10]:29507"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sbiBindAddr(tc.host, port)
			if got != tc.want {
				t.Fatalf("sbiBindAddr(%q, %d) = %q, want %q", tc.host, port, got, tc.want)
			}
			// The result must split back into the unbracketed host and the port.
			wantHost := strings.TrimSuffix(strings.TrimPrefix(tc.host, "["), "]")
			host, gotPort, err := net.SplitHostPort(got)
			if err != nil {
				t.Fatalf("net.SplitHostPort(%q) failed: %v", got, err)
			}
			if host != wantHost || gotPort != strconv.Itoa(port) {
				t.Fatalf("net.SplitHostPort(%q) = (%q, %q), want (%q, %q)", got, host, gotPort, wantHost, strconv.Itoa(port))
			}
		})
	}
}

// TestSbiBindAddrKeepsLegacyFormat checks that every non-IPv6 host gives exactly
// the string the previous fmt.Sprintf("%s:%d") produced.
func TestSbiBindAddrKeepsLegacyFormat(t *testing.T) {
	for _, host := range []string{"0.0.0.0", "127.0.0.1", "10.0.0.1", "", "localhost", "pcf"} {
		for _, port := range []int{29507, 80, 0} {
			if got, legacy := sbiBindAddr(host, port), fmt.Sprintf("%s:%d", host, port); got != legacy {
				t.Errorf("sbiBindAddr(%q, %d) = %q, want legacy %q", host, port, got, legacy)
			}
		}
	}
}
