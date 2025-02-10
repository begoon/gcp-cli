package ext

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"

	"github.com/bitfield/script"
	c "github.com/logrusorgru/aurora/v4"
)

func Die(format string, args ...interface{}) {
	fmt.Println(Color(fmt.Sprintf(format+"\n", args...), c.Red))
	os.Exit(1)
}

func Exec(cmd string, echo bool) *script.Pipe {
	if echo {
		fmt.Println("\n" + Color(cmd, c.White))
	}
	p := script.Exec(cmd).WithStderr(stderr)
	if CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE() != "" {
		env := []string{"CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE=" + CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE()}
		p = p.WithEnv(env)
		fmt.Println("override", Color("CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE", c.Magenta))
	}
	return p
}

func Capture(cmd string, echo bool) []byte {
	b, err := Exec(cmd, echo).Bytes()
	Check(err, cmd)
	return b
}

func Run(cmd string) {
	_, err := Exec(cmd, true).WithStderr(os.Stdout).Stdout()
	Check(err, cmd)
}

func Quiet(cmd string, retries ...int) {
	n := 1
	if len(retries) > 0 {
		n = retries[0]
	}
	var err error
	for i := 0; i < n; i++ {
		_, err = Exec(cmd, false).Stdout()
		if err == nil {
			return
		}
		time.Sleep(1 * time.Second)
	}
	fmt.Println(Color(cmd, c.Red))
	Check(err, fmt.Sprintf("failed after %d attempts", n))
}

func RunJQ(cmd string, q string) {
	_, err := Exec(cmd, true).JQ(q).Stdout()
	Check(err, cmd)
}

var stderr = new(bytes.Buffer)

func Check(err error, extra ...string) {
	if err != nil {
		if stderr.Len() > 0 {
			fmt.Println("stderr:", stderr.String())
		}
		Die("error: %v [%s]", err, strings.Join(extra, " "))
	}
	stderr.Reset()
}

func Atoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		Die("error converting string to int: %s", err)
	}
	return n
}

func HumanizeSize(bytes int) string {
	const KB = 1024
	const MB = KB * 1024
	const GB = MB * 1024

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}

type colorFunc func(arg interface{}) c.Value

func Color(text string, colorizer colorFunc) string {
	return colorizer(text).Bold().String()
}

func Confirm(message string) bool {
	fmt.Println()

	yes := true
	prompt := &survey.Confirm{Message: message, Default: yes}
	err := survey.AskOne(prompt, &yes)
	if err != nil {
		Die("error asking question: %s", err)
	}
	return yes
}

const ConsoleURL = "https://console.cloud.google.com"

func Href(link, text string) string {
	return fmt.Sprintf("\u001b]8;;%s\u001b\\%s\u001b]8;;\u001b\\", link, text)
}

func Notify(msg string) {
	Quiet(fmt.Sprintf("osascript -e 'display notification \"%s\" with title \"OK\"'", msg))
	Quiet(fmt.Sprintf("say \"%s\"", msg))
}

func parseVariables(content string, values map[string]string) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		c := line[0:1]
		if strings.Contains("#; \t", c) {
			continue
		}
		parts := strings.Split(line, "=")
		if len(parts) < 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if name == "" || value == "" {
			continue
		}
		values[name] = value
	}
}

// ---

var variables = map[string]string{}

func PROJECT() string {
	return v("PROJECT")
}

func SERVICE() string {
	v := variables["SERVICE_NAME"]
	if v != "" {
		return v
	}
	v = variables["SERVICE_NAMES"]
	if v == "" {
		v = variables["SERVICE"]
		if v == "" {
			Die("missing SERVICE, SERVICE_NAME or SERVICE_NAMES")
		}
	}
	services := strings.Split(v, ",")
	if len(services) < 2 {
		return services[0]
	}
	prompt := &survey.Select{Message: "service", Options: services}
	var selection string
	err := survey.AskOne(prompt, &selection, survey.WithValidator(survey.Required))
	Check(err)
	variables["SERVICE"] = selection
	return selection
}

func REGION() string {
	return v("REGION")
}

func IMAGE() string {
	return REPO() + "/" + NAME()
}

func NAME() string {
	return v("NAME")
}

func REPO() string {
	return v("REPO")
}

func v(name string) string {
	v := variables[name]
	if v == "" {
		Die("missing %s", name)
	}
	return v
}

func CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE() string {
	return variables["CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE"]
}

// ---

func LoadVariables() {
	files := []string{".cr", ".env", "Makefile"}
	fmt.Println("variables = [", strings.Join(files, ", "), "]")

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err == nil {
			parseVariables(string(content), variables)
		}
	}
	v := map[string]string{
		"PROJECT": PROJECT(),
		"REGION":  REGION(),
		"IMAGE":   IMAGE(),
	}
	if CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE() != "" {
		v["CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE"] = CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE()
	}
	if t := variables["SERVICE"]; t != "" {
		v["SERVICE"] = t
	}
	if t := variables["SERVICE_NAME"]; t != "" {
		v["SERVICE_NAME"] = t
	}
	if t := variables["SERVICE_NAMES"]; t != "" {
		v["SERVICE_NAMES"] = t
	}
	b, err := json.MarshalIndent(v, "", "  ")
	Check(err)
	fmt.Println(string(b))
}
