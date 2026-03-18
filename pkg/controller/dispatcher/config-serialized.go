package dispatcher

import (
	"fmt"
	"regexp"

	"meoe.io/poly-csi/pkg/logs"
)

type Config struct {
	NodeInfoDb NodeInfoStorageConfig `yaml:"nodeInfoDb"`
	Servers    []*ServerConfig       `yaml:"servers"`
	Logs       Logs                  `yaml:"logs" json:"logs"`
}

type KeyInfoConfig struct {
	Path string `yaml:"path"`
}

type Logs struct {
	Rpc map[string]logs.Config `yaml:"nodeRpc" json:"nodeRpc"`
}

func (c *Config) Validate() (errorList []string, warningList []string) {
	if c.NodeInfoDb.Address == "" {
		errorList = append(errorList, "nodeInfoDb.address: field is empty")
	}
	for _, v := range c.Servers {
		er, wl := v.Validate()
		for i, iss := range er {
			er = append(er, fmt.Sprintf("servers[i=%d/name=%s].%s", i, v.Name, iss))
		}
		for i, iss := range wl {
			wl = append(wl, fmt.Sprintf("servers[i=%d/name=%s].%s", i, v.Name, iss))
		}
	}
	return
}

type NodeInfoStorageConfig struct {
	Address string `yaml:"address"`
}

type ServerConfig struct {
	Name         string              `yaml:"name"`
	StableId     string              `yaml:"stableId"`
	ExecType     string              `yaml:"execType"`
	ExecConfig   map[string]any      `yaml:"execConfig"`
	DbType       string              `yaml:"metadataType"`
	DbConfig     map[string]any      `yaml:"metadataConfig"`
	GlobalParams map[string]string   `yaml:"globalParams"`
	Commands     map[string][]string `yaml:"commands"`
	AddressList  []AddressOption     `yaml:"addressList"`
}

func (c *ServerConfig) Validate() (errorList []string, warningList []string) {
	if c.Name == "" {
		errorList = append(errorList, "name: field is empty")
	}
	if ok := regexp.MustCompile(`^[-a-zA-Z0-9]*$`).MatchString(c.Name); !ok {
		errorList = append(errorList, "name: field must contain only alphanumeric symbols or '-'")
	}
	if c.StableId == "" {
		errorList = append(errorList, "stableId: field is empty")
	}
	if len(c.StableId) != 10 {
		errorList = append(errorList, "stableId: field length is not 10")
	}
	if ok := regexp.MustCompile(`^[a-zA-Z0-9]*$`).MatchString(c.StableId); !ok {
		errorList = append(errorList, "stableId: field must contain only alphanumeric symbols")
	}
	if c.ExecType == "" {
		errorList = append(errorList, "transportType: field is empty")
	}
	if c.DbType == "" {
		errorList = append(errorList, "metadataType: field is empty")
	}
	return
}

type AddressOption struct {
	Name     string `yaml:"name"`
	StableId string `yaml:"stableId"`
	Address  string `yaml:"address"`
}

func (c *AddressOption) Validate() (errorList []string, warningList []string) {
	if c.Name == "" {
		errorList = append(errorList, "name: field is empty")
	}
	if ok := regexp.MustCompile(`^[-a-zA-Z0-9]*$`).MatchString(c.Name); !ok {
		errorList = append(errorList, "name: field must contain only alphanumeric symbols or '-'")
	}
	if c.StableId == "" {
		errorList = append(errorList, "stableId: field is empty")
	}
	if len(c.StableId) != 10 {
		errorList = append(errorList, "stableId: field length is not 10")
	}
	if ok := regexp.MustCompile(`^[a-zA-Z0-9]*$`).MatchString(c.StableId); !ok {
		errorList = append(errorList, "stableId: field must contain only alphanumeric symbols")
	}
	if c.Address == "" {
		errorList = append(errorList, "address: field is empty")
	}
	return
}
