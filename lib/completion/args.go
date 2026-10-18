package completion

import (
	"fmt"
	"strings"
)

type Arg struct {
	Name, Descr string
}

func newArg(name, descr string) Arg {
	return Arg{Name: name, Descr: descr}
}

func NewArg(names, descr string) []Arg {
	args := make([]Arg, 0)
	for _, name := range strings.Split(names, ":") {
		args = append(args, newArg(name, descr))
	}
	return args
}

func (a Arg) String() string {
	return fmt.Sprintf(`"%s":"%s"`, a.Name, a.Descr)
}
