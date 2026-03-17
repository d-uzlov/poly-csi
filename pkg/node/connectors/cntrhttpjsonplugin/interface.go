package cntrhttpjsonplugin

import "meoe.io/poly-csi/pkg/logs"

type PluginResponse struct {
	Status string            `json:"status"`
	Err    string            `json:"error"`
	Result string            `json:"result"`
	Logs   []logs.LogRequest `json:"logs"`
}

const ResponseStatusError = "error"
const ResponseStatusOk = "ok"
