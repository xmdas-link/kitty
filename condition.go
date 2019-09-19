package kitty

// SearchCondition ...
type SearchCondition struct {
	FormValues  map[string][]string `json:"form_values,omitempty"`
	Page        *Page               `json:"page,omitempty"`
	ReturnCount int                 `json:"return_count,omitempty"`
}

/*
// CheckParamValid ...
func (s *SearchCondition) CheckParamValid(model string) error {
	ss := NewModelStruct(model)
	for k := range s.FormValues {
		f, ok := ss.FieldOk(ToCamel(k))
		if !ok {
			return fmt.Errorf("invalid param %s", k)
		}
		ki := f.Tag("kitty")
		if !strings.Contains(ki, "param:") || strings.Contains(ki, "-;") {
			return fmt.Errorf("invalid param %s", k)
		}
	}
	return nil
}
*/
