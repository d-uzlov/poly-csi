package blockapi

import (
	"encoding/json"
	"fmt"
)

type Data interface {
	Validate() error
	Clone() Data
}

type Name string

type NodeInfo Data

func ParseNodeInfo(name Name, data json.RawMessage, out *NodeInfo) error {
	// TODO remove this function when/if iscsi does not need node info anymore
	if name != Iscsi {
		return fmt.Errorf("type %s does not have node info, but trying to parse '%s'", name, string(data))
	}
	var result IscsiNodeInfo

	err := json.Unmarshal([]byte(data), &result)
	if err != nil {
		return err
	}
	*out = result
	return nil
}
