package mntriscsiadm

import "fmt"

type Config struct {
	IscsiadmCmd []string `yaml:"iscsiadmCmd"`
	DevFolder     string   `yaml:"devFolder"`
}

func (c *Config) FillDefaults() {}

func (c *Config) Validate() error {
	if len(c.IscsiadmCmd) == 0 {
		return fmt.Errorf("iscsiadmCmd missing")
	}
	if c.DevFolder == "" {
		return fmt.Errorf("devFolder missing")
	}
	return nil
}
