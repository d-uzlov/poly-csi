package dispatcher

import (
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

type DispatchParams struct {
	// TODO use ?
	OnFailPolicy      string `map:"on_fail_policy"`
	StorageType       string `map:"storageType"`
	StorageIdTemplate string `map:"storageIdTemplate"`
	ShareType         string `map:"shareType"`
	ShareIdTemplate   string `map:"shareIdTemplate"`
	ConnectorId       string `map:"connectorId"`
	NodePreset        string `map:"nodePreset"`
	AddressNames      string `map:"addressNames"`
	ServerName        string `map:"serverName"`
	FsGroupPolicy     string `map:"fsGroupPolicy"`
	FsFormatSide      string `map:"fsFormatSide"`
	JournalVolume     string `map:"journalVolume"`
}

const (
	FsFormatController = "controller"
	FsFormatNodeNode   = "node"
)

const paramsPrefix = "dispatch."
const paramsJournalPrefix = "journal."

const (
	keyStorageType       = "dispatch.storageType"
	keyStorageIdTemplate = "dispatch.storageIdTemplate"
	keyShareType         = "dispatch.shareType"
	keyShareIdTemplate   = "dispatch.shareIdTemplate"
	keyConnectorId       = "dispatch.connectorId"
	keyAddressNames      = "dispatch.addressNames"
	keyServerName        = "dispatch.serverName"
)

func parseDispatchParams(prefix string, global map[string]string, static map[string]string, mutable map[string]string) (DispatchParams, error) {
	for k := range global {
		if !strings.HasPrefix(k, paramsPrefix) {
			continue
		}
		return DispatchParams{}, fmt.Errorf("global parameters: dispatch is not allowed in global parameters")
	}
	for k := range mutable {
		if !strings.HasPrefix(k, paramsPrefix) {
			continue
		}
		return DispatchParams{}, fmt.Errorf("mutable parameters: dispatch is not allowed in mutable parameters")
	}

	result := DispatchParams{}
	err := serialmaps.ParseRemove(static, prefix, true, &result)
	if err != nil {
		return DispatchParams{}, fmt.Errorf("static params: %w", err)
	}
	for k := range static {
		if strings.HasPrefix(k, paramsPrefix) {
			return DispatchParams{}, fmt.Errorf("unknown key: %s", k)
		}
	}

	if result.StorageType == "" {
		return DispatchParams{}, fmt.Errorf("missing %s", keyStorageType)
	}
	if result.ShareType == "" {
		return DispatchParams{}, fmt.Errorf("missing %s", keyShareType)
	}
	if result.ConnectorId == "" {
		return DispatchParams{}, fmt.Errorf("missing %s", keyConnectorId)
	}
	if result.AddressNames == "" {
		return DispatchParams{}, fmt.Errorf("missing %s", keyAddressNames)
	}
	if result.ServerName == "" {
		return DispatchParams{}, fmt.Errorf("missing %s", keyServerName)
	}
	if result.OnFailPolicy == "" {
		result.OnFailPolicy = "cleanup"
	}
	if result.FsFormatSide == "" {
		result.FsFormatSide = FsFormatController
	}
	if result.JournalVolume == "" {
		result.JournalVolume = "none"
	}

	return result, nil
}
