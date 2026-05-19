package conv

import "strings"

type viewBuilder struct {
	b strings.Builder
}

func (v *viewBuilder) writeString(s string) {
	v.b.WriteString(s)
}

func (v *viewBuilder) String() string {
	return v.b.String()
}

func (v *viewBuilder) len() int {
	return v.b.Len()
}
