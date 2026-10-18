package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const bashCompletionTestScript = `
function_name=$1
shift
COMP_WORDS=("$@")
COMP_CWORD=$(($# - 1))
"$function_name"
if (( ${#COMPREPLY[@]} )); then
  printf '%s\n' "${COMPREPLY[@]}"
fi
`

const zshCompletionTestScript = `
_arguments() {
  print -r -- "$@"
  print -r -- "$CURRENT:$words[1]"
}
function_name=$1
shift
words=("$@")
CURRENT=$#
"$function_name"
`

func TestDispatch(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "xt")
	build := exec.Command("go", "build", "-o", binary, ".")
	buildOutput, err := build.CombinedOutput()
	require.NoError(t, err, "%s", buildOutput)

	run := func(exe string, args ...string) (string, error) {
		cmd := exec.Command(exe, args...)
		output, err := cmd.CombinedOutput()
		return string(output), err
	}

	for _, args := range [][]string{nil, {"-h"}, {"--help"}, {"help"}} {
		output, err := run(binary, args...)
		require.NoError(t, err)
		require.Contains(t, output, "usage: xt <command>")
	}
	output, err := run(binary, "unknown")
	require.Error(t, err)
	require.Equal(t, 2, err.(*exec.ExitError).ExitCode())
	require.Contains(t, output, "unknown command")

	for name := range commands {
		t.Run(name, func(t *testing.T) {
			link := filepath.Join(dir, name)
			require.NoError(t, os.Symlink("xt", link))

			direct, err := run(binary, name, "-h")
			require.NoError(t, err, direct)
			linked, err := run(link, "-h")
			require.NoError(t, err, linked)
			require.Equal(t, direct, strings.ReplaceAll(linked, link, name))

			_, err = run(binary, name, "-nonexistent-flag")
			require.Error(t, err)
			require.Equal(t, 2, err.(*exec.ExitError).ExitCode())

			if name == "cr" || name == "vm" {
				direct, err = run(binary, name, "completion")
				require.NoError(t, err, direct)
				linked, err = run(link, "completion")
				require.NoError(t, err, linked)
				require.Equal(t, direct, linked)
				require.Contains(t, direct, "#compdef "+name)
			}
		})
	}

	t.Run("bash completion", func(t *testing.T) {
		bashPath, err := exec.LookPath("bash")
		require.NoError(t, err)
		script, err := run(binary, "completion", "bash")
		require.NoError(t, err, script)

		for _, tc := range []struct {
			function string
			words    []string
			want     string
		}{
			{"_xt_bash", []string{"xt", "c"}, "completion\ncr\n"},
			{"_xt_bash", []string{"xt", "cr", "rev"}, "revisions\n"},
			{"_xt_bash", []string{"xt", "vm", "st"}, "start\nstop\n"},
			{"_xt_bash", []string{"xt", "cr", "info", "dep"}, "deploy\n"},
			{"_xt_bash", []string{"xt", "completion", "b"}, "bash\n"},
			{"_xt_bash", []string{"xt", "unknown", ""}, ""},
			{"_xt_bash", []string{"xt", "cr", "no-match"}, ""},
			{"_cr_bash", []string{"cr", "rev"}, "revisions\n"},
			{"_vm_bash", []string{"vm", "st"}, "start\nstop\n"},
		} {
			cmd := exec.Command(bashPath, append([]string{"--noprofile", "--norc", "-s", "--", tc.function}, tc.words...)...)
			cmd.Stdin = strings.NewReader(script + bashCompletionTestScript)
			output, err := cmd.CombinedOutput()
			require.NoError(t, err, "%s", output)
			require.Equal(t, tc.want, string(output), "%v", tc.words)
		}
		for _, name := range []string{"cr", "vm"} {
			direct, err := run(binary, name, "completion", "bash")
			require.NoError(t, err, direct)
			linked, err := run(filepath.Join(dir, name), "completion", "bash")
			require.NoError(t, err, linked)
			require.Equal(t, direct, linked)
			require.Contains(t, script, direct)
			_, err = run(binary, name, "completion", "unknown")
			require.Error(t, err)
			zsh, err := run(binary, name, "completion", "zsh")
			require.NoError(t, err, zsh)
			require.Contains(t, zsh, "#compdef "+name)
		}
	})

	t.Run("zsh completion", func(t *testing.T) {
		zshPath, err := exec.LookPath("zsh")
		require.NoError(t, err)
		script, err := run(binary, "completion", "zsh")
		require.NoError(t, err, script)
		for _, tc := range []struct {
			function string
			words    []string
			want     string
			context  string
		}{
			{"_xt", []string{"xt", ""}, "cr", "2:xt"},
			{"_xt", []string{"xt", "cr", "rev"}, "revisions", "2:cr"},
			{"_xt", []string{"xt", "vm", "st"}, "start", "2:vm"},
			{"_xt", []string{"xt", "cr", "info", "dep"}, "deploy", "3:cr"},
			{"_xt", []string{"xt", "completion", ""}, "zsh", "2:completion"},
			{"_cr", []string{"cr", "rev"}, "revisions", "2:cr"},
			{"_vm", []string{"vm", "st"}, "start", "2:vm"},
		} {
			cmd := exec.Command(zshPath, append([]string{"-f", "-s", "--", tc.function}, tc.words...)...)
			cmd.Stdin = strings.NewReader("autoload -Uz compinit\ncompinit -D\n" + script + zshCompletionTestScript)
			output, err := cmd.CombinedOutput()
			require.NoError(t, err, "%s", output)
			require.Contains(t, string(output), tc.want)
			require.Contains(t, string(output), tc.context)
		}
		for _, args := range [][]string{{"completion"}, {"completion", "fish"}, {"completion", "zsh", "extra"}} {
			output, err := run(binary, args...)
			require.Error(t, err, output)
			require.Contains(t, output, "usage: xt completion <bash|zsh>")
		}
	})

	t.Run("version arguments", func(t *testing.T) {
		project := t.TempDir()
		version := filepath.Join(project, "VERSION.txt")
		require.NoError(t, os.WriteFile(version, []byte("1.2.3\n"), 0o644))
		output, err := run(binary, "ver", "-p", project, "+")
		require.NoError(t, err, output)
		data, err := os.ReadFile(version)
		require.NoError(t, err)
		require.Equal(t, "1.2.4", strings.TrimSpace(string(data)))
	})
}
