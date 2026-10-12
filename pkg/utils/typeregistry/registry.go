package typeregistry

import (
	"fmt"
	"reflect"
)

type Registry struct {
	types map[string]reflect.Type
}

func New() Registry {
	return Registry{
		types: make(map[string]reflect.Type),
	}
}

type Sample struct {
	Name   string
	Sample any
}

func NewPrepopulated(samples []Sample) Registry {
	reg:= Registry{
		types: make(map[string]reflect.Type),
	}
	for _, v := range samples {
		reg.Register(v.Name, v.Sample)
	}
	return reg
}

func (r *Registry) Register(name string, sample any) {
	t := reflect.TypeOf(sample)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	r.types[name] = t
}

func (r *Registry) NameOf(v reflect.Value) (string, error) {
	t := v.Type()
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	for name, registeredType := range r.types {
		if t == registeredType {
			return name, nil
		}
	}
	return "", fmt.Errorf("type is not registered: '%s'", t.String())
}

func (r *Registry) CreateInstance(name string) (reflect.Value, error) {
	t, exists := r.types[name]
	if !exists {
		return reflect.Value{}, fmt.Errorf("typename is not registered: '%s'", name)
	}
	return reflect.New(t), nil
}
