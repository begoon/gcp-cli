package zsh

import (
	"fmt"
	"os"
	"path"
	"strings"
)

type Arg struct {
	Name, Descr string
}

func NewArg(name, descr string) Arg {
	return Arg{Name: name, Descr: descr}
}

func (a Arg) String() string {
	return fmt.Sprintf(`"%s":"%s"`, a.Name, a.Descr)
}

func Args(args ...Arg) string {
	v := make([]string, len(args))
	for _, a := range args {
		v = append(v, a.String())
	}
	return fmt.Sprintf(`_arguments '*: :((%s))'`, strings.Join(v, " "))
}

func Completion(text string) {
	exe := path.Base(os.Args[0])
	prefix := fmt.Sprintf("_%s_COMPLETE", strings.ToUpper(exe))
	if os.Getenv(prefix) != "complete_zsh" {
		return
	}
	fmt.Print(text)
	os.Exit(0)
}

const format = `
#compdef %[1]s
_%[1]s() {
  eval $(env _%[2]s_COMPLETE_ARGS="${words[2,$CURRENT]}" _%[2]s_COMPLETE=complete_zsh %[1]s)
}
compdef _%[1]s %[1]s
`

func Script() {
	exe := path.Base(os.Args[0])
	script := fmt.Sprintf(format, exe, strings.ToUpper(exe))
	fmt.Print(script)
}
