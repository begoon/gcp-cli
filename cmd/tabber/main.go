package main

import (
	"fmt"
	"os"
	"text/tabwriter"
)

func main() {
	w := tabwriter.NewWriter(os.Stdout, 10, 0, 0, ' ', tabwriter.AlignRight|tabwriter.Debug)
	fmt.Fprintln(w, "a\tb\tc")
	fmt.Fprintln(w, "aa\tbb\tcc")
	fmt.Fprintln(w, "aaa\t")
	fmt.Fprintln(w, "aaaa\tdddd\teeee")
	fmt.Fprintln(w, "12345678901234567890\t12345678901234567890\teeee")
	w.Flush()
}
