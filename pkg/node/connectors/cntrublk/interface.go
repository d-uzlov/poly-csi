package cntrublk

import (
	"meoe.io/poly-csi/pkg/node/connectors"
)

type UblkCallback interface {
	Add(devId string, pid string)
	Remove(devId string)
}

type InflateFunc func(callback UblkCallback, conf connectors.CommonConfig) connectors.Manager
