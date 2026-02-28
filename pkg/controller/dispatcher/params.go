package dispatcher

import (
	"fmt"
	"strings"
)

type DispatchParams struct {
	onFailPolicy      string // TODO use ?
	storageType       string
	storageIdTemplate string
	shareType         string
	shareIdTemplate   string
	connectorId       string
	addressNames      string
	serverName        string
}

const paramsPrefix = "dispatch."

const (
	keyStorageType       = "dispatch.storageType"
	keyStorageIdTemplate = "dispatch.storageIdTemplate"
	keyShareType         = "dispatch.shareType"
	keyShareIdTemplate   = "dispatch.shareIdTemplate"
	keyConnectorId       = "dispatch.connectorId"
	keyAddressNames      = "dispatch.addressNames"
	keyServerName        = "dispatch.serverName"
)

func parseDispatchParams(global map[string]string, static map[string]string, mutable map[string]string) (DispatchParams, error) {
	result := DispatchParams{}
	for k := range global {
		if !strings.HasPrefix(k, paramsPrefix) {
			continue
		}
		return DispatchParams{}, fmt.Errorf("global parameters: dispatch is not allowed in global parameters")
	}
	for k, v := range static {
		if !strings.HasPrefix(k, paramsPrefix) {
			continue
		}
		switch k {
		case "dispatch.on-fail-policy":
			result.onFailPolicy = v
		case keyStorageType:
			result.storageType = v
		case keyStorageIdTemplate:
			result.storageIdTemplate = v
		case keyShareType:
			result.shareType = v
		case keyShareIdTemplate:
			result.shareIdTemplate = v
		case keyConnectorId:
			result.connectorId = v
		case keyAddressNames:
			result.addressNames = v
		case keyServerName:
			result.serverName = v
		default:
			return DispatchParams{}, fmt.Errorf("static parameters: unknown key '%s'", k)
		}
	}
	for k := range mutable {
		if !strings.HasPrefix(k, paramsPrefix) {
			continue
		}
		return DispatchParams{}, fmt.Errorf("mutable parameters: dispatch is not allowed in mutable parameters")
	}

	if result.storageType == "" {
		return DispatchParams{}, fmt.Errorf("missing %s", keyStorageType)
	}
	if result.shareType == "" {
		return DispatchParams{}, fmt.Errorf("missing %s", keyShareType)
	}
	if result.connectorId == "" {
		return DispatchParams{}, fmt.Errorf("missing %s", keyConnectorId)
	}
	if result.addressNames == "" {
		return DispatchParams{}, fmt.Errorf("missing %s", keyAddressNames)
	}
	if result.serverName == "" {
		return DispatchParams{}, fmt.Errorf("missing %s", keyServerName)
	}
	if result.onFailPolicy == "" {
		result.onFailPolicy = "cleanup"
	}

	return result, nil
}
