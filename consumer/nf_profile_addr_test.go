// SPDX-FileCopyrightText: 2026 Balázs Toldi
//
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/omec-project/openapi/v2/models"
	openapiutils "github.com/omec-project/openapi/v2/utils"
	pcfContext "github.com/omec-project/pcf/context"
	"github.com/omec-project/pcf/factory"
	"github.com/omec-project/util/sbiaddr"
)

const (
	profileV4    = "10.0.0.7"
	profileV6    = "fd00::7"
	profileFQDN  = "pcf"
	profileV6URI = "http://[fd00::7]:29507"
)

func newAddrTestContext(registerIPv4, registerIPv6 string, preference sbiaddr.Preference) *pcfContext.PCFContext {
	c := &pcfContext.PCFContext{
		NfId:          "test-pcf",
		UriScheme:     models.URISCHEME_HTTP,
		RegisterIPv4:  registerIPv4,
		RegisterIPv6:  registerIPv6,
		SBIPort:       29507,
		SbiPreference: preference,
		NfService:     make(map[models.ServiceName]models.NFService),
	}
	c.InitNFService([]factory.Service{{ServiceName: string(models.SERVICENAME_NPCF_SMPOLICYCONTROL)}}, "1.0.0")
	return c
}

func TestGetNfProfileAddresses(t *testing.T) {
	cases := []struct {
		name         string
		registerIPv4 string
		registerIPv6 string
		preference   sbiaddr.Preference
		wantFqdn     string
		wantIPv4     []string
		wantIPv6     []string
		wantURI      string
	}{
		{name: "IPv4", registerIPv4: profileV4, wantIPv4: []string{profileV4}, wantURI: "http://10.0.0.7:29507"},
		{
			name: "FQDN goes to fqdn, not to ipv4Addresses", registerIPv4: profileFQDN,
			wantFqdn: profileFQDN, wantURI: "http://pcf:29507",
		},
		{name: "IPv6", registerIPv6: profileV6, wantIPv6: []string{profileV6}, wantURI: profileV6URI},
		{name: "IPv6 in RegisterIPv4", registerIPv4: profileV6, wantIPv6: []string{profileV6}, wantURI: profileV6URI},
		{
			name: "dual stack", registerIPv4: profileV4, registerIPv6: profileV6,
			wantIPv4: []string{profileV4}, wantIPv6: []string{profileV6}, wantURI: "http://10.0.0.7:29507",
		},
		{
			name: "dual stack, IPv6 preferred", registerIPv4: profileV4, registerIPv6: profileV6,
			preference: sbiaddr.PreferIPv6,
			wantIPv4:   []string{profileV4}, wantIPv6: []string{profileV6}, wantURI: profileV6URI,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prof, err := getNfProfile(newAddrTestContext(tc.registerIPv4, tc.registerIPv6, tc.preference),
				NfProfileDynamicConfig{})
			if err != nil {
				t.Fatalf("getNfProfile returned error: %v", err)
			}
			if got := prof.GetFqdn(); got != tc.wantFqdn {
				t.Errorf("profile fqdn = %q, want %q", got, tc.wantFqdn)
			}
			if got := prof.GetIpv4Addresses(); !reflect.DeepEqual(got, tc.wantIPv4) {
				t.Errorf("profile ipv4Addresses = %v, want %v", got, tc.wantIPv4)
			}
			if got := prof.GetIpv6Addresses(); !reflect.DeepEqual(got, tc.wantIPv6) {
				t.Errorf("profile ipv6Addresses = %v, want %v", got, tc.wantIPv6)
			}

			// A consumer using the shared resolver (such as a wired SMF or AMF)
			// reaches the PCF at its apiPrefix, whatever the family.
			raw, err := json.Marshal(prof)
			if err != nil {
				t.Fatalf("marshal profile: %v", err)
			}
			var discovered models.NFProfileDiscovery
			if err := json.Unmarshal(raw, &discovered); err != nil {
				t.Fatalf("unmarshal profile: %v", err)
			}
			got := openapiutils.SearchNFServiceURI(&discovered, models.SERVICENAME_NPCF_SMPOLICYCONTROL,
				models.NFSERVICESTATUS_REGISTERED, tc.preference == sbiaddr.PreferIPv6)
			if got != tc.wantURI {
				t.Errorf("SearchNFServiceURI = %q, want %q", got, tc.wantURI)
			}
		})
	}
}

func TestGetNfProfileRejectsAnEmptyAddress(t *testing.T) {
	for _, register := range []string{"", "not an address"} {
		if _, err := getNfProfile(newAddrTestContext(register, "", sbiaddr.PreferIPv4), NfProfileDynamicConfig{}); err == nil {
			t.Errorf("getNfProfile with RegisterIPv4 %q: got no error", register)
		}
	}
}
