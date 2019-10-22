package install

import (
	"strings"
	"testing"

	"github.com/instrumenta/kubeval/kubeval"
	"github.com/stretchr/testify/assert"
)

func testFillInTemplates(t *testing.T, params TemplateParameters) map[string][]byte {
	manifests, err := FillInTemplates(params)
	assert.NoError(t, err)
	assert.Len(t, manifests, 6)
	for fileName, contents := range manifests {
		validationResults, err := kubeval.Validate(contents)
		assert.NoError(t, err)
		for _, result := range validationResults {
			if len(result.Errors) > 0 {
				t.Errorf("found problems with manifest %s (Kind %s):\ncontent:\n%s\nerrors: %s",
					fileName,
					result.Kind,
					string(contents),
					result.Errors)
			}
		}
	}
	return manifests
}

func TestFillInTemplatesAllParameters(t *testing.T) {
	testFillInTemplates(t, TemplateParameters{
		GitURL:             "git@github.com:fluxcd/flux-get-started",
		GitBranch:          "branch",
		GitPaths:           []string{"dir1", "dir2"},
		GitLabel:           "label",
		GitUser:            "User",
		GitEmail:           "this.is@anemail.com",
		Namespace:          "flux",
		GitReadOnly:        true,
		ManifestGeneration: true,
		AdditionalFluxArgs: []string{"arg1=foo", "arg2=bar"},
	})
}

func TestFillInTemplatesMissingValues(t *testing.T) {
	testFillInTemplates(t, TemplateParameters{
		GitURL:    "git@github.com:fluxcd/flux-get-started",
		GitBranch: "branch",
		GitPaths:  []string{},
		GitLabel:  "label",
	})
}

func TestFillInTemplatesConfigFile(t *testing.T) {

	configFile := `config1: configuration1
config2: configuration2
config3: configuration3`

	tests := map[string]struct {
		params              TemplateParameters
		configFileName      string
		configFileNameCheck string
		deploymentFileCheck string
	}{
		"configMap": {
			params: TemplateParameters{
				GitURL:             "git@github.com:fluxcd/flux-get-started",
				GitBranch:          "branch",
				GitPaths:           []string{"dir1", "dir2"},
				GitLabel:           "label",
				GitUser:            "User",
				GitEmail:           "this.is@anemail.com",
				Namespace:          "flux",
				ConfigAsConfigMap:  true,
				AdditionalFluxArgs: []string{"arg1=foo", "arg2=bar"},
			},
			configFileName:      "flux-config.yaml",
			configFileNameCheck: "    config2: config",
			deploymentFileCheck: "name: flux-config",
		},
		"secret": {
			params: TemplateParameters{
				GitURL:             "git@github.com:fluxcd/flux-get-started",
				GitBranch:          "branch",
				GitPaths:           []string{"dir1", "dir2"},
				GitLabel:           "label",
				GitUser:            "User",
				GitEmail:           "this.is@anemail.com",
				Namespace:          "flux",
				ConfigAsConfigMap:  false,
				AdditionalFluxArgs: []string{"arg1=foo", "arg2=bar"},
			},
			configFileName: "flux-config.yaml",
			// the following field value is the base64 encoding of the config file string above
			configFileNameCheck: "  flux-config.yaml: Y29uZmlnMTogY29uZmlndXJhdGlvbjEKY29uZmlnMjogY29uZmlndXJhdGlvbjIKY29uZmlnMzogY29uZmlndXJhdGlvbjM=",
			deploymentFileCheck: "secretName: flux-config",
		},
	}

	for name, test := range tests {
		t.Run(name, func(*testing.T) {
			configContent, err := ConfigContent(strings.NewReader(configFile), test.params.ConfigAsConfigMap)
			if err != nil {
				t.Fatal(err)
			}
			test.params.ConfigFileContent = configContent
			manifests := testFillInTemplates(t, test.params)
			for fileName, contents := range manifests {
				if fileName == test.configFileName {
					assert.Contains(t, string(contents), test.configFileNameCheck)
				}
				if fileName == "flux-deployment.yaml" {
					assert.Contains(t, string(contents), test.deploymentFileCheck)
				}
			}
		})
	}
}
