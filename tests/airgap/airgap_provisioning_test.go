package airgap

import (
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/token"
	ranchFrame "github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/rancher/tfp-automation/config"
	"github.com/rancher/tfp-automation/defaults/configs"
	"github.com/rancher/tfp-automation/defaults/keypath"
	"github.com/rancher/tfp-automation/framework"
	"github.com/rancher/tfp-automation/framework/cleanup"
	"github.com/rancher/tfp-automation/framework/set/resources/airgap"
	"github.com/rancher/tfp-automation/framework/set/resources/rancher2"
	qase "github.com/rancher/tfp-automation/pipeline/qase/results"
	"github.com/rancher/tfp-automation/tests/extensions/provisioning"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TfpAirgapProvisioningTestSuite struct {
	suite.Suite
	client                     *rancher.Client
	session                    *session.Session
	rancherConfig              *rancher.Config
	terraformConfig            *config.TerraformConfig
	terratestConfig            *config.TerratestConfig
	standaloneTerraformOptions *terraform.Options
	terraformOptions           *terraform.Options
	adminUser                  *management.User
	registry                   string
}

func (a *TfpAirgapProvisioningTestSuite) TearDownSuite() {
	keyPath := rancher2.SetKeyPath(keypath.AirgapKeyPath)
	cleanup.Cleanup(a.T(), a.standaloneTerraformOptions, keyPath)
}

func (a *TfpAirgapProvisioningTestSuite) SetupSuite() {
	a.terraformConfig = new(config.TerraformConfig)
	ranchFrame.LoadConfig(config.TerraformConfigurationFileKey, a.terraformConfig)

	a.terratestConfig = new(config.TerratestConfig)
	ranchFrame.LoadConfig(config.TerratestConfigurationFileKey, a.terratestConfig)

	keyPath := rancher2.SetKeyPath(keypath.AirgapKeyPath)
	standaloneTerraformOptions := framework.Setup(a.T(), a.terraformConfig, a.terratestConfig, keyPath)
	a.standaloneTerraformOptions = standaloneTerraformOptions

	registry, err := airgap.CreateMainTF(a.T(), a.standaloneTerraformOptions, keyPath, a.terraformConfig, a.terratestConfig)
	require.NoError(a.T(), err)

	a.registry = registry
}

func (a *TfpAirgapProvisioningTestSuite) TfpSetupSuite(terratestConfig *config.TerratestConfig, terraformConfig *config.TerraformConfig) {
	testSession := session.NewSession()
	a.session = testSession

	rancherConfig := new(rancher.Config)
	ranchFrame.LoadConfig(configs.Rancher, rancherConfig)

	a.rancherConfig = rancherConfig

	adminUser := &management.User{
		Username: "admin",
		Password: rancherConfig.AdminPassword,
	}

	a.adminUser = adminUser

	userToken, err := token.GenerateUserToken(adminUser, a.rancherConfig.Host)
	require.NoError(a.T(), err)

	rancherConfig.AdminToken = userToken.Token

	client, err := rancher.NewClient(rancherConfig.AdminToken, testSession)
	require.NoError(a.T(), err)

	a.client = client
	a.client.RancherConfig.AdminToken = rancherConfig.AdminToken

	keyPath := rancher2.SetKeyPath(keypath.RancherKeyPath)
	terraformOptions := framework.Setup(a.T(), terraformConfig, terratestConfig, keyPath)
	a.terraformOptions = terraformOptions
}

func (a *TfpAirgapProvisioningTestSuite) TestTfpAirgapProvisioning() {
	tests := []struct {
		name   string
		module string
	}{
		{"Airgap RKE1", "airgap_rke1"},
		{"Airgap RKE2", "airgap_rke2"},
		{"Airgap K3S", "airgap_k3s"},
	}

	for _, tt := range tests {
		terratestConfig := *a.terratestConfig
		terraformConfig := *a.terraformConfig
		terraformConfig.Module = tt.module

		terraformConfig.PrivateRegistries.SystemDefaultRegistry = a.registry
		terraformConfig.PrivateRegistries.URL = a.registry

		a.TfpSetupSuite(&terratestConfig, &terraformConfig)

		provisioning.GetK8sVersion(a.T(), a.client, &terratestConfig, &terraformConfig, configs.DefaultK8sVersion)

		tt.name = tt.name + " Kubernetes version: " + terratestConfig.KubernetesVersion
		testUser, testPassword, clusterName, poolName := configs.CreateTestCredentials()

		a.Run((tt.name), func() {
			keyPath := rancher2.SetKeyPath(keypath.RancherKeyPath)
			defer cleanup.Cleanup(a.T(), a.terraformOptions, keyPath)

			clusterIDs := provisioning.Provision(a.T(), a.client, a.rancherConfig, &terraformConfig, &terratestConfig, testUser, testPassword, clusterName, poolName, a.terraformOptions, nil)
			provisioning.VerifyClustersState(a.T(), a.client, clusterIDs)
			provisioning.VerifyWorkloads(a.T(), a.client, clusterIDs)
		})
	}

	if a.terratestConfig.LocalQaseReporting {
		qase.ReportTest()
	}
}

func (a *TfpAirgapProvisioningTestSuite) TestTfpAirgapUpgrading() {
	tests := []struct {
		name   string
		module string
	}{
		{"Upgrading Airgap RKE1", "airgap_rke1"},
		{"Upgrading Airgap RKE2", "airgap_rke2"},
		{"Upgrading Airgap K3S", "airgap_k3s"},
	}

	for _, tt := range tests {
		terratestConfig := *a.terratestConfig
		terraformConfig := *a.terraformConfig
		terraformConfig.Module = tt.module

		terraformConfig.PrivateRegistries.SystemDefaultRegistry = a.registry
		terraformConfig.PrivateRegistries.URL = a.registry

		a.TfpSetupSuite(&terratestConfig, &terraformConfig)

		provisioning.GetK8sVersion(a.T(), a.client, &terratestConfig, &terraformConfig, configs.SecondHighestVersion)

		tt.name = tt.name + " Kubernetes version: " + terratestConfig.KubernetesVersion
		testUser, testPassword, clusterName, poolName := configs.CreateTestCredentials()

		a.Run((tt.name), func() {
			keyPath := rancher2.SetKeyPath(keypath.RancherKeyPath)
			defer cleanup.Cleanup(a.T(), a.terraformOptions, keyPath)

			clusterIDs := provisioning.Provision(a.T(), a.client, a.rancherConfig, &terraformConfig, &terratestConfig, testUser, testPassword, clusterName, poolName, a.terraformOptions, nil)
			provisioning.VerifyClustersState(a.T(), a.client, clusterIDs)
			provisioning.VerifyWorkloads(a.T(), a.client, clusterIDs)

			provisioning.KubernetesUpgrade(a.T(), a.client, a.rancherConfig, &terraformConfig, &terratestConfig, testUser, testPassword, clusterName, poolName, a.terraformOptions)
			provisioning.VerifyClustersState(a.T(), a.client, clusterIDs)
			provisioning.VerifyWorkloads(a.T(), a.client, clusterIDs)
		})
	}

	if a.terratestConfig.LocalQaseReporting {
		qase.ReportTest()
	}
}

func TestTfpAirgapProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(TfpAirgapProvisioningTestSuite))
}
