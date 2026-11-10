// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
//

package util

import (
	"github.com/omec-project/openapi/v2/models"
	openapiutils "github.com/omec-project/openapi/v2/utils"
	pcfContext "github.com/omec-project/pcf/context"
	"github.com/omec-project/util/sbiaddr"
)

// SearchNFServiceUri returns the base URI (scheme://host:port, an IPv6 host in
// brackets, or the service's apiPrefix) of the first service of nfProfile with
// the given name and status, or "" if there is none. It reads nfServiceList,
// sorted by key, before the deprecated nfServices, and picks the address
// family by the PCF's preferredFamily.
func SearchNFServiceUri(nfProfile models.NFProfileDiscovery, serviceName models.ServiceName,
	nfServiceStatus models.NFServiceStatus,
) string {
	return openapiutils.SearchNFServiceURI(&nfProfile, serviceName, nfServiceStatus,
		pcfContext.PCF_Self().SbiPreference == sbiaddr.PreferIPv6)
}
