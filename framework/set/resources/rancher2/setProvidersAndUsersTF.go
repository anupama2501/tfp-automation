package rancher2

import (
	"os"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/tfp-automation/config"
	"github.com/rancher/tfp-automation/framework/set/defaults"
	"github.com/sirupsen/logrus"
	"github.com/zclconf/go-cty/cty"
)

const (
	apiURL              = "api_url"
	globalRoleBinding   = "rancher2_global_role_binding"
	globalRoleID        = "global_role_id"
	insecure            = "insecure"
	name                = "name"
	provider            = "provider"
	rancher2            = "rancher2"
	rancherSource       = "source"
	rancherUser         = "rancher2_user"
	rc                  = "-rc"
	requiredProviders   = "required_providers"
	terraform           = "terraform"
	testPassword        = "password"
	tokenKey            = "token_key"
	version             = "version"
	user                = "user"
	userID              = "user_id"
	username            = "username"
	providerEnvVar      = "RANCHER2_PROVIDER_VERSION"
	awsProviderEnvVar   = "AWS_PROVIDER_VERSION"
	localProviderEnvVar = "LOCALS_PROVIDER_VERSION"
)

// SetProvidersAndUsersTF is a helper function that will set the general Terraform configurations in the main.tf file.
func SetProvidersAndUsersTF(rancherConfig *rancher.Config, terraformConfig *config.TerraformConfig, testUser, testPassword string, authProvider bool, configMap []map[string]any) (*hclwrite.File, *hclwrite.Body) {
	providerVersion, awsProviderVersion, localProviderVersion, source := getProviderVersions(terraformConfig)

	newFile := hclwrite.NewEmptyFile()
	rootBody := newFile.Body()

	createRequiredProviders(rootBody, terraformConfig, awsProviderVersion, localProviderVersion, providerVersion, source, configMap)

	rootBody.AppendNewline()

	createProvider(rootBody, rancherConfig)

	createUser(rootBody, testUser, testPassword)

	if !authProvider {
		createGlobalRoleBinding(rootBody, testUser, userID)
	}

	return newFile, rootBody
}

// getProviderVersions returns the versions for the providers based on environment variables.
func getProviderVersions(terraformConfig *config.TerraformConfig) (string, string, string, string) {
	providerVersion := os.Getenv(providerEnvVar)
	if providerVersion == "" {
		logrus.Fatalf("Expected env var not set %s", providerEnvVar)
	}

	var awsProviderVersion, localProviderVersion string

	if strings.Contains(terraformConfig.Module, "custom") || strings.Contains(terraformConfig.Module, "airgap") || terraformConfig.MultiCluster {
		awsProviderVersion = os.Getenv(awsProviderEnvVar)
		if awsProviderVersion == "" {
			logrus.Fatalf("Expected env var not set %s", awsProviderEnvVar)
		}

		localProviderVersion = os.Getenv(localProviderEnvVar)
		if providerVersion == "" {
			logrus.Fatalf("Expected env var not set %s", localProviderEnvVar)
		}
	}

	source := "rancher/rancher2"
	if strings.Contains(providerVersion, rc) {
		source = "terraform.local/local/rancher2"
	}

	return providerVersion, awsProviderVersion, localProviderVersion, source
}

// createRequiredProviders creates the required_providers block.
func createRequiredProviders(rootBody *hclwrite.Body, terraformConfig *config.TerraformConfig, awsProviderVersion, localProviderVersion, providerVersion, source string, configMap []map[string]any) {
	tfBlock := rootBody.AppendNewBlock(terraform, nil)
	tfBlockBody := tfBlock.Body()

	reqProvsBlock := tfBlockBody.AppendNewBlock(requiredProviders, nil)
	reqProvsBlockBody := reqProvsBlock.Body()

	customModule := false

	if terraformConfig.MultiCluster {
		for _, terratestConfig := range configMap {
			module := terratestConfig["terraform"].(config.TerraformConfig).Module

			if strings.Contains(module, defaults.Custom) {
				customModule = true
			}
		}
	}

	if strings.Contains(terraformConfig.Module, defaults.Custom) || strings.Contains(terraformConfig.Module, defaults.Airgap) || customModule {
		reqProvsBlockBody.SetAttributeValue(defaults.Aws, cty.ObjectVal(map[string]cty.Value{
			defaults.Source:  cty.StringVal(defaults.AwsSource),
			defaults.Version: cty.StringVal(awsProviderVersion),
		}))

		reqProvsBlockBody.SetAttributeValue(defaults.Local, cty.ObjectVal(map[string]cty.Value{
			defaults.Source:  cty.StringVal(defaults.LocalSource),
			defaults.Version: cty.StringVal(localProviderVersion),
		}))
	}

	reqProvsBlockBody.SetAttributeValue(rancher2, cty.ObjectVal(map[string]cty.Value{
		rancherSource: cty.StringVal(source),
		version:       cty.StringVal(providerVersion),
	}))

	if strings.Contains(terraformConfig.Module, defaults.Custom) || strings.Contains(terraformConfig.Module, defaults.Airgap) {
		awsProvBlock := rootBody.AppendNewBlock(defaults.Provider, []string{defaults.Aws})
		awsProvBlockBody := awsProvBlock.Body()

		awsProvBlockBody.SetAttributeValue(defaults.Region, cty.StringVal(terraformConfig.AWSConfig.Region))
		awsProvBlockBody.SetAttributeValue(defaults.AccessKey, cty.StringVal(terraformConfig.AWSCredentials.AWSAccessKey))
		awsProvBlockBody.SetAttributeValue(defaults.SecretKey, cty.StringVal(terraformConfig.AWSCredentials.AWSSecretKey))

		rootBody.AppendNewline()
		rootBody.AppendNewBlock(defaults.Provider, []string{defaults.Local})
		rootBody.AppendNewline()
	}
}

// createProvider creates a provider block for the given rancher config.
func createProvider(rootBody *hclwrite.Body, rancherConfig *rancher.Config) {
	provBlock := rootBody.AppendNewBlock(provider, []string{rancher2})
	provBlockBody := provBlock.Body()

	provBlockBody.SetAttributeValue(apiURL, cty.StringVal("https://"+rancherConfig.Host))
	provBlockBody.SetAttributeValue(tokenKey, cty.StringVal(rancherConfig.AdminToken))
	provBlockBody.SetAttributeValue(insecure, cty.BoolVal(*rancherConfig.Insecure))

	rootBody.AppendNewline()
}

// createUser creates the user block for a new user.
func createUser(rootBody *hclwrite.Body, testUser, testpassword string) {
	userBlock := rootBody.AppendNewBlock(defaults.Resource, []string{rancherUser, rancherUser})
	userBlockBody := userBlock.Body()

	userBlockBody.SetAttributeValue(name, cty.StringVal(testUser))
	userBlockBody.SetAttributeValue(username, cty.StringVal(testUser))
	userBlockBody.SetAttributeValue(testPassword, cty.StringVal(testpassword))
	userBlockBody.SetAttributeValue(defaults.Enabled, cty.BoolVal(true))

	rootBody.AppendNewline()
}

// createGlobalRoleBinding creates a global role binding block for the given user.
func createGlobalRoleBinding(rootBody *hclwrite.Body, testUser string, userID string) {
	globalRoleBindingBlock := rootBody.AppendNewBlock(defaults.Resource, []string{globalRoleBinding, globalRoleBinding})
	globalRoleBindingBlockBody := globalRoleBindingBlock.Body()

	globalRoleBindingBlockBody.SetAttributeValue(name, cty.StringVal(testUser))
	globalRoleBindingBlockBody.SetAttributeValue(globalRoleID, cty.StringVal(user))

	standardUser := hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte(rancherUser + "." + rancherUser + ".id")},
	}

	globalRoleBindingBlockBody.SetAttributeRaw(userID, standardUser)
}
