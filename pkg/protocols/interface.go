package protocols

import (
	"encoding/json"
	"fmt"
	
	"github.com/go-playground/validator/v10"

	"meoe.io/poly-csi/pkg/utils/secrets"
	"meoe.io/poly-csi/pkg/utils/serialmaps"
)

type Config interface {
	Validate() error
}

func Parse[T Config](public map[string]string, secret []byte) (T, error) {
	var result T
	err := serialmaps.Parse(public, "", &result)
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
