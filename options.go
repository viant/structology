package structology

//Option marker option
type Option func(m *Marker)

//Options represents marker option
type Options []Option

//Apply applies options
func (o Options) Apply(m *Marker) {
	if len(o) == 0 {
		return
	}
	for _, opt := range o {
		opt(m)
	}
}

//WithIndex filed name to index mapping
func WithIndex(index map[string]int) Option {
	return func(m *Marker) {
		m.index = index
	}
}
