package serialmaps

import (
	"fmt"
	"reflect"
	"strconv"
)

const tag = "map"

// Parse fills a struct of type T from map[string]string using the `map` tag.
// Example: `Qwe string `map:"qwe"“
func Parse[T any](src map[string]string, prefix string, out *T) error {
	return ParseRemove(src, prefix, false, out)
}

// ParseRemove parses struct from a map, removing any fields that it uses from src
func ParseRemove[T any](src map[string]string, prefix string, remove bool, out *T) error {
	v := reflect.ValueOf(out).Elem()
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("expected struct, got %s", v.Kind())
	}

	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)

		// Skip unexported fields.
		if sf.PkgPath != "" {
			continue
		}

		key := sf.Tag.Get(tag)
		if key == "" {
			return fmt.Errorf("field %s: missing '%s' tag", sf.Name, tag)
		}
		if key == "-" {
			continue
		}

		mapKey := prefix + key
		raw, ok := src[mapKey]
		if !ok {
			continue
		}
		if remove {
			delete(src, mapKey)
		}

		if err := setFromString(v.Field(i), raw); err != nil {
			return fmt.Errorf("field %s: %w", sf.Name, err)
		}
	}

	return nil
}

// Serialize converts a struct value into map[string]string using the `map` tag.
func Serialize[T any](in T, prefix string) (map[string]string, error) {
	v := reflect.ValueOf(in)
	if !v.IsValid() {
		return nil, fmt.Errorf("invalid value")
	}

	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil, fmt.Errorf("nil pointer")
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct, got %s", v.Kind())
	}

	t := v.Type()
	out := make(map[string]string, t.NumField())

	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)

		// Skip unexported fields.
		if sf.PkgPath != "" {
			continue
		}

		key := sf.Tag.Get(tag)
		if key == "" {
			return nil, fmt.Errorf("field %s: missing '%s' tag", sf.Name, tag)
		}
		if key == "-" {
			continue
		}

		s, err := stringFromValue(v.Field(i))
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", sf.Name, err)
		}
		out[prefix+key] = s
	}

	return out, nil
}

func setFromString(v reflect.Value, s string) error {
	if !v.CanSet() {
		return fmt.Errorf("not settable")
	}

	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.String:
		v.SetString(s)
		return nil

	case reflect.Bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return err
		}
		v.SetBool(b)
		return nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(s, 10, v.Type().Bits())
		if err != nil {
			return err
		}
		v.SetInt(n)
		return nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		n, err := strconv.ParseUint(s, 10, v.Type().Bits())
		if err != nil {
			return err
		}
		v.SetUint(n)
		return nil

	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(s, v.Type().Bits())
		if err != nil {
			return err
		}
		v.SetFloat(f)
		return nil

	default:
		return fmt.Errorf("unsupported field kind: %s", v.Kind())
	}
}

func stringFromValue(v reflect.Value) (string, error) {
	if !v.IsValid() {
		return "", nil
	}

	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return "", nil
		}
		return stringFromValue(v.Elem())
	}

	switch v.Kind() {
	case reflect.String:
		return v.String(), nil
	case reflect.Bool:
		return strconv.FormatBool(v.Bool()), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(v.Uint(), 10), nil
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, v.Type().Bits()), nil
	default:
		return "", fmt.Errorf("unsupported field kind: %s", v.Kind())
	}
}
