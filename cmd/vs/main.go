package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"slices"
	"strings"

	c "github.com/logrusorgru/aurora/v4"

	"gcp/lib/ext"
)

var places = []string{"github", "iproov", "vmi", "other"}

func main() {
	verbose := flag.Bool("v", false, "verbose")

	flag.Parse()
	place := flag.Arg(0)

	history := readHistory("places")
	if place == "" {
		places = historyPriority(history, places)
		place = ext.FuzzySelector("where", places)
	}
	updateHistory(place, history, "places")

	if *verbose {
		fmt.Println("PLACE", ext.Color(place, c.White))
	}

	var home string
	var dirs []string

	if place == "vmi" {
		home = strings.TrimSpace(string(ext.Capture("ssh "+place+` pwd`, *verbose)))
		if *verbose {
			fmt.Println("HOME", home)
		}
		cmd := "ssh " + place + " ls -d1 */"
		dirs = strings.Split(string(ext.Capture(cmd, *verbose)), "\n")
	} else {
		home = os.Getenv("HOME")
		cmd := fmt.Sprintf(`sh -c "ls -d1 %s/*/"`, path.Join(home, place))
		dirs = strings.Split(string(ext.Capture(cmd, *verbose)), "\n")
	}

	if *verbose {
		fmt.Println("DIRS")
		fmt.Println(strings.Join(dirs, "\n"))
	}
	dir := selectDir(dirs, home, place)

	if strings.HasSuffix(dir, "/abc/") {
		cmd := fmt.Sprintf(`sh -c "ls -d1 %s/*/"`, path.Join(home, dir))
		dirs = strings.Split(string(ext.Capture(cmd, *verbose)), "\n")
		dir = selectDir(dirs, home, place+"-abc")
	}

	if *verbose {
		fmt.Println("DIR", c.Yellow(dir))
	}

	var cmd string
	if place == "vmi" {
		cmd = fmt.Sprintf("code --remote ssh-remote+%s %s/%s", place, home, dir)
	} else {
		cmd = fmt.Sprintf("code ~/%s", dir)
	}

	ext.Run(cmd)
}

func selectDir(dirs []string, home, place string) string {
	relativeDirs := make([]string, len(dirs))
	for i, dir := range dirs {
		relativeDirs[i] = strings.TrimPrefix(dir, home+"/")
	}
	history := readHistory(place)
	relativeDirs = historyPriority(history, relativeDirs)
	dir := ext.FuzzySelector("which", relativeDirs)
	updateHistory(dir, history, place)
	return dir
}

func historyPriority(history []string, lines []string) []string {
	historyIndex := make(map[string]int)
	for i, h := range history {
		historyIndex[h] = i
	}

	matching := []string{}
	other := []string{}

	for _, line := range lines {
		if _, ok := historyIndex[line]; ok {
			matching = append(matching, line)
			delete(historyIndex, line)
		} else {
			other = append(other, line)
		}
	}

	sortMatching := make([]string, len(history))
	matchIndex := make(map[string]int)
	for i, m := range matching {
		matchIndex[m] = i
	}

	for i, h := range history {
		if _, ok := matchIndex[h]; ok {
			sortMatching[i] = h
		}
	}

	result := []string{}
	for _, s := range sortMatching {
		if s != "" {
			result = append(result, s)
		}
	}
	return append(result, other...)
}

const historyFilePrefix = ".vs_history-"

func historyFilename(filename string) string {
	home, err := os.UserHomeDir()
	ext.Check(err)
	return path.Join(home, historyFilePrefix+filename)
}

func updateHistory(current string, history []string, filename string) {
	filename = historyFilename(filename)
	history = slices.DeleteFunc(history, func(s string) bool {
		return s == current
	})
	history = append([]string{current}, history...)
	ext.Check(os.WriteFile(filename, []byte(strings.Join(history, "\n")+"\n"), 0o644))
}

func readHistory(filename string) []string {
	filename = historyFilename(filename)
	content, err := os.ReadFile(filename)
	if err != nil {
		return []string{}
	}
	return slices.DeleteFunc(strings.Split(string(content), "\n"), func(s string) bool {
		return s == ""
	})
}
