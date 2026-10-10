package cntriscsiadm

import "fmt"

type Config struct {
	IscsiadmCmd       []string `yaml:"iscsiadmCmd"`
	InitiatorNamePath string   `yaml:"initiatorNamePath"`
}

func (c *Config) FillDefaults() {}

func (c *Config) Validate() error {
	if len(c.IscsiadmCmd) == 0 {
		return fmt.Errorf("iscsiadmCmd missing")
	}
	if len(c.InitiatorNamePath) == 0 {
		return fmt.Errorf("initiatorNamePath missing")
	}
	return nil
}
