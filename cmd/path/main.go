package main

import (
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"

	"gcp/lib/ext"

	c "github.com/logrusorgru/aurora/v4"
)

var (
	fPathDupes  = flag.Bool("d", false, "show duplicate paths")
	fExeShadows = flag.Bool("f", false, "show shadowed executables")
)

func main() {
	flag.Parse()

	path := os.Getenv("PATH")

	m := map[string]bool{}
	exes := map[string][]string{}

	for _, v := range strings.Split(path, ":") {
		dir, err := os.ReadDir(v)
		nonexistent := err != nil && os.IsNotExist(err)
		if err != nil && !nonexistent {
			ext.Die("error reading directory: %s", err)
		}
		n := 0
		dupe := m[v]
		for _, d := range dir {
			fi, err := d.Info()
			if err != nil {
				ext.Die("error getting file info: %s", err)
			}
			if d.IsDir() || !(fi.Mode()&0o111 != 0) {
				continue
			}
			n++
			if !dupe {
				exes[d.Name()] = append(exes[d.Name()], v)
			}
		}
		if dupe && !*fPathDupes {
			continue
		}
		if nonexistent {
			fmt.Print(c.CrossedOut(v), " ❌")
		} else {
			fmt.Print(colorizePath(v))
		}
		fmt.Printf(" (%d)", n)
		if *fPathDupes && m[v] {
			fmt.Print(" 🔄")
		}
		fmt.Println()
		m[v] = true
	}

	if *fExeShadows {
		sortedExes := []string{}
		maxSz := 0
		for k := range exes {
			sortedExes = append(sortedExes, k)
			if len(k) > maxSz {
				maxSz = len(k)
			}
		}
		slices.Sort(sortedExes)
		for _, exe := range sortedExes {
			locations := exes[exe]
			if len(locations) == 1 {
				continue
			}
			for i, location := range locations {
				locations[i] = colorizePath(location)
			}
			fmt.Printf("%*s %s\n", maxSz, c.Red(exe), strings.Join(locations, ", "))
		}
	}
}

var homePath string

func home() string {
	homePath, err := os.UserHomeDir()
	if err != nil {
		ext.Die("error getting home directory: %s", err)
	}
	return homePath
}

var hightlights = []string{"/opt/homebrew"}

func colorizePath(location string) string {
	if strings.HasPrefix(location, home()) {
		location = c.BrightWhite(strings.ReplaceAll(location, home()+"/", "~/")).String()
	} else {
		for _, h := range hightlights {
			if strings.Contains(location, h) {
				location = strings.ReplaceAll(location, h, c.Cyan(h).String())
			}
		}
	}
	return location
}

func executable(filename string) (bool, error) {
	info, err := os.Stat(filename)
	if err != nil {
		return false, err
	}
	return info.Mode().Perm()&0o111 != 0, nil
}
