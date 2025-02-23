package main

import (
	"gcp/lib/ext"
)

func main() {
	ext.Selector("pick one", []string{"a", "b", "c"})
}
