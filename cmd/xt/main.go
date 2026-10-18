package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gcp/internal/cr"
	pathcmd "gcp/internal/path"
	"gcp/internal/ver"
	"gcp/internal/vm"
	"gcp/internal/vs"
	"gcp/lib/completion/bash"
	"gcp/lib/completion/zsh"
)

var commands = map[string]func(){
	"cr":   cr.Run,
	"vm":   vm.Run,
	"ver":  ver.Run,
	"vs":   vs.Run,
	"path": pathcmd.Run,
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: xt <command> [arguments...]")
	fmt.Fprintln(w, "\ncommands:")
	fmt.Fprintln(w, "  cr    Cloud Run services and registry images")
	fmt.Fprintln(w, "  vm    Compute Engine VMs and SSH configuration")
	fmt.Fprintln(w, "  ver   Project versions")
	fmt.Fprintln(w, "  vs    Open projects in VS Code")
	fmt.Fprintln(w, "  path  Inspect PATH and executables")
	fmt.Fprintln(w, "  completion <bash|zsh>  Generate shell completion script")
}

func main() {
	name := filepath.Base(os.Args[0])
	if _, ok := commands[name]; !ok {
		if len(os.Args) == 1 || os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "help" {
			usage(os.Stdout)
			return
		}
		if os.Args[1] == "completion" {
			if len(os.Args) != 3 || (os.Args[2] != "bash" && os.Args[2] != "zsh") {
				fmt.Fprintln(os.Stderr, "usage: xt completion <bash|zsh>")
				os.Exit(2)
			}
			if os.Args[2] == "zsh" {
				fmt.Print(zsh.Suite("xt", map[string]string{
					"cr": cr.CompletionRoot, "vm": vm.CompletionRoot,
					"ver": "", "vs": "", "path": "",
					"completion": zsh.Args(zsh.NewArg("bash:zsh", "shell")),
				}, "cr", "vm"))
				return
			}
			fmt.Print(bash.Suite("xt", map[string]string{
				"cr":  bash.Args(cr.CompletionArgs...),
				"vm":  bash.Args(vm.CompletionArgs...),
				"ver": "", "vs": "", "path": "",
				"completion": "bash zsh",
			}, bash.Script("cr", cr.CompletionArgs...), bash.Script("vm", vm.CompletionArgs...)))
			return
		}
		name = os.Args[1]
		if _, ok := commands[name]; !ok {
			fmt.Fprintf(os.Stderr, "xt: unknown command %q\n\n", name)
			usage(os.Stderr)
			os.Exit(2)
		}
		os.Args = os.Args[1:]
	}
	commands[name]()
}
