package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"gcp/lib/completion/zsh"
	"gcp/lib/ext"
	"io"
	"io/fs"
	"os"
	"path"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/briandowns/spinner"

	"github.com/bitfield/script"
	c "github.com/logrusorgru/aurora/v4"
)

func main() {
	zsh.Completion(CompletionRoot)

	flag.Usage = func() {
		fmt.Printf("usage: %s command [command]...", os.Args[0])
		fmt.Println("commands:")
		fmt.Println("  h, health      /health")
		fmt.Println("  r, revisions   list revisions")
		fmt.Println("  w, wait        wait for new iamge revision")
		fmt.Println("  i, info        show service info")
		fmt.Println("  d, deploy      deploy a revision (default)")
		fmt.Println("  b, bounce      bounce the service")
		fmt.Println("  c, create      create a new service")
		fmt.Println("  m, metadata    show image metadata")
		fmt.Println("  t, terraform   cross-reference terraform")
		fmt.Println("  v, variables   show environment variables and secrets")
		fmt.Println("  init           create .cr file")
		fmt.Println("  completion     generate completion script")
		fmt.Println()
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		args = []string{"d"}
	}

	if args[0] == "completion" {
		zsh.Script()
		os.Exit(0)
	}

	ext.LoadVariables()

	for _, cmd := range args {
		switch cmd {
		case "h", "health":
			healthCmd()

		case "r", "revisions":
			revisionsCmd()

		case "w", "wait":
			waitCmd()

		case "i", "info":
			infoCmd()

		case "d", "deploy":
			deployCmd()

		case "b", "bounce":
			bounceCmd()

		case "c", "create":
			createCmd()

		case "m", "metadata":
			metadataCmd()

		case "t", "terraform":
			terraformCmd()

		case "v", "variables":
			variablesCmd()

		case "init":
			initCmd()

		case "x":
			i, _ := debug.ReadBuildInfo()
			fmt.Println(i)
			os.Exit(0)

		default:
			ext.Die("unknown command: %s", cmd)
		}
	}
}

func queryService(service, project, region string) (b []byte, err error) {
	cmd := fmt.Sprintf(
		"gcloud run services describe %s --region %s --project %s --format json",
		service, region, project)
	return ext.Exec(cmd, true).Bytes()
}

func serviceExists(service, project, region string) bool {
	b, err := queryService(service, project, region)

	fmt.Println(string(b))
	return err == nil
}

func serviceInfo(service, project, region string) (s Service) {
	b, err := queryService(service, project, region)
	ext.Check(err)

	ext.Check(json.Unmarshal(b, &s))
	return s
}

func queryImages(echo bool) []Image {
	cmd := fmt.Sprintf(``+
		`gcloud artifacts docker images list %s `+
		`--include-tags --sort-by "~CREATE_TIME" --format json --limit 10`,
		ext.IMAGE())
	b := ext.Capture(cmd, echo)
	images := []Image{}
	ext.Check(json.Unmarshal(b, &images))
	return images
}

func textualizeVersions(images []Image) []string {
	versions := []string{}
	sizeWidth := maxSizeWidth(images)
	for _, image := range images {
		versions = append(versions, formatVersion(image, sizeWidth))
	}
	return versions
}

type Service struct {
	Metadata struct {
		Annotations map[string]string `json:"annotations"`
	} `json:"metadata"`
	Spec struct {
		Template struct {
			Metadata struct {
				Annotations map[string]string `json:"annotations"`
			} `json:"metadata"`
			Spec struct {
				ServiceAccountName string `json:"serviceAccountName"`
				Containers         []struct {
					Image string `json:"image"`
					Env   []struct {
						Name      string `json:"name"`
						Value     string `json:"value,omitempty"`
						ValueFrom *struct {
							SecretKeyRef struct {
								Key  string `json:"key"`
								Name string `json:"name"`
							} `json:"secretKeyRef"`
						} `json:"valueFrom,omitempty"`
					} `json:"env"`
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
		fmt.Println("running", ext.Color(current, c.Magenta))
	}

	imagesSelector := []string{}
	sizeWidth := maxSizeWidth(images)
	for _, image := range images {
		t := formatVersion(image, sizeWidth)

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
			t += ext.Color(" (running)", c.White)
		}
		imagesSelector = append(imagesSelector, t)
	}

	prompt := &survey.Select{Message: "image", Options: imagesSelector}

	var selection string
	err := survey.AskOne(prompt, &selection, survey.WithValidator(survey.Required))
	ext.Check(err)
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
	ext.Die("image not found: %s\n", selection)
	return 0, ""
}

func maxSizeWidth(images []Image) int {
	sz := 0
	for _, image := range images {
		if image.Metadata.ImageSizeBytes != "" {
			sz = max(sz, len(ext.HumanizeSize(ext.Atoi(image.Metadata.ImageSizeBytes))))
		}
	}
	return sz
}

func formatVersion(i Image, minSizeWidth int) string {
	t, err := time.Parse(time.RFC3339, i.CreateTime)
	ext.Check(err)

	datetime := t.Format(time.DateTime)
	version := trimVersion(i.Version)

	size := ""
	if i.Metadata.ImageSizeBytes != "" {
		size = ext.HumanizeSize(ext.Atoi(i.Metadata.ImageSizeBytes))
	}
	if len(size) < minSizeWidth {
		size = strings.Repeat(" ", minSizeWidth-len(size)) + size
	}

	tags := i.Version
	if len(i.Tags) > 0 {
		tags = strings.Join(i.Tags, ", ")
	}

	return fmt.Sprintf("%s | %v | %s | %v", datetime, version, size, tags)
}

func trimVersion(v string) string {
	return strings.Split(v, ":")[1][0:12]
}

type ImageParts struct {
	Repo, Host, Project, Location, Prefix, Name, Tag string
}

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
		ext.ConsoleURL, project, location, prefix, name, project,
	)
	return link
}

func serviceLink(project, region, service string) string {
	link := fmt.Sprintf(
		"%s/run/detail/%s/%s/revisions?project=%s",
		ext.ConsoleURL, region, service, project)
	return link
}

// ---

func deploy(service, image, project, region string) string {
	return fmt.Sprintf(
		"gcloud run deploy %s --image %s --region %s --project=%s",
		service, image, region, project)
}

func deployCmd() {
	serviceName := ext.SERVICE()
	service := serviceInfo(serviceName, ext.PROJECT(), ext.REGION())

	urls := strings.Split(service.Metadata.Annotations["run.googleapis.com/urls"], ",")
	fmt.Println(strings.Join(urls, ", "))

	currentImage := service.Spec.Template.Spec.Containers[0].Image
	fmt.Println("image", ext.Color(currentImage, c.Yellow))
	if strings.Contains(currentImage, "/") {
		// If the image is fully qualified, print it as a link.
		// This check is necessary because the image can be malformed
		// if the last deployment was attempted with a malformed image.
		fmt.Println(registryLink(currentImage))
	}

	images := queryImages(true)

	index, version := selectImage(images, currentImage)
	fmt.Println(">", version)
	delimiter := ":"
	if strings.HasPrefix(version, "sha256") {
		delimiter = "@"
	}
	image := images[index].Package + delimiter + version

	if !ext.Confirm(fmt.Sprintf("deploy [%s]", ext.Color(image, c.Yellow))) {
		return
	}

	cmd := deploy(serviceName, image, ext.PROJECT(), ext.REGION())
	ext.Run(cmd)

	ext.Notify("deployed")
}

func bounceCmd() {
	healthCmd()

	serviceName := ext.SERVICE()
	service := serviceInfo(serviceName, ext.PROJECT(), ext.REGION())

	fmt.Println("service", ext.Color(serviceLink(ext.PROJECT(), ext.REGION(), serviceName), c.Blue))

	image := service.Spec.Template.Spec.Containers[0].Image
	fmt.Println("image", ext.Color(image, c.Yellow))
	fmt.Println(registryLink(image))

	if !ext.Confirm(fmt.Sprintf("bounce [%s]", ext.Color(image, c.Yellow))) {
		return
	}

	cmd := deploy(serviceName, image, ext.PROJECT(), ext.REGION())
	cmd += " --update-env-vars BOUNCED=" + time.Now().Format(time.RFC3339)
	ext.Run(cmd)

	ext.Notify("bounced")
}

var (
	fStub   = flag.String("stub", "", "stub image for new service")
	fExpand = flag.Bool("x", false, "expand secret values")
)

func createCmd() {
	serviceName := ext.SERVICE()
	if serviceExists(serviceName, ext.PROJECT(), ext.REGION()) {
		ext.Die("service already exists: %s", serviceName)
	}

	image := *fStub

	if *fStub == "" {
		images := queryImages(true)

		index, version := selectImage(images, "UNDEFINED")
		fmt.Println(">", version)
		delimiter := ":"
		if strings.HasPrefix(version, "sha256") {
			delimiter = "@"
		}
		image = images[index].Package + delimiter + version
	} else {
		_, _, fqn := strings.Cut(image, "/")
		if !fqn {
			image = ext.REPO() + "/" + image
		}
	}

	if !ext.Confirm(fmt.Sprintf("deploy [%s]", ext.Color(image, c.Yellow))) {
		return
	}

	cmd := fmt.Sprintf(""+
		"gcloud run deploy %s --image %s --region %s --project=%s "+
		"--allow-unauthenticated "+
		"--port=8000 "+
		"--min-instances=0 "+
		"--max-instances=1 "+
		"--memory=512Mi "+
		"--cpu=1 "+
		"--ingress=all "+
		"--execution-environment=gen2 "+
		"--update-env-vars CREATED_AT=%s",
		serviceName, image, ext.PROJECT(), ext.REGION(), time.Now().Format(time.RFC3339))

	ext.Run(cmd)

	ext.Notify("new service created")
}

func metadataCmd() {
	images := queryImages(true)

	index, version := selectImage(images, "")
	fmt.Println(">", version)
	sep := ":"
	if strings.HasPrefix(version, "sha256") {
		sep = "@"
	}
	image := images[index].Package + sep + version

	cmd := "skopeo inspect docker://" + image
	ext.RunJQ(cmd, ".Name, .Digest, .Architecture, .Env")
}

func infoCmd() {
	serviceName := ext.SERVICE()
	service := serviceInfo(serviceName, ext.PROJECT(), ext.REGION())

	fmt.Println("service", ext.Color(serviceLink(ext.PROJECT(), ext.REGION(), serviceName), c.Blue))

	urls := strings.Split(service.Metadata.Annotations["run.googleapis.com/urls"], ",")
	fmt.Println("urls", ext.Color(strings.Join(urls, ", "), c.Magenta))

	currentImage := service.Spec.Template.Spec.Containers[0].Image
	fmt.Println("image", ext.Color(currentImage, c.Yellow))
	if strings.Contains(currentImage, "/") {
		// If the image is fully qualified, print it as a link.
		fmt.Println(registryLink(currentImage))
	}
	healthCmd()
}

func healthCmd() {
	service := serviceInfo(ext.SERVICE(), ext.PROJECT(), ext.REGION())
	url := service.Status.Address.URL + "/health"

	fmt.Println("\n" + ext.Color("GET ", c.Blue) + ext.Color(url, c.White))

	b, err := script.Get(url).Bytes()
	ext.Check(err)

	var v interface{}
	ext.Check(json.NewDecoder(bytes.NewReader(b)).Decode(&v))

	b, err = json.MarshalIndent(v, "", "  ")
	ext.Check(err)

	fmt.Println(string(b))
}

func revisionsCmd() {
	images := queryImages(true)
	for _, version := range textualizeVersions(images) {
		fmt.Println(version)
	}
}

func waitCmd() {
	lastImage := queryImages(true)[0]
	version := formatVersion(lastImage, 0)
	fmt.Println(">", version)

	s := spinner.New(
		spinner.CharSets[14], 100*time.Millisecond,
		spinner.WithHiddenCursor(false),
	)
	s.Start()
	for {
		time.Sleep(1 * time.Second)
		image := queryImages(false)[0]
		if lastImage.Version != image.Version {
			s.Stop()
			version := formatVersion(image, 0)
			fmt.Println(ext.Color(version, c.Yellow))
			break
		}
	}
	ext.Notify("new revision is pushed")
}

func initCmd() {
	cr := ".cr"
	if _, err := os.Create(cr); err == nil {
		ext.Die(".cr already exists")
	}
	f, err := os.Create(cr)
	ext.Check(err)
	defer f.Close()
	_, err = io.WriteString(f, strings.TrimSpace(CR)+"\n")
	ext.Check(err)
}

const CR = `
REPO=europe-docker.pkg.dev/PROJECT/REPO
NAME=IMAGE

PROJECT=project
REGION=region
SERVICE=service
`

func variablesCmd() {
	serviceName := ext.SERVICE()
	project := ext.PROJECT()
	region := ext.REGION()
	service := serviceInfo(serviceName, project, region)

	container := service.Spec.Template.Spec.Containers[0]

	// environment variables
	fmt.Println(ext.Color("env", c.Blue))
	for _, e := range container.Env {
		if e.ValueFrom != nil {
			continue
		}
		fmt.Printf("  %s=%s\n", ext.Color(e.Name, c.Yellow), e.Value)
	}

	// secrets
	// parse alias→real secret name from annotation "run.googleapis.com/secrets"
	// format: "alias-1:projects/PROJECT/secrets/SECRET_NAME,alias-2:..."
	secretAliases := map[string]string{}
	if mapping := service.Spec.Template.Metadata.Annotations["run.googleapis.com/secrets"]; mapping != "" {
		for entry := range strings.SplitSeq(mapping, ",") {
			alias, ref, _ := strings.Cut(entry, ":")
			// ref is "projects/PROJECT/secrets/SECRET_NAME"
			parts := strings.Split(ref, "/")
			if len(parts) >= 4 {
				secretAliases[alias] = parts[len(parts)-1]
			}
		}
	}

	fmt.Println(ext.Color("\nsecrets", c.Blue))
	hasSecrets := false
	for _, e := range container.Env {
		if e.ValueFrom == nil {
			continue
		}
		hasSecrets = true
		alias := e.ValueFrom.SecretKeyRef.Name
		secretVersion := e.ValueFrom.SecretKeyRef.Key
		secretName := secretAliases[alias]
		if secretName == "" {
			secretName = alias
		}
		secretLink := fmt.Sprintf("%s/security/secret-manager/secret/%s/versions?project=%s", ext.ConsoleURL, secretName, project)
		suffix := ""
		if *fExpand {
			cmd := fmt.Sprintf("gcloud secrets versions access %s --secret=%s --project=%s", secretVersion, secretName, project)
			val, err := ext.Exec(cmd, false).String()
			if err == nil {
				suffix = " (" + strings.TrimSpace(val) + ")"
			}
		}
		fmt.Printf("  %s → %s%s\n", ext.Color(e.Name, c.Yellow), ext.Href(secretLink, secretName+":"+secretVersion), suffix)
	}
	if !hasSecrets {
		fmt.Println("  (none)")
	}

	// console link
	link := fmt.Sprintf("%s/run/detail/%s/%s?project=%s", ext.ConsoleURL, region, serviceName, project)
	fmt.Println(ext.Color("\nconsole", c.Blue))
	fmt.Println(" ", link)

	// service account
	sa := service.Spec.Template.Spec.ServiceAccountName
	if sa != "" {
		fmt.Println(ext.Color("\nservice account", c.Blue))
		fmt.Println(" ", sa)
		if *fExpand {
			cmd := fmt.Sprintf(
				`gcloud projects get-iam-policy %s `+
					`--flatten="bindings[].members" `+
					`--filter="bindings.members:serviceAccount:%s" `+
					`--format="value(bindings.role)"`,
				project, sa)
			roles, err := ext.Exec(cmd, false).String()
			if err == nil {
				for role := range strings.SplitSeq(strings.TrimSpace(roles), "\n") {
					if role != "" {
						fmt.Println("  -", role)
					}
				}
			}
		}
	}
}

func terraformCmd() {
	tf := ext.TF()
	fmt.Println(tf)
	files := markedMainTF(tf)

	services := ext.SERVICES()
	for i, s := range services {
		parts := strings.SplitN(s, "-", 2)
		if len(parts) != 2 {
			ext.Die("service name must be in a form of 'env-name': %q", s)
		}
		services[i] = parts[1]
	}
	slices.Sort(services)
	services = slices.Compact(services)
	fmt.Println("@", services)

	for _, file := range files {
		lines := strings.Split(file.Content, "\n")
		for i, v := range lines {
			for _, s := range services {
				needle := s + "_image_tag"
				if strings.Contains(v, needle) && strings.Contains(v, `"`) {
					fmt.Printf("%s:%d:\n%s\n", c.White(file.Name), i+1, v)
				}
			}
		}
	}
}

type markedFile struct {
	Name    string
	Content string
}

func markedMainTF(tf string) []markedFile {
	project := ext.PROJECT()
	files := []markedFile{}
	fs.WalkDir(os.DirFS(tf), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(name, "main.tf") {
			return nil
		}
		name = path.Join(tf, name)
		content, err := os.ReadFile(name)
		if err != nil {
			ext.Die("read file: %v", err)
		}
		if !strings.Contains(string(content), "# @mark=") || !strings.Contains(string(content), project) {
			return nil
		}
		files = append(files, markedFile{Name: name, Content: string(content)})
		return nil
	})
	return files
}

var CompletionRoot = zsh.Args(
	zsh.NewArg("h:health", "/health"),
	zsh.NewArg("r:list", "list revisions"),
	zsh.NewArg("w:wait", "wait for new image revision"),
	zsh.NewArg("i:info", "show service info"),
	zsh.NewArg("d:deploy", "deploy a revision (default)"),
	zsh.NewArg("b:bounce", "bounce the service"),
	zsh.NewArg("c:create", "create a new service"),
	zsh.NewArg("m:metadata", "show image metadata"),
	zsh.NewArg("t:terraform", "cross-reference terraform"),
	zsh.NewArg("v:variables", "show environment variables and secrets"),
	zsh.NewArg("init", "create .cr file"),
)
