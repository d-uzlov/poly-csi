package serialmaps

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

const structMapTagName = "map"

type parser struct {
	src map[string]string
	opt options
}

// Parse fills a struct of type T from map[string]string using the `map` tag.
// Example: `Qwe string `map:"qwe"“
func Parse[T any](src map[string]string, prefix string, out *T, opts ...Option) error {
	if out == nil {
		return fmt.Errorf("nil out pointer")
	}
	v := reflect.ValueOf(out)
	if v.Kind() != reflect.Pointer {
		return fmt.Errorf("out is not a pointer")
	}
	tagValue := prefix
	prefix = ""
	opt := options{}
	for _, v := range opts {
		v(&opt)
	}
	p := parser{
		src: src,
		opt: opt,
	}
	return p.parseDispatch(v.Elem(), prefix, tagValue)
}

func (p *parser) parseDispatch(v reflect.Value, prefix string, tagValue string) error {
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
		return p.parseStruct(v, mapKey)

	case reflect.Map:
		return p.parseMap(v, mapKey)

	case reflect.Array, reflect.Slice:
		return p.parseArray(v, mapKey)

	case reflect.Interface:
		return p.parseInterface(v, mapKey)

	case reflect.Chan:
		return fmt.Errorf("value kind %s is not supported", v.Kind())

	default:
		// continue to simplified parsing below
	}

	s, ok := p.src[mapKey]
	if !ok {
		return nil
	}
	if p.opt.removeToken {
		delete(p.src, mapKey)
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

func (p *parser) parseInterface(v reflect.Value, prefix string) error {
	if v.Kind() != reflect.Interface {
		return fmt.Errorf("expected interface, got %s", v.Kind())
	}

	if !v.IsNil() {
		return p.parseDispatch(v.Elem(), prefix, "v")
	}

	typeKey := prefix + ".t"
	typename, ok := p.src[typeKey]
	if !ok {
		return fmt.Errorf("missing key '%s'", typeKey)
	}
	if typename == "nil" {
		// the interface was nil during serialize
		return nil
	}

	elem, err := p.opt.reg.CreateInstance(typename)
	if err != nil {
		return err
	}
	t := v.Type()
	if !elem.Type().Implements(t) {
		return fmt.Errorf("type registered with '%s' does not implement interface %s", typename, t.String())
	}
	if elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}

	err = p.parseDispatch(elem, prefix, "v")
	if err != nil {
		return err
	}
	// need to set after parsing in case the interface is stored by value
	if elem.Type().Implements(t) {
		v.Set(elem)
	} else if elem.Addr().Type().Implements(t) {
		v.Set(elem.Addr())
	} else {
		return fmt.Errorf("type registered with '%s' does not implement interface %s", typename, t.String())
	}
	return nil
}

func (p *parser) parseStruct(v reflect.Value, prefix string) error {
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
			return fmt.Errorf("missing '%s' tag on field '%s' of type '%s'", structMapTagName, sf.Name, t.Name())
		}
		if tagValue == "-" {
			continue
		}

		f := v.Field(i)
		err := p.parseDispatch(f, prefix, tagValue)
		if err != nil {
			return fmt.Errorf("field %s: %w", sf.Name, err)
		}
	}

	return nil
}

func (p *parser) parseArray(v reflect.Value, prefix string) error {
	if v.Kind() != reflect.Array && v.Kind() != reflect.Slice {
		return fmt.Errorf("expected slice, got %s", v.Kind())
	}

	elemKind := v.Type().Elem().Kind()
	if elemKind == reflect.Uint8 {
		arr, err := base64.StdEncoding.DecodeString(p.src[prefix])
		if err != nil {
			return fmt.Errorf("base64 decode: %w", err)
		}
		v.Set(reflect.ValueOf(arr))
		return nil
	}

	lenKey := prefix + ".len"
	lenStr, ok := p.src[lenKey]
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
		err := p.parseDispatch(instance, prefix, strconv.Itoa(i))
		if err != nil {
			return fmt.Errorf("elem %d: %w", i, err)
		}
	}

	return nil
}

func (p *parser) parseMap(v reflect.Value, prefix string) error {
	if v.Kind() != reflect.Map {
		return fmt.Errorf("expected map, got %s", v.Kind())
	}

	keysKey := prefix + ".keys"
	keysStr, ok := p.src[keysKey]
	if !ok {
		return fmt.Errorf("missing key: '%s'", keysKey)
	}
	keys := strings.Split(keysStr, ",")

	if v.IsNil() {
		v.Set(reflect.MakeMap(v.Type()))
	}

	for i, k := range keys {
		key := reflect.ValueOf(k)
		instance := reflect.New(v.Type().Elem()).Elem()
		err := p.parseDispatch(instance, prefix, strconv.Itoa(i))
		if err != nil {
			return fmt.Errorf("elem '%s': %w", k, err)
		}
		v.SetMapIndex(key, instance)
	}

	return nil
}
