package main

// CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/briandowns/spinner"

	"github.com/bitfield/script"
	c "github.com/logrusorgru/aurora/v4"
)

func main() {
	flag.Parse()

	for _, cmd := range flag.Args() {
		switch cmd {
		case "h", "health":
			health()

		case "r", "revisions":
			revisions()

		case "w", "wait":
			wait()

		case "i", "info":
			info()

		case "d", "deploy":
			deploy()

		case "b", "bounce":
			bounce()

		case "m", "metadata":
			metadata()

		default:
			die("unknown command: %s", cmd)
		}
	}
}

func describeCmd(service, project, region string) string {
	return T(fmt.Sprintf(
		"gcloud run services describe %s --region %s --project %s --format json",
		service, region, project))
}

func serviceInfo(service, project, region string) Service {
	s := Service{}
	check(json.Unmarshal(capture(describeCmd(service, project, region), true), &s))
	return s
}

func imagesCmd(image string) string {
	return T(fmt.Sprintf(``+
		`gcloud artifacts docker images list %s `+
		`--include-tags --sort-by "~CREATE_TIME" --format json --limit 10`,
		image))
}

func images(echo bool) []Image {
	b := capture(imagesCmd(IMAGE()), echo)
	images := []Image{}
	check(json.Unmarshal(b, &images))
	return images
}

func textualizeVersions(images []Image) []string {
	versions := []string{}
	for _, image := range images {
		versions = append(versions, formatVersion(image))
	}
	return versions
}

func die(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
	os.Exit(1)
}

func T(v string) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(v, "\n", " "), "  ", " "))
}

func exec(cmd string, echo bool) *script.Pipe {
	if echo {
		fmt.Println("\n" + color(cmd, c.White))
	}
	p := script.Exec(cmd).WithStderr(stderr)
	if CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE() != "" {
		env := []string{"CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE=" + CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE()}
		p = p.WithEnv(env)
		fmt.Println("override " + color("CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE", c.Magenta))
	}
	return p
}

func capture(cmd string, echo bool) []byte {
	b, err := exec(cmd, echo).Bytes()
	check(err)
	return b
}

func run(cmd string) {
	_, err := exec(cmd, true).Stdout()
	check(err)
}

func quiet(cmd string) {
	_, err := exec(cmd, false).Stdout()
	check(err)
}

func runJQ(cmd string, q string) {
	_, err := exec(cmd, true).JQ(q).Stdout()
	check(err)
}

var stderr = new(bytes.Buffer)

func check(err error) {
	if err != nil {
		if stderr.Len() > 0 {
			fmt.Println("stderr:", stderr.String())
		}
		die("error: %v", err)
	}
	stderr.Reset()
}

type Service struct {
	Metadata struct {
		Annotations map[string]string `json:"annotations"`
	} `json:"metadata"`
	Spec struct {
		Template struct {
			Spec struct {
				Containers []struct {
					Image string `json:"image"`
				} `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
	Status struct {
		Address struct {
			URL string `json:"url"`
		} `json:"address"`
	} `json:"status"`
	URL string `json:"url"`
}

type Image struct {
	UpdateTime string   `json:"updateTime"`
	CreateTime string   `json:"createTime"`
	Package    string   `json:"package"`
	Tags       []string `json:"tags"`
	Metadata   struct {
		ImageSizeBytes string `json:"imageSizeBytes"`
	} `json:"metadata"`
	Version string `json:"version"`
}

func selectImage(images []Image, current string) (int, string) {
	if current != "" {
		fmt.Println("running", color(current, c.Magenta))
	}

	imagesSelector := []string{}
	for _, image := range images {
		t := formatVersion(image)

		matched := false
		for _, tag := range image.Tags {
			imageID := image.Package + ":" + tag
			if imageID == current {
				matched = true
				break
			}
		}
		if !matched {
			imageID := image.Package + "@" + image.Version
			matched = imageID == current
		}
		if matched {
			t += color(" (running)", c.White)
		}
		imagesSelector = append(imagesSelector, t)
	}

	prompt := &survey.Select{Message: "image", Options: imagesSelector}

	var selection string
	err := survey.AskOne(prompt, &selection, survey.WithValidator(survey.Required))
	check(err)
	fmt.Println(selection)

	for i, v := range imagesSelector {
		if v != selection {
			continue
		}
		selectedImage := images[i]
		if len(selectedImage.Tags) == 0 {
			return i, selectedImage.Version
		}
		if slices.Contains(selectedImage.Tags, "latest") {
			return i, "latest"
		}
		return i, selectedImage.Tags[0]
	}
	die("image not found: %s\n", selection)
	return 0, ""
}

func formatVersion(i Image) string {
	t, err := time.Parse(time.RFC3339, i.CreateTime)
	check(err)

	// since := HumanizeDuration(time.Since(t))
	datetime := t.Format(time.DateTime)
	version := trimVersion(i.Version)
	size := humanizeSize(atoi(i.Metadata.ImageSizeBytes))
	tags := i.Version
	if len(i.Tags) > 0 {
		tags = strings.Join(i.Tags, ", ")
	}
	return fmt.Sprintf("%s | %v | %s | %v", datetime, version, size, tags)
}

func trimVersion(v string) string {
	return strings.Split(v, ":")[1][0:12]
}

func atoi(bytes string) int {
	n, err := strconv.Atoi(bytes)
	if err != nil {
		die("error converting string to int: %s", err)
	}
	return n
}

func humanizeSize(bytes int) string {
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

func color(text string, colorizer colorFunc) string {
	return colorizer(text).Bold().String()
}

func confirm(message string) bool {
	fmt.Println()

	yes := true
	prompt := &survey.Confirm{Message: message, Default: yes}
	err := survey.AskOne(prompt, &yes)
	if err != nil {
		die("error asking question: %s", err)
	}
	return yes
}

type ImageParts struct {
	Repo, Host, Project, Location, Prefix, Name, Tag string
}

func parseImage(image string) ImageParts {
	v := ImageParts{}

	parts := strings.Split(image, ":") // repo:tag
	v.Repo = parts[0]
	v.Tag = parts[1]

	parts = strings.Split(v.Repo, "/") // host/project/location/prefix/name
	v.Host = parts[0]
	v.Project = parts[1]
	v.Prefix = parts[2]
	v.Name = parts[3]
	return v
}

const console = "https://console.cloud.google.com"

func registryLink(image string) string {
	parts := strings.Split(image, ":")
	repo := strings.Split(parts[0], "/")
	host := strings.Split(repo[0], "-")
	project := repo[1]
	location := host[0]
	prefix := repo[2]
	name := repo[3]
	link := fmt.Sprintf(
		"%s/artifacts/docker/%s/%s/%s/%s?project=%s",
		console, project, location, prefix, name, project,
	)
	return link
}

func serviceLink(project, region, service string) string {
	link := fmt.Sprintf(
		"%s/run/detail/%s/%s/revisions?project=%s",
		console, region, service, project)
	return link
}

func href(link, text string) string {
	return fmt.Sprintf("\u001b]8;;%s\u001b\\%s\u001b]8;;\u001b\\", link, text)
}

func notify(msg string) {
	quiet(fmt.Sprintf("osascript -e 'display notification \"%s\" with title \"OK\"'", msg))
	quiet(fmt.Sprintf("say \"%s\"", msg))
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
	return variables["PROJECT"]
}

func SERVICE() string {
	v := variables["SERVICE_NAME"]
	if v != "" {
		return v
	}
	v = variables["SERVICE_NAMES"]
	if v == "" {
		die("missing SERVICE_NAME or SERVICE_NAMES")
	}
	services := strings.Split(v, ",")
	if len(services) < 2 {
		return services[0]
	}
	prompt := &survey.Select{Message: "service", Options: services}
	var selection string
	err := survey.AskOne(prompt, &selection, survey.WithValidator(survey.Required))
	check(err)
	return selection
}

func REGION() string {
	return variables["REGION"]
}

func IMAGE() string {
	return variables["REPO"] + "/" + variables["NAME"]
}

func CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE() string {
	return variables["CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE"]
}

// ---

func init() {
	files := []string{".env"}
	fmt.Println("variables", files)
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err == nil {
			parseVariables(string(content), variables)
		}
	}
	b, err := json.MarshalIndent(variables, "", "  ")
	check(err)
	fmt.Println(string(b))
}

// ---

func deployCmd(service, image, project, region string) string {
	return T(fmt.Sprintf(
		"gcloud run deploy %s --image %s --region %s --project=%s",
		service, image, region, project))
}

func deploy() {
	serviceName := SERVICE()
	service := serviceInfo(serviceName, PROJECT(), REGION())

	urls := strings.Split(service.Metadata.Annotations["run.googleapis.com/urls"], ",")
	fmt.Println(strings.Join(urls, ", "))

	currentImage := service.Spec.Template.Spec.Containers[0].Image
	fmt.Println("image", color(currentImage, c.Yellow))
	if strings.Contains(currentImage, "/") {
		// If the image is fully qualified, print it as a link
		fmt.Println(registryLink(currentImage))
	}

	images := images(true)

	index, version := selectImage(images, currentImage)
	fmt.Println(">", version)
	delimiter := ":"
	if strings.HasPrefix(version, "sha256") {
		delimiter = "@"
	}
	image := images[index].Package + delimiter + version

	if !confirm(fmt.Sprintf("deploy [%s]", color(image, c.Yellow))) {
		return
	}

	cmd := deployCmd(serviceName, image, PROJECT(), REGION())
	run(cmd)

	notify("deployed")
}

func bounce() {
	health()

	serviceName := SERVICE()
	service := serviceInfo(serviceName, PROJECT(), REGION())

	fmt.Println("service", color(serviceLink(PROJECT(), REGION(), serviceName), c.Blue))

	image := service.Spec.Template.Spec.Containers[0].Image
	fmt.Println("image", color(image, c.Yellow))
	fmt.Println(registryLink(image))

	if !confirm(fmt.Sprintf("bounce [%s]", color(image, c.Yellow))) {
		return
	}

	cmd := deployCmd(serviceName, image, PROJECT(), REGION())
	cmd += " --update-env-vars BOUNCED=" + time.Now().Format(time.RFC3339)
	run(cmd)

	notify("bounced")
}

func metadata() {
	images := images(true)

	index, version := selectImage(images, "")
	fmt.Println(">", version)
	sep := ":"
	if strings.HasPrefix(version, "sha256") {
		sep = "@"
	}
	image := images[index].Package + sep + version

	cmd := "skopeo inspect docker://" + image
	runJQ(cmd, ".Name, .Digest, .Architecture, .Env")
}

func info() {
	serviceName := SERVICE()
	service := serviceInfo(serviceName, PROJECT(), REGION())

	fmt.Println("service", color(serviceLink(PROJECT(), REGION(), serviceName), c.Blue))

	urls := strings.Split(service.Metadata.Annotations["run.googleapis.com/urls"], ",")
	fmt.Println("urls", color(strings.Join(urls, ", "), c.Magenta))

	currentImage := service.Spec.Template.Spec.Containers[0].Image
	fmt.Println("image", color(currentImage, c.Yellow))
	if strings.Contains(currentImage, "/") {
		// If the image is fully qualified, print it as a link.
		fmt.Println(registryLink(currentImage))
	}
	health()
}

func health() {
	service := serviceInfo(SERVICE(), PROJECT(), REGION())
	url := service.Status.Address.URL + "/health"

	fmt.Println("\n" + color("GET ", c.Blue) + color(url, c.White))
	script.Get(url).Stdout()
}

func revisions() {
	images := images(true)
	for _, version := range textualizeVersions(images) {
		fmt.Println(version)
	}
}

func wait() {
	lastImage := images(true)[0]
	fmt.Println(">", formatVersion(lastImage))

	s := spinner.New(
		spinner.CharSets[14], 100*time.Millisecond,
		spinner.WithHiddenCursor(false),
	)
	s.Start()
	for {
		time.Sleep(1 * time.Second)
		image := images(false)[0]
		if lastImage.Version != image.Version {
			s.Stop()
			fmt.Println(color(formatVersion(image), c.Yellow))
			break
		}
	}
	notify("new revision is pushed")
}
