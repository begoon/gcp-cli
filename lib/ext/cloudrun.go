package ext

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"gopkg.in/yaml.v3"
)

type cloudRunDefaults struct {
	Repo    string `yaml:"repo"`
	Image   string `yaml:"image"`
	Region  string `yaml:"region"`
	Project string `yaml:"project"`
}

type cloudRunService struct {
	Name             string `yaml:"name"`
	cloudRunDefaults `yaml:",inline"`
}

type cloudRunFile struct {
	cloudRunDefaults  `yaml:",inline"`
	Services          []cloudRunService `yaml:"services"`
	TF                string            `yaml:"tf,omitempty"`
	IAPServiceAccount string            `yaml:"iap_service_account,omitempty"`
	CredentialFile    string            `yaml:"credential_file,omitempty"`
}

var cloudRunConfig *cloudRunFile
var selectedCloudRunService string
var cloudRunValue = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]*$`)

func parseCloudRun(data []byte) (*cloudRunFile, error) {
	var config cloudRunFile
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("only one YAML document is allowed")
	}
	if config.Services == nil {
		return nil, errors.New("services must be a list (use [] for image-only configuration)")
	}
	if err := validateCloudRunDefaults(config.cloudRunDefaults); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, service := range config.Services {
		if !cloudRunValue.MatchString(service.Name) || strings.ContainsAny(service.Name, "/._") {
			return nil, fmt.Errorf("invalid service name %q", service.Name)
		}
		if seen[service.Name] {
			return nil, fmt.Errorf("duplicate service %q", service.Name)
		}
		seen[service.Name] = true
		if err := validateCloudRunDefaults(config.resolve(service)); err != nil {
			return nil, fmt.Errorf("service %s: %w", service.Name, err)
		}
	}
	return &config, nil
}

func validateCloudRunDefaults(value cloudRunDefaults) error {
	for _, field := range []struct{ name, value string }{{"repo", value.Repo}, {"image", value.Image}, {"region", value.Region}, {"project", value.Project}} {
		if !cloudRunValue.MatchString(field.value) {
			return fmt.Errorf("missing or invalid %s: %q", field.name, field.value)
		}
		if field.name != "repo" && strings.Contains(field.value, "/") {
			return fmt.Errorf("%s must not contain /", field.name)
		}
	}
	return nil
}

func (config *cloudRunFile) resolve(service cloudRunService) cloudRunDefaults {
	result := config.cloudRunDefaults
	if service.Repo != "" {
		result.Repo = service.Repo
	}
	if service.Image != "" {
		result.Image = service.Image
	}
	if service.Region != "" {
		result.Region = service.Region
	}
	if service.Project != "" {
		result.Project = service.Project
	}
	return result
}

func readCloudRun(path string) (*cloudRunFile, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return parseCloudRun(data)
}

func applyCloudRun(config *cloudRunFile, name string) error {
	resolved := config.cloudRunDefaults
	if name != "" {
		found := false
		for _, service := range config.Services {
			if service.Name == name {
				resolved = config.resolve(service)
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("unknown service %q", name)
		}
	}
	cloudRunConfig = config
	selectedCloudRunService = name
	variables = map[string]string{
		"REPO": strings.TrimRight(resolved.Repo, "/"), "NAME": resolved.Image,
		"PROJECT": resolved.Project, "REGION": resolved.Region, "SERVICE": name,
		"TF": config.TF, "IAP_SERVICE_ACCOUNT": config.IAPServiceAccount,
		"CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE": config.CredentialFile,
	}
	return nil
}

// LoadCloudRunVariables gives .cloudrun.yaml exclusive precedence for cr.
func LoadCloudRunVariables() {
	config, err := readCloudRun(".cloudrun.yaml")
	Check(err, ".cloudrun.yaml")
	if config == nil {
		cloudRunConfig = nil
		selectedCloudRunService = ""
		LoadVariables()
		return
	}
	fmt.Println("configuration = .cloudrun.yaml")
	name := ""
	if len(config.Services) == 1 {
		name = config.Services[0].Name
	}
	if len(config.Services) > 1 {
		names := make([]string, len(config.Services))
		for i, service := range config.Services {
			names[i] = service.Name
		}
		Check(survey.AskOne(&survey.Select{Message: "service", Options: names}, &name, survey.WithValidator(survey.Required)))
	}
	Check(applyCloudRun(config, name))
	values := map[string]string{"PROJECT": PROJECT(), "REGION": REGION(), "IMAGE": IMAGE()}
	if name != "" {
		values["SERVICE"] = name
	}
	data, err := json.MarshalIndent(values, "", "  ")
	Check(err)
	fmt.Println(string(data))
}
