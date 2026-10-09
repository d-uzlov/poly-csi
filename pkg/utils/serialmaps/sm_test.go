package serialmaps

import (
	"maps"
	"reflect"
	"testing"
)

type Simple struct {
	Qwe       string `map:"qwe1"`
	Asd       int    `map:"a-s-d"`
	Zxc       int    `map:"zxc,omitempty"`
	Ephemeral string `map:"-"`
	Ok        bool   `map:"is_ok"`
	Ptr       *int   `map:"ptr"`
}

func TestSimple(t *testing.T) {
	input := Simple{Qwe: "123", Asd: 456, Ephemeral: "789", Ok: true, Ptr: new(1)}

	serialized, err := Serialize(input, "zzz")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	expected := map[string]string{
		"zzz.qwe1":  "123",
		"zzz.a-s-d": "456",
		"zzz.is_ok": "true",
		"zzz.ptr":   "1",
	}

	if !maps.Equal(serialized, expected) {
		t.Error("serialized != expected")
		t.Error("serialized", serialized)
		t.Error("expected  ", expected)
		t.Fail()
	}

	serialized2, err := Serialize(input, "prefix")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	expected2 := map[string]string{
		"prefix.qwe1":  "123",
		"prefix.a-s-d": "456",
		"prefix.is_ok": "true",
		"prefix.ptr":   "1",
	}

	if !maps.Equal(serialized2, expected2) {
		t.Error("serialized2 != expected2")
		t.Error("serialized2", serialized2)
		t.Error("expected2  ", expected2)
		t.Fail()
	}

	parsed := Simple{}
	err = Parse(serialized, "zzz", &parsed)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	input.Ephemeral = ""

	if !reflect.DeepEqual(parsed, input) {
		t.Error("parsed != expected")
		t.Error("parsed", parsed)
		t.Error("expected", input)
		t.Fail()
	}
}

type WithArray struct {
	Str    string   `map:"str"`
	StrArr []string `map:"str_arr"`
	IntArr [2]int   `map:"int_arr"`
	ArrPtr *[2]int  `map:"arr_ptr"`
}

func TestWithArray(t *testing.T) {
	input := WithArray{Str: "123", StrArr: []string{"1", "2"}, IntArr: [2]int{3, 4}, ArrPtr: &[2]int{10, 11}}

	serialized, err := Serialize(input, "zzz")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	expected := map[string]string{
		"zzz.str":         "123",
		"zzz.str_arr.len": "2",
		"zzz.str_arr.0":   "1",
		"zzz.str_arr.1":   "2",
		"zzz.int_arr.len": "2",
		"zzz.int_arr.0":   "3",
		"zzz.int_arr.1":   "4",
		"zzz.arr_ptr.len": "2",
		"zzz.arr_ptr.0":   "10",
		"zzz.arr_ptr.1":   "11",
	}

	if !maps.Equal(serialized, expected) {
		t.Error("serialized != expected")
		t.Error("serialized", serialized)
		t.Error("expected  ", expected)
		t.Fail()
	}

	parsed := WithArray{}
	err = Parse(serialized, "zzz", &parsed)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if !reflect.DeepEqual(parsed, input) {
		t.Error("parsed != expected")
		t.Error("parsed", parsed)
		t.Error("expected", input)
		t.Fail()
	}
}

type WithMap struct {
	M   map[string]string  `map:"nested_map"`
	Ptr *map[string]string `map:"map_ptr"`
}

func TestWithMap(t *testing.T) {
	input := WithMap{M: map[string]string{"k1": "v1", "k2": "v2"}, Ptr: &map[string]string{"k1": "v1", "k2": "v2"}}

	serialized, err := Serialize(input, "zzz")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	expected := map[string]string{
		"zzz.nested_map.keys": "k1,k2",
		"zzz.nested_map.k1":   "v1",
		"zzz.nested_map.k2":   "v2",
		"zzz.map_ptr.keys":    "k1,k2",
		"zzz.map_ptr.k1":      "v1",
		"zzz.map_ptr.k2":      "v2",
	}

	if !maps.Equal(serialized, expected) {
		t.Error("serialized != expected")
		t.Error("serialized", serialized)
		t.Error("expected  ", expected)
		t.Fail()
	}

	parsed := WithMap{}
	err = Parse(serialized, "zzz", &parsed)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if !reflect.DeepEqual(parsed, input) {
		t.Error("parsed != expected")
		t.Error("parsed", parsed)
		t.Error("expected", input)
		t.Fail()
	}
}

type VerySimple struct {
	Val string `map:"val"`
}

type WithNestedStructs struct {
	Nested VerySimple            `map:"s"`
	Ptr    *VerySimple           `map:"ptr"`
	Mapped map[string]VerySimple `map:"mapped"`
	Sliced []VerySimple          `map:"sliced"`
}

func TestWithNested(t *testing.T) {
	input := WithNestedStructs{
		Nested: VerySimple{Val: "val"},
		Ptr:    &VerySimple{Val: "val_in_ptr"},
		Mapped: map[string]VerySimple{"key": {Val: "mapped_val"}},
		Sliced: []VerySimple{{Val: "sliced_0"}, {"sliced_1"}},
	}

	serialized, err := Serialize(input, "zzz")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	expected := map[string]string{
		"zzz.s.val":          "val",
		"zzz.ptr.val":        "val_in_ptr",
		"zzz.mapped.keys":    "key",
		"zzz.mapped.key.val": "mapped_val",
		"zzz.sliced.len":     "2",
		"zzz.sliced.0.val":   "sliced_0",
		"zzz.sliced.1.val":   "sliced_1",
	}

	if !maps.Equal(serialized, expected) {
		t.Error("serialized != expected")
		t.Error("serialized", serialized)
		t.Error("expected  ", expected)
		t.Fail()
	}

	parsed := WithNestedStructs{}
	err = Parse(serialized, "zzz", &parsed)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if !reflect.DeepEqual(parsed, input) {
		t.Error("parsed != expected")
		t.Error("parsed", parsed)
		t.Error("expected", input)
		t.Fail()
	}
}
