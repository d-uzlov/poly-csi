package cntrnvme

import "fmt"

type Config struct {
	NvmeCmd        []string `yaml:"nvmeCmd"`
	ParseKernelLog bool     `yaml:"parseKernelLog"`
}

func (c *Config) Validate() error {
	if len(c.NvmeCmd) == 0 {
		return fmt.Errorf("nvmeCmd missing")
	}
	return nil
}
