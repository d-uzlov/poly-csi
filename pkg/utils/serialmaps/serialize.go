package serialmaps

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// Serialize converts a struct value into map[string]string using the `map` tag.
func Serialize[T any](in T, prefix string) (map[string]string, error) {
	return SerializeInto(in, nil, prefix)
}

// SerializeInto will inject values into provided map if out is not nil
// If out already contains required key, SerializeInto will return an error.
func SerializeInto[T any](in T, out map[string]string, prefix string) (map[string]string, error) {
	v := reflect.ValueOf(in)
	if out == nil {
		out = map[string]string{}
	}
	tagValue := prefix
	prefix = ""
	err := serializeDispatch(v, out, prefix, tagValue)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func serializeDispatch(v reflect.Value, out map[string]string, prefix string, tagValue string) error {
	if !v.IsValid() {
		return fmt.Errorf("invalid value")
	}

	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return fmt.Errorf("nil pointer")
		}
		v = v.Elem()
	}

	tagFields := strings.Split(tagValue, ",")
	omitempty := slices.Contains(tagFields, "omitempty")

	key := tagFields[0]
	mapKey := key
	if prefix != "" {
		if strings.HasSuffix(prefix, "/") {
			mapKey = prefix + key
		} else {
			mapKey = prefix + "." + key
		}
	}
	_, ok := out[mapKey]
	if ok {
		return fmt.Errorf("out map already contains key '%s'", mapKey)
	}

	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			if omitempty {
				return nil
			}
			out[mapKey] = ""
			return nil
		}
		v = v.Elem()
		if !v.IsValid() {
			return fmt.Errorf("invalid value: %v", v)
		}
	}

	if omitempty && v.IsZero() {
		return nil
	}

	switch v.Kind() {
	case reflect.Interface:
		return serializeInterface(v, out, mapKey)
	case reflect.Struct:
		return serializeStruct(v, out, mapKey)
	case reflect.Map:
		return serializeMap(v, out, mapKey)
	case reflect.Array, reflect.Slice:
		return serializeArray(v, out, mapKey)
	case reflect.Pointer:
		// serializable data types should not contain pointers to pointers
		return fmt.Errorf("multilevel pointers are not supported")
	case reflect.String:
		out[mapKey] = v.String()
	case reflect.Bool:
		out[mapKey] = strconv.FormatBool(v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		out[mapKey] = strconv.FormatInt(v.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		out[mapKey] = strconv.FormatUint(v.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		out[mapKey] = strconv.FormatFloat(v.Float(), 'f', -1, v.Type().Bits())
	default:
		return fmt.Errorf("unsupported field kind: %s", v.Kind())
	}
	return nil
}

func serializeInterface(v reflect.Value, out map[string]string, prefix string) error {
	if v.Kind() != reflect.Interface {
		return fmt.Errorf("expected interface, got %s", v.Kind())
	}

	if v.IsNil() {
		typeKey := prefix + ".t"
		out[typeKey] = "nil"
		return nil
	}

	reg, ok := v.Interface().(Registrar)
	if !ok {
		return fmt.Errorf("interface must implement Registrar")
	}

	typename := reg.RegisterSerialmapsType()
	typeKey := prefix + ".t"
	out[typeKey] = typename

	return serializeDispatch(v.Elem(), out, prefix, "v")
}

func serializeStruct(v reflect.Value, out map[string]string, prefix string) error {
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("expected struct, got %s", v.Kind())
	}

	t := v.Type()
	for i := range t.NumField() {
		sf := t.Field(i)
		sv := v.Field(i)
		if !sv.IsValid() {
			return fmt.Errorf("field %s: invalid value: %v", sf.Name, sv)
		}

		// skip unexported fields
		if sf.PkgPath != "" {
			continue
		}

		tagValue := sf.Tag.Get(structMapTagName)
		if tagValue == "" {
			return fmt.Errorf("missing '%s' tag", structMapTagName)
		}
		if tagValue == "-" {
			continue
		}

		err := serializeDispatch(sv, out, prefix, tagValue)
		if err != nil {
			return fmt.Errorf("field %s: %w", sf.Name, err)
		}
	}

	return nil
}

func serializeArray(v reflect.Value, out map[string]string, prefix string) error {
	if v.Kind() != reflect.Array && v.Kind() != reflect.Slice {
		return fmt.Errorf("expected slice, got %s", v.Kind())
	}
	elemKind := v.Type().Elem().Kind()
	if elemKind == reflect.Uint8 {
		out[prefix] = base64.StdEncoding.EncodeToString(v.Interface().([]byte))
		return nil
	}
	lenKey := prefix + ".len"
	out[lenKey] = strconv.Itoa(v.Len())
	for i := range v.Len() {
		arrayValue := v.Index(i)
		err := serializeDispatch(arrayValue, out, prefix, strconv.Itoa(i))
		if err != nil {
			return fmt.Errorf("elem %d: %w", i, err)
		}
	}
	return nil
}

func serializeMap(v reflect.Value, out map[string]string, prefix string) error {
	if v.Kind() != reflect.Map {
		return fmt.Errorf("expected map, got %s", v.Kind())
	}

	var keys []string
	for _, k := range v.MapKeys() {
		if k.Kind() != reflect.String {
			return fmt.Errorf("map key is not string")
		}

		ks := k.String()
		if strings.Contains(ks, ",") {
			return fmt.Errorf("map key '%s' contains comma ','", ks)
		}

		keys = append(keys, ks)
	}

	slices.Sort(keys)
	keysKey := prefix + ".keys"
	out[keysKey] = strings.Join(keys, ",")

	for i, k := range keys {
		val := v.MapIndex(reflect.ValueOf(k))
		if !val.IsValid() {
			return fmt.Errorf("elem '%s': invalid value", k)
		}

		err := serializeDispatch(val, out, prefix, strconv.Itoa(i))
		if err != nil {
			return fmt.Errorf("elem '%s': %w", k, err)
		}
	}

	return nil
}
