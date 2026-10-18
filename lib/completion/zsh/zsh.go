package zsh

import (
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	"gcp/lib/completion"
)

type Arg = completion.Arg

var NewArg = completion.NewArg

func Args(args ...[]Arg) string {
	v := make([]string, 0, len(args))
	for _, a := range args {
		for _, p := range a {
			v = append(v, p.String())
		}
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

const format = `#compdef %[1]s

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

const suiteHeader = `#compdef %[1]s

_%[1]s() {
  if (( CURRENT == 2 )); then
    %[2]s
    return
  fi
  local command=$words[2]
  words=("${words[@]:1}")
  (( CURRENT-- ))
  case "$command" in
`

// suite generates completion functions without requiring command symlinks
func Suite(name string, commands map[string]string, standalone ...string) string {
	var names []string
	for command := range commands {
		names = append(names, command)
	}
	sort.Strings(names)
	var root []Arg
	for _, command := range names {
		root = append(root, Arg{Name: command})
	}
	var out strings.Builder
	fmt.Fprintf(&out, suiteHeader, name, Args(root))
	for _, command := range names {
		if commands[command] != "" {
			fmt.Fprintf(&out, "    %s) %s ;;\n", command, commands[command])
		}
	}
	fmt.Fprintf(&out, "  esac\n}\ncompdef _%s %s\n", name, name)
	for _, command := range standalone {
		fmt.Fprintf(&out, "\n_%s() {\n  %s\n}\ncompdef _%s %s\n", command, commands[command], command, command)
	}
	return out.String()
}
