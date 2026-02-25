package nodeinfo

import (
	"fmt"
	"strings"

	"meoe.io/poly-csi/pkg/node/mounts"
	"meoe.io/poly-csi/pkg/utils/encrypt"
)

type NodeInfo struct {
	Id           string                            `json:"id"`
	PublicKey    *encrypt.RsaPublicKey             `json:"public_key"`
	HasMultipath bool                              `json:"has_multipath"`
	Mounters     map[string]mounts.MounterNodeInfo `json:"mounters"`
	Ips          []string                          `json:"ips"`
}

func (ni *NodeInfo) Validate() error {
	if ni.Id == "" {
		return fmt.Errorf("node id is empty")
	}
	if ni.PublicKey == nil {
		return fmt.Errorf("public key is not set")
	}
	return nil
}

func (ni *NodeInfo) Serialize(remotePubKey encrypt.RsaPublicKey, aesSecret []byte) (map[string]string, error) {
	result := map[string]string{
		"id":              ni.Id,
		"node_public_key": string(ni.PublicKey.SerializePem()),
		"has_multipath":   "0",
		"ips":             strings.Join(ni.Ips, " "),
	}
	if ni.HasMultipath {
		result["has_multipath"] = "1"
	}
	for k, v := range ni.Mounters {
		serialized, err := v.Serialize(aesSecret)
		if err != nil {
			return nil, fmt.Errorf("mounter %s: serialize failed: %w", k, err)
		}
		key := "mounter." + k
		result[key] = serialized
	}

	encrypt.InjectAesSecret(result, remotePubKey, aesSecret)

	return result, nil
}

func ParseNodeInfo(src map[string]string, privateKey encrypt.RsaPrivateKey) (*NodeInfo, error) {
	aesSecret, err := encrypt.ExtractAesSecret(src, privateKey)
	if err != nil {
		return nil, fmt.Errorf("extract aes secret: %w", err)
	}

	result := NodeInfo{
		Mounters: map[string]mounts.MounterNodeInfo{},
	}
	for k, v := range src {
		switch k {
		case "id":
			result.Id = v
			continue
		case "node_public_key":
			pk, err := encrypt.ParsePublicRsaPem([]byte(v))
			if err != nil {
				return nil, fmt.Errorf("parse node public key: %w", err)
			}
			result.PublicKey = pk
			continue
		case "has_multipath":
			if v == "1" {
				result.HasMultipath = true
			}
			continue
		case "ips":
			result.Ips = strings.Fields(v)
		}
		mounterId, foundMounter := strings.CutPrefix(k, "mounter.")
		if foundMounter {
			mountInfo, err := mounts.DeserializeMounterNodeInfo(v, aesSecret)
			if err != nil {
				return nil, fmt.Errorf("deserialize mounter %s info: %w", mounterId, err)
			}
			result.Mounters[mounterId] = mountInfo
			continue
		}
		// TODO add logs for ignored fields?
	}
	return &result, nil
}
