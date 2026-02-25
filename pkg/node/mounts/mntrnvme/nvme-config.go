package mntrnvme

import "fmt"

type Config struct {
	NvmeCmd        []string `yaml:"nvmeCmd"`
	ParseKernelLog bool     `yaml:"parseKernelLog"`
	DevFolder      string   `yaml:"devFolder"`
}

func (c *Config) FillDefaults() {}

func (c *Config) Validate() error {
	if len(c.NvmeCmd) == 0 {
		return fmt.Errorf("nvmeCmd missing")
	}
	if c.DevFolder == "" {
		return fmt.Errorf("devFolder missing")
	}
	return nil
}
