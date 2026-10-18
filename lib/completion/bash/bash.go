package bash

import (
	"fmt"
	"sort"
	"strings"

	"gcp/lib/completion"
)

func Args(groups ...[]completion.Arg) string {
	var names []string
	for _, group := range groups {
		for _, arg := range group {
			names = append(names, arg.Name)
		}
	}
	return strings.Join(names, " ")
}

func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// script names are fixed command identifiers supplied by the application
func Script(name string, groups ...[]completion.Arg) string {
	return fmt.Sprintf(`_%[1]s_bash() {
  local word
  COMPREPLY=()
  while IFS= read -r word; do
    COMPREPLY+=("$word")
  done < <(compgen -W %[2]s -- "${COMP_WORDS[COMP_CWORD]}")
}
complete -F _%[1]s_bash %[1]s
`, name, quote(Args(groups...)))
}

// suite registers the dispatcher and its supported standalone commands
func Suite(name string, commands map[string]string, scripts ...string) string {
	var names []string
	for command := range commands {
		names = append(names, command)
	}
	sort.Strings(names)
	var out strings.Builder
	for _, script := range scripts {
		out.WriteString(script)
	}
	fmt.Fprintf(&out, `_%s_bash() {
  local word choices
  COMPREPLY=()
  if (( COMP_CWORD == 1 )); then
    choices=%s
  else
    case "${COMP_WORDS[1]}" in
`, name, quote(strings.Join(names, " ")))
	for _, command := range names {
		fmt.Fprintf(&out, "      %s) choices=%s ;;\n", command, quote(commands[command]))
	}
	fmt.Fprintf(&out, `      *) return ;;
    esac
  fi
  while IFS= read -r word; do
    COMPREPLY+=("$word")
  done < <(compgen -W "$choices" -- "${COMP_WORDS[COMP_CWORD]}")
}
complete -F _%s_bash %s
`, name, name)
	return out.String()
}
