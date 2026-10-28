package serialmaps

import "strings"

type Parser struct {
	parent *Parser
	src    map[string]string
	prefix string
	opts   []Option
}

func NewParser(src map[string]string, opts ...Option) *Parser {
	return &Parser{
		parent: nil,
		src:    src,
		prefix: "",
		opts:   opts,
	}
}

func (p *Parser) Parse[T any](out *T) error {
	if p.parent != nil {
		err := p.parent.Parse(out)
		if err != nil {
			return err
		}
	}
	return Parse(p.src, p.prefix, out, p.opts...)
}

func (p *Parser) Take(key string) string {
	fullKey := key
	if p.prefix != "" {
		fullKey = p.prefix + "." + key
	}
	val := p.src[fullKey]
	delete(p.src, fullKey)
	return val
}

func (p *Parser) WithPrefix(prefix string) *Parser {
	fullPrefix := prefix
	if p.prefix != "" {
		fullPrefix = p.prefix + "." + prefix
	}
	return &Parser{
		src:    p.src,
		prefix: fullPrefix,
		opts:   p.opts,
	}
}

func (p *Parser) HasRemaining() bool {
	fullPrefix := p.prefix + "."
	for k := range p.src {
		if strings.HasPrefix(k, fullPrefix) {
			return true
		}
	}
	return false
}

func (p *Parser) WithOverride(prefix string) *Parser {
	return &Parser{
		parent: p,
		src:    p.src,
		prefix: prefix,
		opts:   p.opts,
	}
}
