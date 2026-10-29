package ext

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const yamlConfig = `repo: registry.example/default
image: example-api
region: europe-west2
project: default-project
services:
  - name: default-api
  - name: other-api
    repo: registry.example/other
    project: other-project
    region: europe-west6
`

func TestCloudRunOverridesAndIsolation(t *testing.T) {
	oldVariables, oldConfig, oldSelected := variables, cloudRunConfig, selectedCloudRunService
	t.Cleanup(func() { variables, cloudRunConfig, selectedCloudRunService = oldVariables, oldConfig, oldSelected })
	config, err := parseCloudRun([]byte(yamlConfig))
	if err != nil {
		t.Fatal(err)
	}
	variables = map[string]string{"PROJECTS": "wrong", "SERVICE_NAME": "wrong", "IAP_SERVICE_ACCOUNT": "fallback"}
	for _, name := range []string{"other-api", "default-api"} {
		if err := applyCloudRun(config, name); err != nil {
			t.Fatal(err)
		}
		project, region, repo := "default-project", "europe-west2", "registry.example/default"
		if name == "other-api" {
			project, region, repo = "other-project", "europe-west6", "registry.example/other"
		}
		if PROJECT() != project || REGION() != region || IMAGE() != repo+"/example-api" || SERVICE() != name {
			t.Fatalf("unexpected resolved values: %#v", variables)
		}
		if len(SERVICES()) != 2 || IAP_SERVICE_ACCOUNT() != "" {
			t.Fatal("fallback settings leaked or service list lost")
		}
	}
	if err := applyCloudRun(config, "unknown"); err == nil {
		t.Fatal("unknown service accepted")
	}
}

func TestCloudRunInvalid(t *testing.T) {
	for _, data := range []string{
		"", "[", strings.Replace(yamlConfig, "image:", "imgae:", 1),
		strings.Replace(yamlConfig, "image: example-api\n", "", 1),
		strings.Replace(yamlConfig, "name: other-api", "name: default-api", 1),
		strings.Replace(yamlConfig, "other-project", "other-project --flag", 1),
		yamlConfig + "---\n{}\n", yamlConfig + "project: duplicate\n",
		"repo: registry/repo\nimage: api\nregion: eu\nproject: test\n",
	} {
		if _, err := parseCloudRun([]byte(data)); err == nil {
			t.Errorf("accepted invalid config %q", data)
		}
	}
}

func TestCloudRunFilePrecedence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".cloudrun.yaml")
	config, err := readCloudRun(path)
	if err != nil || config != nil {
		t.Fatal("absent YAML should permit fallback loading")
	}
	if err := os.WriteFile(path, []byte("invalid: ["), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readCloudRun(path); err == nil {
		t.Fatal("invalid YAML must not permit fallback")
	}
	if err := os.WriteFile(path, []byte(yamlConfig), 0600); err != nil {
		t.Fatal(err)
	}
	config, err = readCloudRun(path)
	if err != nil || config == nil {
		t.Fatal(err)
	}
}

func TestCloudRunImageOnly(t *testing.T) {
	data := strings.Split(yamlConfig, "services:")[0] + "services: []\n"
	config, err := parseCloudRun([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Services) != 0 {
		t.Fatal("unexpected services")
	}
}

func TestCloudRunLoadIgnoresFallbackFiles(t *testing.T) {
	oldVariables, oldConfig, oldSelected := variables, cloudRunConfig, selectedCloudRunService
	t.Cleanup(func() { variables, cloudRunConfig, selectedCloudRunService = oldVariables, oldConfig, oldSelected })
	t.Chdir(t.TempDir())
	for _, file := range []string{".cr", ".env", "Makefile"} {
		if err := os.WriteFile(file, []byte("PROJECT=wrong\nREGION=wrong\nREPO=wrong\nNAME=wrong\nSERVICE=wrong\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	data := strings.Split(yamlConfig, "  - name: other-api")[0]
	if err := os.WriteFile(".cloudrun.yaml", []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	LoadCloudRunVariables()
	if PROJECT() != "default-project" || SERVICE() != "default-api" || IMAGE() != "registry.example/default/example-api" {
		t.Fatalf("fallback settings leaked: %#v", variables)
	}
}
