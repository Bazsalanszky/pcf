// SPDX-FileCopyrightText: 2026 Balázs Toldi
//
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/omec-project/openapi/v2/models"
	"github.com/omec-project/pcf/factory"
	"github.com/omec-project/util/sbiaddr"
)

const (
	testV4      = "10.0.0.7"
	testV6      = "fd00::7"
	testFQDN    = "pcf"
	testV4URI   = "http://10.0.0.7:29507"
	testV6URI   = "http://[fd00::7]:29507"
	testFQDNURI = "http://pcf:29507"
)

func TestGetSbiUri(t *testing.T) {
	cases := []struct {
		name         string
		registerIPv4 string
		registerIPv6 string
		preference   sbiaddr.Preference
		wantURI      string
	}{
		{"IPv4", testV4, "", sbiaddr.PreferIPv4, testV4URI},
		{"FQDN", testFQDN, "", sbiaddr.PreferIPv4, testFQDNURI},
		{"IPv6 only", "", testV6, sbiaddr.PreferIPv4, testV6URI},
		{"IPv6 in RegisterIPv4", testV6, "", sbiaddr.PreferIPv4, testV6URI},
		{"dual, IPv4 preferred", testV4, testV6, sbiaddr.PreferIPv4, testV4URI},
		{"dual, IPv6 preferred", testV4, testV6, sbiaddr.PreferIPv6, testV6URI},
		{"FQDN and IPv6, IPv6 preferred", testFQDN, testV6, sbiaddr.PreferIPv6, testFQDNURI},
		{"nothing", "", "", sbiaddr.PreferIPv4, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &PCFContext{
				UriScheme:     models.URISCHEME_HTTP,
				RegisterIPv4:  tc.registerIPv4,
				RegisterIPv6:  tc.registerIPv6,
				SBIPort:       29507,
				SbiPreference: tc.preference,
			}
			if got := c.GetSbiUri(); got != tc.wantURI {
				t.Fatalf("GetSbiUri() = %q, want %q", got, tc.wantURI)
			}
			if got := c.GetIPv4Uri(); got != tc.wantURI {
				t.Fatalf("GetIPv4Uri() = %q, want %q", got, tc.wantURI)
			}
			if tc.wantURI == "" {
				return
			}
			// The callback URIs (nf-status-notify, amfstatus) are built from it.
			callback := c.GetIPv4Uri() + "/npcf-callback/v1/nf-status-notify"
			if _, err := url.Parse(callback); err != nil {
				t.Fatalf("url.Parse(%q): %v", callback, err)
			}
		})
	}
}

// TestGetIPv4UriKeepsLegacyFormat checks that IPv4 and host-name
// configurations give the same URI as the former "%s://%s:%d" formatting.
func TestGetIPv4UriKeepsLegacyFormat(t *testing.T) {
	for _, host := range []string{factory.PCF_DEFAULT_IPV4, testV4, testFQDN, "pcf.omec.svc.cluster.local"} {
		for _, scheme := range []models.UriScheme{models.URISCHEME_HTTP, models.URISCHEME_HTTPS} {
			for _, port := range []int{8000, 29507} {
				c := &PCFContext{UriScheme: scheme, RegisterIPv4: host, SBIPort: port}
				if got, want := c.GetIPv4Uri(), fmt.Sprintf("%s://%s:%d", scheme, host, port); got != want {
					t.Errorf("GetIPv4Uri() = %q, want %q", got, want)
				}
			}
		}
	}
}

type wantEndPoint struct {
	ipv4, ipv6 string
}

func TestInitNFServiceAddresses(t *testing.T) {
	cases := []struct {
		name         string
		registerIPv4 string
		registerIPv6 string
		wantFqdn     string
		wantPrefix   string
		wantEPs      []wantEndPoint
	}{
		{"IPv4", testV4, "", "", testV4URI, []wantEndPoint{{ipv4: testV4}}},
		{"FQDN goes to fqdn, not to an IPv4 field", testFQDN, "", testFQDN, testFQDNURI, []wantEndPoint{{}}},
		{"IPv6", "", testV6, "", testV6URI, []wantEndPoint{{ipv6: testV6}}},
		{"dual stack: two endpoints, IPv4 first", testV4, testV6, "", testV4URI, []wantEndPoint{{ipv4: testV4}, {ipv6: testV6}}},
		{"FQDN and IPv6", testFQDN, testV6, testFQDN, testFQDNURI, []wantEndPoint{{ipv6: testV6}}},
	}
	services := []factory.Service{
		{ServiceName: string(models.SERVICENAME_NPCF_SMPOLICYCONTROL)},
		{ServiceName: string(models.SERVICENAME_NPCF_AM_POLICY_CONTROL)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &PCFContext{
				UriScheme:    models.URISCHEME_HTTP,
				RegisterIPv4: tc.registerIPv4,
				RegisterIPv6: tc.registerIPv6,
				SBIPort:      29507,
				NfService:    make(map[models.ServiceName]models.NFService),
			}
			c.InitNFService(services, "1.0.0")
			if len(c.NfService) != len(services) {
				t.Fatalf("got %d services, want %d", len(c.NfService), len(services))
			}
			for name, svc := range c.NfService {
				if got := svc.GetFqdn(); got != tc.wantFqdn {
					t.Errorf("%s: fqdn = %q, want %q", name, got, tc.wantFqdn)
				}
				if got := svc.GetApiPrefix(); got != tc.wantPrefix {
					t.Errorf("%s: apiPrefix = %q, want %q", name, got, tc.wantPrefix)
				}
				endPoints := svc.GetIpEndPoints()
				if len(endPoints) != len(tc.wantEPs) {
					t.Fatalf("%s: got %d ipEndPoints, want %d: %+v", name, len(endPoints), len(tc.wantEPs), endPoints)
				}
				for i, ep := range endPoints {
					if ep.GetIpv4Address() != tc.wantEPs[i].ipv4 || ep.GetIpv6Address() != tc.wantEPs[i].ipv6 ||
						ep.GetPort() != 29507 || ep.GetTransport() != models.TRANSPORTPROTOCOL_TCP {
						t.Errorf("%s: ipEndPoints[%d] = %+v, want v4 %q v6 %q port 29507 TCP",
							name, i, ep, tc.wantEPs[i].ipv4, tc.wantEPs[i].ipv6)
					}
				}
			}
		})
	}
}
