package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

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
