package mntrnvme

import (
	"fmt"
)

type NvmePublishContext struct {
	Nqn         string
	Port        string
	Transport   string
	IoQueues    string
}

func InjectPublishContext(dst map[string]string, src NvmePublishContext) error {
	dst["nvme.nqn"] = src.Nqn
	dst["nvme.port"] = src.Port
	dst["nvme.transport"] = src.Transport
	dst["nvme.io_queues"] = src.IoQueues
	return nil
}

func ParsePublishContext(src map[string]string) (NvmePublishContext, error) {
	nqn := src["nvme.nqn"]
	if nqn == "" {
		return NvmePublishContext{}, fmt.Errorf("missing 'nvme.nqn'")
	}
	port := src["nvme.port"]
	if port == "" {
		return NvmePublishContext{}, fmt.Errorf("missing 'nvme.port'")
	}
	transport := src["nvme.transport"]
	if transport == "" {
		return NvmePublishContext{}, fmt.Errorf("missing 'nvme.transport'")
	}
	ioQueues := src["nvme.io_queues"]
	if ioQueues == "" {
		return NvmePublishContext{}, fmt.Errorf("missing 'nvme.io_queues'")
	}

	return NvmePublishContext{
		Nqn:         nqn,
		Port:        port,
		Transport:   transport,
		IoQueues:    ioQueues,
	}, nil
}
