package publishcontext

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/go-playground/validator/v10"
	"meoe.io/poly-csi/pkg/api/protocols"
	"meoe.io/poly-csi/pkg/api/volumecontext"
	"meoe.io/poly-csi/pkg/utils/encrypt"
	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

type PublishContext struct {
	Protocol       protocols.Name        `map:"protocol" validate:"required"`
	ProtocolPublic protocols.Config      `map:"public" validate:"-"`
	ProtocolSecret encrypt.SecretWrapper `map:"secret" validate:"required"`
	AddressList    []AddressOption       `map:"addr" validate:"required"`
}

type AddressOption struct {
	Name    string `map:"n" validate:"required"`
	Address string `map:"a" validate:"required"`
}

func (pc *PublishContext) SetProtocol(name protocols.Name, data protocols.Config, publicKey encrypt.RsaPublicKey) error {
	pc.Protocol = name

	public, secret, err := protocols.SplitSecret(data)
	if err != nil {
		return fmt.Errorf("split protocol info: %w", err)
	}

	pc.ProtocolPublic = public

	secretBytes, err := json.Marshal(secret)
	if err != nil {
		return err
	}
	pc.ProtocolSecret, err = encrypt.NewWrapper(publicKey)
	if err != nil {
		return err
	}
	err = pc.ProtocolSecret.SetData(secretBytes)
	if err != nil {
		return err
	}
	return nil
}

func (pc *PublishContext) GetProtocol(key encrypt.RsaPrivateKey) (protocols.Config, error) {
	result := pc.ProtocolPublic.Clone()
	secretBytes, err := pc.ProtocolSecret.Extract(key)
	if err != nil {
		return nil, err
	}
	ptr := reflect.New(reflect.TypeOf(result))
	ptr.Elem().Set(reflect.ValueOf(result))
	err = json.Unmarshal(secretBytes, ptr.Interface())
	if err != nil {
		return nil, err
	}
	result = ptr.Elem().Interface().(protocols.Config)

	// TODO cache validator instance
	validate := validator.New(validator.WithRequiredStructEnabled())
	err = validate.Struct(result)
	if err != nil {
		err = protocols.FormatValidationError(err)
		return result, err
	}

	err = result.Validate()
	if err != nil {
		return result, fmt.Errorf("validateContext: %w", err)
	}

	return result, nil
}

func (pc *PublishContext) Validate() error {
	// TODO cache validator instance
	validate := validator.New(validator.WithRequiredStructEnabled())
	err := validate.Struct(pc)
	if err != nil {
		err = protocols.FormatValidationError(err)
		return err
	}
	return nil
}

func Parse(src map[string]string) (PublishContext, error) {
	var result PublishContext
	var proto []serialmaps.Registrar
	for _, v := range protocols.ListShareTypes() {
		proto = append(proto, v)
	}
	err := serialmaps.Parse(src, volumecontext.Prefix, &result, serialmaps.WithRegister(proto...))
	return result, err
}
