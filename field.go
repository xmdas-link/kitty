package kitty

import "github.com/fatih/structs"

// Field .
type Field struct {
	field *structs.Field
}

// Field returns the field from a nested struct. It panics if the nested struct
// is not exported or if the field was not found.
func (f *Field) Field(name string) *Field {
	field, ok := f.FieldOk(name)
	if !ok {
		panic("field not found")
	}

	return field
}

// FieldOk returns the field from a nested struct. The boolean returns whether
// the field was found (true) or not (false).
func (f *Field) FieldOk(name string) (*Field, bool) {
	if f, ok := f.field.FieldOk(name); ok {
		return &Field{f}, true
	}
	return nil, false
}
