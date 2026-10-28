package serialmaps

import "testing"

type ParserTestStruct struct {
	Field string `map:"f"`
}

func TestParser(t *testing.T) {
	src := map[string]string{
		"top":      "123",
		"man.f0":   "456",
		"nested.f": "789",
	}

	parser := NewParser(src, WithRemove(true))
	top := parser.Take("top")
	if top != "123" {
		t.Error("top", top, "!=", "expected", "123")
	}

	nestedManual := parser.WithPrefix("man").Take("f0")
	if nestedManual != "456" {
		t.Error("nestedManual", nestedManual, "!=", "expected", "456")
	}

	var nested ParserTestStruct
	err := parser.WithPrefix("nested").Parse(&nested)
	if err != nil {
		t.Error(err)
	}

	if nested.Field != "789" {
		t.Error("nested.Field", nested.Field, "!=", "expected", "789")
	}
}
