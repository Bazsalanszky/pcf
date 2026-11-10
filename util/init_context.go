// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-FileCopyrightText: 2024 Canonical Ltd.
// SPDX-License-Identifier: Apache-2.0
//

package util

import (
	"cmp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/omec-project/openapi/v2/models"
	pcfContext "github.com/omec-project/pcf/context"
	"github.com/omec-project/pcf/factory"
	"github.com/omec-project/pcf/logger"
	"github.com/omec-project/util/sbiaddr"
)

// InitPcfContext Init PCF Context from config file
func InitPcfContext(context *pcfContext.PCFContext) {
	config := factory.PcfConfig
	logger.UtilLog.Infof("pcfconfig Info: Version[%s] Description[%s]", config.Info.Version, config.Info.Description)
	configuration := config.Configuration
	context.NfId = uuid.New().String()
	if configuration.PcfName != "" {
		context.Name = configuration.PcfName
	}

	sbi := configuration.Sbi
	context.NrfUri = configuration.NrfUri
	context.UriScheme = ""
	context.RegisterIPv4 = factory.PCF_DEFAULT_IPV4 // default localhost
	context.RegisterIPv6 = ""
	context.SbiPreference = sbiaddr.PreferIPv4
	context.SBIPort = factory.PCF_DEFAULT_PORT_INT // default port
	if sbi != nil {
		if sbi.Scheme != "" {
			context.UriScheme = models.UriScheme(sbi.Scheme)
		}
		setRegisterAddrs(context, sbi)
		if sbi.Port != 0 {
			context.SBIPort = sbi.Port
		}
		if sbi.Scheme == "https" {
			context.UriScheme = models.URISCHEME_HTTPS
		} else {
			context.UriScheme = models.URISCHEME_HTTP
		}
		if tls := sbi.TLS; tls != nil {
			if tls.Key != "" {
				context.Key = tls.Key
			}
			if tls.PEM != "" {
				context.PEM = tls.PEM
			}
		}

		var err error
		if context.SbiPreference, err = sbiaddr.ParsePreference(sbi.PreferredFamily); err != nil {
			logger.UtilLog.Warnf("SBI preferredFamily: %v; using IPv4", err)
		}
		setBindingAddr(context, sbi)
	}
	context.EnableNrfCaching = configuration.EnableNrfCaching
	if configuration.EnableNrfCaching {
		if configuration.NrfCacheEvictionInterval == 0 {
			context.NrfCacheEvictionInterval = time.Duration(900) // 15 mins
		} else {
			context.NrfCacheEvictionInterval = time.Duration(configuration.NrfCacheEvictionInterval)
		}
	}
	serviceList := configuration.ServiceList
	context.InitNFService(serviceList, config.Info.Version)
	context.TimeFormat = configuration.TimeFormat
	context.DefaultBdtRefId = configuration.DefaultBdtRefId
	for _, service := range context.NfService {
		context.PcfServiceUris[service.ServiceName] = service.GetApiPrefix() + "/" + string(service.ServiceName) + "/" + service.Versions[0].ApiVersionInUri
		pcfSuppFeats, err := pcfContext.NewSupportedFeature(service.GetSupportedFeatures())
		if err != nil {
			logger.UtilLog.Errorf("NewSupportedFeature error: %+v", err)
			pcfSuppFeats = pcfContext.NewEmptySupportedFeature()
		}
		context.PcfSuppFeats[service.ServiceName] = *pcfSuppFeats
	}
}

// setRegisterAddrs resolves registerIPv4 and registerIPv6 (each an address, a
// host name or the name of an environment variable such as POD_IP or POD_IPS)
// into the addresses the PCF registers at the NRF. RegisterIPv4 keeps holding
// the IPv4 address or the host name, as before; an IPv6 address goes to
// RegisterIPv6, whichever key it came from.
func setRegisterAddrs(context *pcfContext.PCFContext, sbi *factory.Sbi) {
	addrs, err := sbiaddr.ResolveRegister(sbi.RegisterIPv4, sbi.RegisterIPv6)
	if err != nil {
		logger.UtilLog.Warnf("SBI register address: %v", err)
	}
	if addrs.Empty() {
		addrs.IPv4 = factory.PCF_DEFAULT_IPV4
	}
	if addrs.FQDN != "" && addrs.IPv4 != "" {
		logger.UtilLog.Warnf("SBI register address: registering FQDN %s, not IPv4 %s", addrs.FQDN, addrs.IPv4)
	}
	context.RegisterIPv4 = cmp.Or(addrs.FQDN, addrs.IPv4)
	context.RegisterIPv6 = addrs.IPv6
}

// setBindingAddr sets the SBI listen host from bindingAddr, or from its legacy
// alias bindingIPv4. Either may name an environment variable; the default is
// 0.0.0.0, which also accepts IPv6 connections.
func setBindingAddr(context *pcfContext.PCFContext, sbi *factory.Sbi) {
	if strings.TrimSpace(sbi.BindingAddr) == "" && strings.TrimSpace(sbi.BindingIPv4) == "" {
		logger.UtilLog.Warnln("SBI binding address is not set. Using the 0.0.0.0 address as default")
	}
	var err error
	if context.BindingIPv4, err = sbiaddr.BindHost(sbi.BindingAddr, sbi.BindingIPv4); err != nil {
		logger.UtilLog.Warnf("SBI binding address: %v", err)
	}
}
