// Package fieldmask parses read-only JSON field masks and projects JSON values.
package fieldmask

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode"
)

const (
	maxInputLength = 4096
	maxDepth       = 16
	maxSelections  = 256
)

var ErrInvalidMask = errors.New("invalid field mask")

// Mask is a validated selection against one JSON response schema.
type Mask struct {
	all    bool
	fields map[string]*selection
	schema *objectSchema
	canon  string
}

type selection struct {
	all           bool
	whole         bool
	requireObject bool
	fields        map[string]*selection
}

type objectSchema struct {
	fields map[string]*schemaField
}

type schemaField struct {
	name  string
	child *objectSchema
}

// Parse compiles fields using JSON tags in schema. Empty fields means all fields,
// matching an omitted query parameter. Use a non-empty value for explicit masks.
func Parse(fields string, schema any) (*Mask, error) {
	return ParseOptional(fields, fields != "", schema)
}

// ParseOptional compiles fields using JSON tags in schema. Set supplied when a
// fields query parameter was present; an explicitly empty value is invalid.
func ParseOptional(fields string, supplied bool, schema any) (*Mask, error) {
	root, err := schemaFor(reflect.TypeOf(schema))
	if err != nil {
		return nil, err
	}
	fields = strings.TrimSpace(fields)
	if fields == "" {
		if supplied {
			return nil, invalid("empty mask")
		}
		return &Mask{all: true, schema: root, canon: "*"}, nil
	}
	if fields == "*" {
		return &Mask{all: true, schema: root, canon: "*"}, nil
	}
	if len(fields) > maxInputLength {
		return nil, invalid("mask exceeds maximum length")
	}
	p := parser{input: fields}
	selected, err := p.parseSelection(0, 0)
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	if p.pos != len(p.input) {
		return nil, invalid("unexpected character")
	}
	if len(selected.fields) == 0 && !selected.all {
		return nil, invalid("empty mask")
	}
	if err := validateSelection(selected, root, 0); err != nil {
		return nil, err
	}
	if selected.all {
		return &Mask{all: true, schema: root, canon: "*"}, nil
	}
	return &Mask{fields: selected.fields, schema: root, canon: canonical(selected)}, nil
}

// Canonical returns stable snake_case syntax suitable for cursor scope checks.
func (m *Mask) Canonical() string {
	if m == nil || m.all {
		return "*"
	}
	return m.canon
}

// Project returns JSON with unselected fields removed. Raw JSON values are kept
// intact, so large integers and null/false/zero values retain their representation.
func (m *Mask) Project(value any) (json.RawMessage, error) {
	if m == nil || m.all {
		encoded, err := json.Marshal(value)
		return json.RawMessage(encoded), err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	projected, err := projectValue(encoded, m.schema, &selection{fields: m.fields})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(projected), nil
}

func schemaFor(t reflect.Type) (*objectSchema, error) {
	if t == nil {
		return nil, invalid("schema is nil")
	}
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, invalid("schema must be a struct or a collection of structs")
	}
	return objectFor(t, 0)
}

func objectFor(t reflect.Type, depth int) (*objectSchema, error) {
	if depth > maxDepth {
		return nil, invalid("schema nesting exceeds maximum depth")
	}
	obj := &objectSchema{fields: make(map[string]*schemaField)}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}
		tag := field.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name := strings.Split(tag, ",")[0]
		if name == "" {
			name = lowerFirst(field.Name)
		}
		if name == "" || name == "-" {
			continue
		}
		fieldType := field.Type
		for fieldType.Kind() == reflect.Pointer || fieldType.Kind() == reflect.Slice || fieldType.Kind() == reflect.Array {
			fieldType = fieldType.Elem()
		}
		var child *objectSchema
		if fieldType.Kind() == reflect.Struct && fieldType != reflect.TypeOf(json.RawMessage{}) {
			var err error
			child, err = objectFor(fieldType, depth+1)
			if err != nil {
				return nil, err
			}
		}
		if _, exists := obj.fields[name]; exists {
			return nil, invalid("schema has duplicate JSON field " + name)
		}
		obj.fields[name] = &schemaField{name: name, child: child}
	}
	return obj, nil
}

func validateSelection(sel *selection, schema *objectSchema, depth int) error {
	if sel.all {
		return nil
	}
	if depth > maxDepth {
		return invalid("mask nesting exceeds maximum depth")
	}
	for name, nested := range sel.fields {
		field, ok := schema.fields[name]
		if !ok {
			return invalid("unknown field " + name)
		}
		if nested.requireObject && field.child == nil {
			return invalid("field " + name + " is not an object")
		}
		if nested.all || nested.whole || len(nested.fields) == 0 {
			continue
		}
		if field.child == nil {
			return invalid("field " + name + " is not an object")
		}
		if err := validateSelection(nested, field.child, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func projectValue(raw json.RawMessage, schema *objectSchema, sel *selection) (json.RawMessage, error) {
	if sel == nil || sel.all {
		return raw, nil
	}
	trimmed := bytes.TrimSpace(raw)
	if bytes.Equal(trimmed, []byte("null")) {
		return raw, nil
	}
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var values []json.RawMessage
		if err := json.Unmarshal(trimmed, &values); err != nil {
			return nil, err
		}
		for i := range values {
			projected, err := projectValue(values[i], schema, sel)
			if err != nil {
				return nil, err
			}
			values[i] = projected
		}
		return json.Marshal(values)
	}
	if schema == nil {
		return raw, nil
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &values); err != nil {
		return nil, err
	}
	result := make(map[string]json.RawMessage, len(sel.fields))
	for name, nested := range sel.fields {
		field := schema.fields[name]
		value, exists := values[field.name]
		if !exists {
			continue
		}
		if nested.all || nested.whole || len(nested.fields) == 0 || field.child == nil {
			result[field.name] = value
			continue
		}
		projected, err := projectValue(value, field.child, nested)
		if err != nil {
			return nil, err
		}
		result[field.name] = projected
	}
	return json.Marshal(result)
}

func canonical(sel *selection) string {
	if sel.all {
		return "*"
	}
	keys := make([]string, 0, len(sel.fields))
	for key := range sel.fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		child := sel.fields[key]
		part := key
		if !child.whole && (child.all || len(child.fields) > 0) {
			part += "(" + canonical(child) + ")"
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ",")
}

type parser struct {
	input string
	pos   int
	count int
}

func (p *parser) parseSelection(depth int, terminator byte) (*selection, error) {
	if depth > maxDepth {
		return nil, invalid("mask nesting exceeds maximum depth")
	}
	sel := &selection{fields: make(map[string]*selection)}
	for {
		p.skipSpace()
		name, err := p.readName()
		if err != nil {
			return nil, err
		}
		if name == "*" {
			if len(sel.fields) != 0 {
				return nil, invalid("wildcard cannot be combined with sibling fields")
			}
			sel.all = true
		} else {
			if sel.all {
				return nil, invalid("wildcard cannot be combined with sibling fields")
			}
			target := sel
			for {
				p.count++
				if p.count > maxSelections {
					return nil, invalid("mask has too many fields")
				}
				canonicalName := snakeCase(name)
				next := target.fields[canonicalName]
				if next == nil {
					next = &selection{fields: make(map[string]*selection)}
					target.fields[canonicalName] = next
				}
				target = next
				p.skipSpace()
				if p.pos >= len(p.input) || p.input[p.pos] != '.' {
					break
				}
				p.pos++
				p.skipSpace()
				name, err = p.readName()
				if err != nil {
					return nil, err
				}
				if name == "*" {
					target.whole = true
					target.requireObject = true
					break
				}
			}
			p.skipSpace()
			if p.pos < len(p.input) && p.input[p.pos] == '(' {
				p.pos++
				nested, err := p.parseSelection(depth+1, ')')
				if err != nil {
					return nil, err
				}
				if !target.whole {
					target.merge(nested)
				}
			} else {
				target.whole = true
			}
		}
		p.skipSpace()
		if p.pos >= len(p.input) {
			if terminator != 0 {
				return nil, invalid("unclosed parenthesis")
			}
			return sel, nil
		}
		current := p.input[p.pos]
		if current == ',' {
			p.pos++
			p.skipSpace()
			if p.pos >= len(p.input) || (terminator != 0 && p.input[p.pos] == terminator) {
				return nil, invalid("empty field selection")
			}
			continue
		}
		if terminator != 0 && current == terminator {
			p.pos++
			if len(sel.fields) == 0 && !sel.all {
				return nil, invalid("empty nested selection")
			}
			return sel, nil
		}
		return nil, invalid("expected comma or closing parenthesis")
	}
}

func (s *selection) merge(other *selection) {
	if other.all || other.whole {
		s.whole = true
		s.requireObject = other.all || other.requireObject
		s.fields = nil
		return
	}
	if s.all || s.whole {
		return
	}
	if s.fields == nil {
		s.fields = make(map[string]*selection)
	}
	for name, nested := range other.fields {
		current := s.fields[name]
		if current == nil {
			current = &selection{fields: make(map[string]*selection)}
			s.fields[name] = current
		}
		current.merge(nested)
	}
}

func (p *parser) readName() (string, error) {
	p.skipSpace()
	if p.pos >= len(p.input) {
		return "", invalid("expected field name")
	}
	if p.input[p.pos] == '*' {
		p.pos++
		return "*", nil
	}
	start := p.pos
	for p.pos < len(p.input) {
		r := rune(p.input[p.pos])
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
			break
		}
		p.pos++
	}
	if start == p.pos {
		return "", invalid("expected field name")
	}
	name := p.input[start:p.pos]
	if len(name) > 128 {
		return "", invalid("field name exceeds maximum length")
	}
	return name, nil
}

func (p *parser) skipSpace() {
	for p.pos < len(p.input) && unicode.IsSpace(rune(p.input[p.pos])) {
		p.pos++
	}
}

func snakeCase(name string) string {
	runes := []rune(name)
	var out strings.Builder
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 && runes[i-1] != '_' && (unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1]) || (i+1 < len(runes) && unicode.IsLower(runes[i+1]))) {
				out.WriteByte('_')
			}
			out.WriteRune(unicode.ToLower(r))
		} else {
			out.WriteRune(r)
		}
	}
	return out.String()
}

func lowerFirst(value string) string {
	if value == "" {
		return value
	}
	runes := []rune(value)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

func invalid(detail string) error { return fmt.Errorf("%w: %s", ErrInvalidMask, detail) }
