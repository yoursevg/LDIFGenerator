package ldif

type AttributeValue struct {
	Name   string  `json:"name"`
	Values []Value `json:"values"`
}

type Value struct {
	Text   string `json:"text,omitempty"`
	Bytes  []byte `json:"bytes,omitempty"`
	Base64 bool   `json:"base64,omitempty"`
}

type Record struct {
	DN         string           `json:"dn"`
	Attributes []AttributeValue `json:"attributes"`
}

func NewRecord(dn string) Record {
	return Record{DN: dn}
}

func (r *Record) Add(name string, values ...string) {
	if len(values) == 0 {
		return
	}
	vals := make([]Value, 0, len(values))
	for _, v := range values {
		vals = append(vals, Value{Text: v})
	}
	r.Attributes = append(r.Attributes, AttributeValue{Name: name, Values: vals})
}

func (r *Record) AddBinary(name string, values ...[]byte) {
	if len(values) == 0 {
		return
	}
	vals := make([]Value, 0, len(values))
	for _, v := range values {
		vals = append(vals, Value{Bytes: v, Base64: true})
	}
	r.Attributes = append(r.Attributes, AttributeValue{Name: name, Values: vals})
}
