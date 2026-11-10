// SPDX-FileCopyrightText: 2026 Balázs Toldi
//
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/omec-project/openapi/v2/models"
	pcfContext "github.com/omec-project/pcf/context"
)

// TestAmfStatusChangeSubscribeCallbackOnIPv6 checks that a PCF registered
// with an IPv6 address gives the AMF a callback URI it can parse. With the
// former "%s://%s:%d" formatting it was "http://fd00::7:29507/...", which
// url.Parse rejects.
func TestAmfStatusChangeSubscribeCallbackOnIPv6(t *testing.T) {
	self := pcfContext.PCF_Self()
	origV4, origV6, origScheme, origPort := self.RegisterIPv4, self.RegisterIPv6, self.UriScheme, self.SBIPort
	t.Cleanup(func() {
		self.RegisterIPv4, self.RegisterIPv6, self.UriScheme, self.SBIPort = origV4, origV6, origScheme, origPort
		self.AMFStatusSubsData.Delete("1")
	})
	self.RegisterIPv4, self.RegisterIPv6, self.UriScheme, self.SBIPort = "", profileV6, models.URISCHEME_HTTP, 29507

	var got models.SubscriptionDataAmf
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", "http://amf/namf-comm/v1/subscriptions/1")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(got); err != nil {
			t.Errorf("encode response body: %v", err)
		}
	}))
	defer server.Close()

	if _, err := AmfStatusChangeSubscribe(server.URL, nil); err != nil {
		t.Fatalf("AmfStatusChangeSubscribe: %v", err)
	}
	const want = "http://[fd00::7]:29507/npcf-callback/v1/amfstatus"
	if got.AmfStatusUri != want {
		t.Fatalf("amfStatusUri = %q, want %q", got.AmfStatusUri, want)
	}
	u, err := url.Parse(got.AmfStatusUri)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", got.AmfStatusUri, err)
	}
	if u.Hostname() != profileV6 || u.Port() != "29507" {
		t.Fatalf("amfStatusUri host %q port %q, want %s and 29507", u.Hostname(), u.Port(), profileV6)
	}
}
