package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrimVersion(t *testing.T) {
	v := trimVersion("abc.pkg.dev/xyz/x/abc@sha256:cf2337dbf22aab4e4530f5472dbea7845887c6e9416b453ba89d")
	require.Equal(t, "cf2337dbf22a", v)
}
