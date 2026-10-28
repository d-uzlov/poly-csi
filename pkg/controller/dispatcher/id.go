package dispatcher

import (
	"bytes"
	"text/template"
)

type IdHelper struct {
	Fields IdFields `json:"fields"`
}

type IdFields struct {
	RequestName string `json:"requestName"`
	K8sNs       string `json:"k8SNs"`
	K8sPvc      string `json:"k8SPvc"`
}

func NewIdHelper(requestName string, params map[string]string) IdHelper {
	return IdHelper{
		Fields: IdFields{
			RequestName: requestName,
			K8sNs:       params["csi.storage.k8s.io/pvc/namespace"],
			K8sPvc:      params["csi.storage.k8s.io/pvc/name"],
		},
	}
}

func (id IdHelper) Generate(tmpl string) (string, error) {
	t, err := template.New("").Parse(tmpl)
	if err != nil {
		return "", err
	}

	buf := bytes.Buffer{}
	err = t.Execute(&buf, id.Fields)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
