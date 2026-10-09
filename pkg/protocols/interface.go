package protocols

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/go-playground/validator/v10"

	"meoe.io/poly-csi/pkg/controller/volumecontext"
	"meoe.io/poly-csi/pkg/utils/secrets"
	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

type Config interface {
	Validate() error
}

type Name string

type NodeInfo Config

func Parse[T Config](public map[string]string, secret []byte) (T, error) {
	var result T
	err := serialmaps.Parse(public, volumecontext.ProtocolPrefix, &result)
	if err != nil {
		return result, fmt.Errorf("public: %w", err)
	}

	if len(secret) != 0 {
		err = json.Unmarshal(secret, &result)
		if err != nil {
			return result, fmt.Errorf("secret: %w", err)
		}
	}

	// TODO cache validator instance
	validate := validator.New(validator.WithRequiredStructEnabled())
	err = validate.Struct(result)
	if err != nil {
		err = FormatValidationError(err)
		return result, err
	}

	err = result.Validate()
	if err != nil {
		return result, fmt.Errorf("validateContext: %w", err)
	}

	return result, nil
}

func SplitSecret[T Config](conf T) (public T, secret T, err error) {
	var zero T
	public, err = secrets.ClearFields(conf, "")
	if err != nil {
		return zero, zero, err
	}
	secret, err = secrets.ClearFields(conf, "+")
	if err != nil {
		return zero, zero, err
	}
	return
}

func FormatValidationError(err error) error {
	if err == nil {
		return nil
	}

	var validationErrors validator.ValidationErrors
	ok := errors.As(err, &validationErrors)
	if !ok {
		return err
	}

	var errList []string

	for _, e := range validationErrors {
		if e.Param() != "" {
			errList = append(errList, fmt.Sprintf("field '%s=%v' failed rule '%s=%s'", e.Field(), e.Value(), e.Tag(), e.Param()))
		} else {
			errList = append(errList, fmt.Sprintf("field '%s=%v' failed rule '%s'", e.Field(), e.Value(), e.Tag()))
		}
	}

	if len(errList) == 0 {
		panic("error is not nil but list of errors is empty")
	}

	return errors.New(strings.Join(errList, ", "))
}

type Sample struct {
	Share Config
	Info  NodeInfo
}

var protocols = map[string]Sample{
	string(Aoe):   {Share: AoeShare{}, Info: nil},
	string(Iscsi): {Share: IscsiShare{}, Info: IscsiNodeInfo{}},
	string(Nbd):   {Share: NbdShare{}, Info: nil},
	string(Nfs):   {Share: NfsShare{}, Info: nil},
	string(Nvme):  {Share: NvmeShare{}, Info: nil},
	string(Smb):   {Share: SmbShare{}, Info: nil},
}

func ParseNodeInfo(name Name, data string, out *NodeInfo) error {
	sample, ok := protocols[string(name)]
	if !ok {
		keys := slices.Collect(maps.Keys(protocols))
		slices.Sort(keys)
		return fmt.Errorf("unknown protocol name '%s', must be one of %v", name, strings.Join(keys, ", "))
	}
	if sample.Info == nil {
		if data != "" && data != "null" {
			return fmt.Errorf("trying to parse node info for a protocol '%s' that does not feature node info: data='%s'", name, data)
		}
		return nil
	}

	err := json.Unmarshal([]byte(data), &sample.Info)
	if err != nil {
		return err
	}
	*out = sample.Info
	return nil
}
