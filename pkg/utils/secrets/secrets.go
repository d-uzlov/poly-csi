package secrets

import (
	"fmt"
	"reflect"
)

const securityTag = "secret"

func ClearFields[T any](obj T, securityValue string) (T, error) {
	v := reflect.ValueOf(obj)
	if !v.IsValid() {
		return obj, fmt.Errorf("invalid value")
	}

	pointer := false
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return obj, nil
		}
		v = v.Elem()
		pointer = true
	}

	if v.Kind() != reflect.Struct {
		return obj, fmt.Errorf("input obj is not a struct")
	}

	resultPtr := reflect.New(v.Type())
	result := resultPtr.Elem()
	t := result.Type()

	for i := 0; i < result.NumField(); i++ {
		field := t.Field(i)

		if field.PkgPath != "" {
			return obj, fmt.Errorf("input obj has unexported field: %v", field.Name)
		}

		if field.Tag.Get(securityTag) != securityValue {
			result.Field(i).Set(v.Field(i))
		} else {
			result.Field(i).SetZero()
		}
	}

	if pointer {
		return resultPtr.Interface().(T), nil
	} else {
		return result.Interface().(T), nil
	}
}
