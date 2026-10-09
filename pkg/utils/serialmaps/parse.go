package serialmaps

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

const structMapTagName = "map"

// Parse fills a struct of type T from map[string]string using the `map` tag.
// Example: `Qwe string `map:"qwe"“
func Parse[T any](src map[string]string, prefix string, out *T) error {
	return ParseRemove(src, prefix, false, out)
}

// ParseRemove parses struct from a map, removing any fields that it uses from src
func ParseRemove[T any](src map[string]string, prefix string, remove bool, out *T) error {
	if out == nil {
		return fmt.Errorf("nil out pointer")
	}
	v := reflect.ValueOf(out)
	if v.Kind() != reflect.Pointer {
		return fmt.Errorf("out is not a pointer")
	}
	tagValue := prefix
	prefix = ""
	return parseDispatch(src, v.Elem(), prefix, tagValue, remove)
}

func parseDispatch(src map[string]string, v reflect.Value, prefix string, tagValue string, remove bool) error {
	if !v.CanSet() {
		return fmt.Errorf("not settable")
	}

	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}

	tagFields := strings.Split(tagValue, ",")
	key := tagFields[0]
	mapKey := key
	if prefix != "" {
		if strings.HasSuffix(prefix, "/") {
			mapKey = prefix + key
		} else {
			mapKey = prefix + "." + key
		}
	}

	switch v.Kind() {
	case reflect.Struct:
		return parseStruct(src, v, mapKey, remove)

	case reflect.Map:
		return parseMap(src, v, mapKey, remove)

	case reflect.Array, reflect.Slice:
		return parseArray(src, v, mapKey, remove)
	default:
		// continue to simplified parsing below
	}

	s, ok := src[mapKey]
	if !ok {
		return nil
	}
	if remove {
		delete(src, mapKey)
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

func parseStruct(src map[string]string, v reflect.Value, prefix string, remove bool) error {
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("expected struct, got %s", v.Kind())
	}

	t := v.Type()
	for i := range t.NumField() {
		sf := t.Field(i)

		// Skip unexported fields.
		if sf.PkgPath != "" {
			continue
		}

		tagValue := sf.Tag.Get(structMapTagName)
		if tagValue == "" {
			return fmt.Errorf("missing '%s' tag", structMapTagName)
		}
		if tagValue == "-" {
			return nil
		}

		if err := parseDispatch(src, v.Field(i), prefix, tagValue, remove); err != nil {
			return fmt.Errorf("field %s: %w", sf.Name, err)
		}
	}

	return nil
}

func parseArray(src map[string]string, v reflect.Value, prefix string, remove bool) error {
	if v.Kind() != reflect.Array && v.Kind() != reflect.Slice {
		return fmt.Errorf("expected slice, got %s", v.Kind())
	}

	lenKey := prefix + ".len"
	lenStr, ok := src[lenKey]
	if !ok {
		return fmt.Errorf("missing key: '%s'", lenKey)
	}

	len, err := strconv.Atoi(lenStr)
	if err != nil {
		return fmt.Errorf("parse array length: '%s=%s': %w", lenKey, lenStr, err)
	}

	if v.Kind() == reflect.Slice && v.IsNil() {
		v.Set(reflect.MakeSlice(v.Type(), len, len))
	}

	for i := range len {
		instance := v.Index(i)
		if instance.Kind() == reflect.Pointer && instance.IsNil() {
			instance = reflect.New(v.Type().Elem())
		}
		err := parseDispatch(src, instance, prefix, strconv.Itoa(i), remove)
		if err != nil {
			return fmt.Errorf("elem %d: %w", i, err)
		}
	}

	return nil
}

func parseMap(src map[string]string, v reflect.Value, prefix string, remove bool) error {
	if v.Kind() != reflect.Map {
		return fmt.Errorf("expected map, got %s", v.Kind())
	}

	keysKey := prefix + ".keys"
	keysStr, ok := src[keysKey]
	if !ok {
		return fmt.Errorf("missing key: '%s'", keysKey)
	}
	keys := strings.Split(keysStr, ",")

	if v.IsNil() {
		v.Set(reflect.MakeMap(v.Type()))
	}

	for _, k := range keys {
		key := reflect.ValueOf(k)
		instance := reflect.New(v.Type().Elem()).Elem()
		err := parseDispatch(src, instance, prefix, k, remove)
		if err != nil {
			return fmt.Errorf("elem '%s': %w", k, err)
		}
		v.SetMapIndex(key, instance)
	}

	return nil
}
