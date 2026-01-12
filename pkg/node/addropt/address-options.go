package addropt

import (
	"encoding/json"
	"fmt"
	"strings"
)

type AddressOption struct {
	Name        string              `json:"n"`
	Address     string              `json:"a"`
}

func Inject(src []AddressOption, dst map[string]string) error {
	var keys []string
	for _, v := range src {
		encoded, err := json.Marshal(v)
		if err != nil {
			return err
		}
		key:=fmt.Sprintf("addr_opt.%s", v.Name)
		dst[key] = string(encoded)
		keys = append(keys, key)
	}
	dst["addr_keys"] = strings.Join(keys, " ")
	return nil
}

func Parse(src map[string]string) ([]AddressOption, error) {
	var result []AddressOption
	for k := range strings.FieldsSeq(src["addr_keys"]) {
		encoded := src[k]
		if encoded == "" {
			return nil, fmt.Errorf("missing '%s'", k)
		}
		var opt AddressOption
		err := json.Unmarshal([]byte(encoded), &opt)
		if err != nil {
			return nil, err
		}
		result = append(result, opt)
	}
	return result, nil
}
