package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/BurntSushi/toml"
	c "github.com/logrusorgru/aurora/v4"
	"golang.org/x/mod/semver"

	"gcp/lib/ext"
)

var projectFiles = []string{"VERSION.txt", "package.json", "pyproject.toml"}

type PyProject struct {
	Project struct {
		Version string `toml:"version"`
	} `toml:"project"`
}

type PyPoetryProject struct {
	Tool struct {
		Poetry struct {
			Version string `toml:"version"`
		} `toml:"poetry"`
	} `toml:"tool"`
}

type PackageJSON struct {
	Version string `json:"version"`
}

var (
	flagBase    = flag.String("p", ".", "project directory ('.' by default)")
	flagVerbose = flag.Bool("v", false, "verbose")
)

func main() {
	flag.Usage = func() {
		fmt.Printf("usage: %s [flags] [A.B.C | +]\n", os.Args[0])
		fmt.Println()
		flag.PrintDefaults()
	}

	flag.Parse()

	files := find(*flagBase)
	if *flagVerbose {
		fmt.Println(files)
	}

	for _, name := range files {
		path := path.Join(*flagBase, name)
		content, err := os.ReadFile(path)
		if err != nil {
			ext.Die("reading %s: %s", path, err)
		}
		switch {
		case strings.HasSuffix(name, "VERSION.txt"):
			processVersionTxt(path, content)
		case strings.HasSuffix(name, "package.json"):
			processPackageJSON(path, content)
		case strings.HasSuffix(name, "pyproject.toml"):
			processPyProjectTOML(path, content)
		}
	}
}

func processPyProjectTOML(file string, content []byte) {
	var version string

	var py PyProject
	err := toml.Unmarshal(content, &py)
	if err != nil {
		ext.Die("unmarshaling %s / PEP-621: %s", file, err)
	}
	version = py.Project.Version

	if version == "" {
		var pp PyPoetryProject
		err := toml.Unmarshal(content, &pp)
		if err != nil {
			ext.Die("unmarshaling %s / Poetry: %s", file, err)
		}
		version = pp.Tool.Poetry.Version
	}
	if version == "" {
		ext.Die("version not found in %q (PEP-621 or Poetry formats)", file)
	}
	updateVersionFile(file, version, content)
}

func processPackageJSON(file string, content []byte) {
	if !bytes.Contains(content, []byte("version")) {
		if *flagVerbose {
			fmt.Println(c.BrightRed("(!)"), "version not found in", file)
		}
		return
	}
	var pj PackageJSON
	err := json.Unmarshal(content, &pj)
	if err != nil {
		ext.Die("unmarshaling %s: %s", file, err)
	}
	updateVersionFile(file, pj.Version, content)
}

func processVersionTxt(file string, content []byte) {
	version := strings.TrimSpace(string(content))
	updateVersionFile(file, version, content)
}

func updateVersionFile(filename, version string, content []byte) {
	newVersion := updateVersion(filename, version)
	if newVersion == version {
		return
	}
	b := []byte(strings.Replace(string(content), version, newVersion, 1))
	err := os.WriteFile(filename, b, 0o644)
	if err != nil {
		ext.Die("writing %s: %s", filename, err)
	}
	fmt.Println("written to", c.Cyan(filename))
}

func updateVersion(filename, version string) string {
	fmt.Println("version", c.Yellow(version), "in", c.Cyan(filename))
	if !semver.IsValid("v" + version) {
		ext.Die("invalid version %q in %q", c.Red(version), filename)
	}
	newVersion := flag.Arg(0)
	if newVersion == "" {
		return version
	}
	if newVersion == "+" {
		v := strings.Split(version, ".")
		if len(v) < 3 {
			ext.Die("expected version %q to have 3 parts", c.Red(version))
		}
		newVersion = fmt.Sprintf("%s.%s.%d", v[0], v[1], ext.Atoi(v[2])+1)
	}
	newVersion = strings.TrimPrefix(newVersion, "v")
	if newVersion == "" {
		return version
	}
	if !semver.IsValid("v" + newVersion) {
		ext.Die("invalid version %q", c.Red(newVersion))
	}
	if semver.Compare("v"+version, "v"+newVersion) >= 0 {
		fmt.Println(c.BrightRed("(!)"), newVersion, "<=", version)
	}
	newVersion = semver.Canonical("v" + newVersion)[1:]
	fmt.Println("new version", c.White(newVersion))

	return newVersion
}

var skipDirs = []string{"node_modules"}

func find(fromDir string) (files []string) {
	if fromDir == "" {
		var err error
		fromDir, err = os.Getwd()
		if err != nil {
			ext.Die("getting current directory: %s", err)
		}
	}
	err := fs.WalkDir(os.DirFS(fromDir), ".", func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		skip := false
		for _, dir := range skipDirs {
			if strings.Contains(path, dir) {
				skip = true
				break
			}
		}
		if info.IsDir() || skip {
			return nil
		}
		for _, file := range projectFiles {
			if info.Name() == file {
				if *flagVerbose {
					fmt.Printf("found %s\n", c.Yellow(path))
				}
				files = append(files, path)
			}
		}
		return nil
	})
	if err != nil {
		ext.Die("walking the path %v: %v", fromDir, err)
	}
	return
}
