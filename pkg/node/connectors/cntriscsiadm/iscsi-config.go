package cntriscsiadm

import "fmt"

type Config struct {
	IscsiadmCmd []string `yaml:"iscsiadmCmd"`
}

func (c *Config) FillDefaults() {}

func (c *Config) Validate() error {
	if len(c.IscsiadmCmd) == 0 {
		return fmt.Errorf("iscsiadmCmd missing")
	}
	return nil
}
