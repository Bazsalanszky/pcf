// SPDX-FileCopyrightText: 2026 Balázs Toldi
//
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"fmt"
	"net/url"
	"sort"
	"testing"

	"github.com/omec-project/openapi/v2/models"
	pcfContext "github.com/omec-project/pcf/context"
	"github.com/omec-project/util/sbiaddr"
)

const (
	peerV4    = "10.0.0.4"
	peerV6    = "fd00::4"
	peerV6URI = "http://[fd00::4]:29504"
)

func udrService(scheme models.UriScheme, endPoints ...models.IpEndPoint) models.NFService {
	return models.NFService{
		ServiceInstanceId: "datarepository",
		ServiceName:       models.SERVICENAME_NUDR_DR,
		NfServiceStatus:   models.NFSERVICESTATUS_REGISTERED,
		Scheme:            scheme,
		IpEndPoints:       endPoints,
	}
}

func v4EndPoint(addr string, port int32) models.IpEndPoint {
	return models.IpEndPoint{Ipv4Address: new(addr), Port: new(port)}
}

func v6EndPoint(addr string, port int32) models.IpEndPoint {
	return models.IpEndPoint{Ipv6Address: new(addr), Port: new(port)}
}

func setPreference(t *testing.T, p sbiaddr.Preference) {
	t.Helper()
	self := pcfContext.PCF_Self()
	orig := self.SbiPreference
	self.SbiPreference = p
	t.Cleanup(func() { self.SbiPreference = orig })
}

func TestSearchNFServiceUri_DualStack(t *testing.T) {
	withFqdn := func(svc models.NFService, fqdn string) models.NFService {
		svc.SetFqdn(fqdn)
		return svc
	}
	dual := udrService(models.URISCHEME_HTTP, v4EndPoint(peerV4, 29504), v6EndPoint(peerV6, 29504))
	cases := []struct {
		name       string
		profile    models.NFProfileDiscovery
		preference sbiaddr.Preference
		want       string
	}{
		{
			// The removed PCF copy returned the bare "udr" here.
			name: "profile fqdn gives a URI, not a bare name",
			profile: models.NFProfileDiscovery{
				Fqdn:       new("udr"),
				NfServices: []models.NFService{udrService(models.URISCHEME_HTTP, models.IpEndPoint{Port: new(int32(29504))})},
			},
			want: "http://udr:29504",
		},
		{
			name: "service fqdn without endpoint uses the scheme's default port",
			profile: models.NFProfileDiscovery{
				NfServices: []models.NFService{withFqdn(udrService(models.URISCHEME_HTTPS), "udr.omec.svc")},
			},
			want: "https://udr.omec.svc:443",
		},
		{
			name: "IPv6-only endpoint is bracketed",
			profile: models.NFProfileDiscovery{
				NfServices: []models.NFService{udrService(models.URISCHEME_HTTP, v6EndPoint(peerV6, 29504))},
			},
			want: peerV6URI,
		},
		{
			name: "IPv6-only profile address",
			profile: models.NFProfileDiscovery{
				Ipv6Addresses: []string{peerV6},
				NfServices:    []models.NFService{udrService(models.URISCHEME_HTTP, models.IpEndPoint{Port: new(int32(29504))})},
			},
			want: peerV6URI,
		},
		{
			name:    "dual stack, IPv4 preferred (default)",
			profile: models.NFProfileDiscovery{NfServices: []models.NFService{dual}},
			want:    "http://10.0.0.4:29504",
		},
		{
			name:       "dual stack, preferredFamily IPv6",
			profile:    models.NFProfileDiscovery{NfServices: []models.NFService{dual}},
			preference: sbiaddr.PreferIPv6,
			want:       peerV6URI,
		},
		{
			name: "apiPrefix is returned as it is",
			profile: models.NFProfileDiscovery{NfServices: []models.NFService{func() models.NFService {
				svc := dual
				svc.SetApiPrefix(peerV6URI)
				return svc
			}()}},
			preference: sbiaddr.PreferIPv4,
			want:       peerV6URI,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setPreference(t, tc.preference)
			got := SearchNFServiceUri(tc.profile, models.SERVICENAME_NUDR_DR, models.NFSERVICESTATUS_REGISTERED)
			if got != tc.want {
				t.Fatalf("SearchNFServiceUri() = %q, want %q", got, tc.want)
			}
			if _, err := url.Parse(got); err != nil {
				t.Fatalf("url.Parse(%q): %v", got, err)
			}
		})
	}
}

// TestSearchNFServiceUriKeepsLegacyResults compares the shared resolver with
// a verbatim copy of the PCF resolver it replaces, for the profiles the old
// copy handled: IPv4 endpoints and profile addresses, apiPrefix, with the
// http or https scheme and with or without a port. Profiles with an fqdn
// (bare name before, a URI now), an empty or unknown scheme and IPv6 are
// covered by TestSearchNFServiceUri_DualStack instead.
func TestSearchNFServiceUriKeepsLegacyResults(t *testing.T) {
	var profiles []models.NFProfileDiscovery
	for _, scheme := range []models.UriScheme{models.URISCHEME_HTTP, models.URISCHEME_HTTPS} {
		for _, port := range []int32{0, 8000, 29504} {
			for _, apiPrefix := range []string{"", "http://10.0.0.9:29504"} {
				for _, profileV4 := range [][]string{nil, {"10.0.0.8"}} {
					for _, endPoint := range []models.IpEndPoint{v4EndPoint(peerV4, port), {Port: new(port)}} {
						svc := udrService(scheme, endPoint)
						if apiPrefix != "" {
							svc.SetApiPrefix(apiPrefix)
						}
						profiles = append(profiles,
							models.NFProfileDiscovery{Ipv4Addresses: profileV4, NfServices: []models.NFService{svc}},
							models.NFProfileDiscovery{
								Ipv4Addresses: profileV4,
								NfServiceList: &map[string]models.NFService{"b": svc, "a": udrService(scheme)},
							})
					}
				}
			}
		}
	}
	for _, preference := range []sbiaddr.Preference{sbiaddr.PreferIPv4, sbiaddr.PreferIPv6} {
		setPreference(t, preference)
		for i, profile := range profiles {
			want := legacySearchNFServiceUri(profile, models.SERVICENAME_NUDR_DR, models.NFSERVICESTATUS_REGISTERED)
			got := SearchNFServiceUri(profile, models.SERVICENAME_NUDR_DR, models.NFSERVICESTATUS_REGISTERED)
			if got != want {
				t.Errorf("profile %d (%v): SearchNFServiceUri() = %q, legacy = %q", i, preference, got, want)
			}
		}
	}
	if len(profiles) != 96 {
		t.Fatalf("built %d profiles, want 96", len(profiles))
	}
}

// legacySearchNFServiceUri, legacyNfProfileServices and legacyGetSbiUri are
// the PCF's util/search_nf_service.go before this change, unchanged apart
// from the names.
func legacySearchNFServiceUri(nfProfile models.NFProfileDiscovery, serviceName models.ServiceName,
	nfServiceStatus models.NFServiceStatus,
) (nfUri string) {
	for _, service := range legacyNfProfileServices(nfProfile) {
		if service.GetServiceName() == serviceName && service.GetNfServiceStatus() == nfServiceStatus {
			if nfProfile.GetFqdn() != "" {
				nfUri = nfProfile.GetFqdn()
			} else if service.GetFqdn() != "" {
				nfUri = service.GetFqdn()
			} else if service.GetApiPrefix() != "" {
				nfUri = service.GetApiPrefix()
			} else if len(service.GetIpEndPoints()) > 0 {
				point := service.GetIpEndPoints()[0]
				if point.GetIpv4Address() != "" {
					nfUri = legacyGetSbiUri(service.GetScheme(), point.GetIpv4Address(), point.GetPort())
				} else if len(nfProfile.GetIpv4Addresses()) != 0 {
					nfUri = legacyGetSbiUri(service.GetScheme(), nfProfile.GetIpv4Addresses()[0], point.GetPort())
				}
			}
		}
		if nfUri != "" {
			break
		}
	}

	return
}

func legacyNfProfileServices(nfProfile models.NFProfileDiscovery) []models.NFService {
	nfServiceList := nfProfile.GetNfServiceList()
	if len(nfServiceList) == 0 {
		return nfProfile.GetNfServices()
	}
	instanceIds := make([]string, 0, len(nfServiceList))
	for instanceId := range nfServiceList {
		instanceIds = append(instanceIds, instanceId)
	}
	sort.Strings(instanceIds)
	services := make([]models.NFService, 0, len(nfServiceList))
	for _, instanceId := range instanceIds {
		services = append(services, nfServiceList[instanceId])
	}
	return services
}

func legacyGetSbiUri(scheme models.UriScheme, ipv4Address string, port int32) (uri string) {
	if port != 0 {
		uri = fmt.Sprintf("%s://%s:%d", scheme, ipv4Address, port)
	} else {
		switch scheme {
		case models.URISCHEME_HTTP:
			uri = fmt.Sprintf("%s://%s:80", scheme, ipv4Address)
		case models.URISCHEME_HTTPS:
			uri = fmt.Sprintf("%s://%s:443", scheme, ipv4Address)
		}
	}
	return
}
