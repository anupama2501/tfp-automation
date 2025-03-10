package sanity

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
	"github.com/rancher/tfp-automation/framework/set/resources/rancher2"
	resources "github.com/rancher/tfp-automation/framework/set/resources/sanity"
	qase "github.com/rancher/tfp-automation/pipeline/qase/results"
	"github.com/rancher/tfp-automation/tests/extensions/provisioning"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TfpSanityTestSuite struct {
	suite.Suite
	client                     *rancher.Client
	session                    *session.Session
	rancherConfig              *rancher.Config
	terraformConfig            *config.TerraformConfig
	terratestConfig            *config.TerratestConfig
	standaloneTerraformOptions *terraform.Options
	terraformOptions           *terraform.Options
	adminUser                  *management.User
}

func (t *TfpSanityTestSuite) TearDownSuite() {
	keyPath := rancher2.SetKeyPath(keypath.SanityKeyPath)
	cleanup.Cleanup(t.T(), t.standaloneTerraformOptions, keyPath)
}

func (t *TfpSanityTestSuite) SetupSuite() {
	t.terraformConfig = new(config.TerraformConfig)
	ranchFrame.LoadConfig(config.TerraformConfigurationFileKey, t.terraformConfig)

	t.terratestConfig = new(config.TerratestConfig)
	ranchFrame.LoadConfig(config.TerratestConfigurationFileKey, t.terratestConfig)

	keyPath := rancher2.SetKeyPath(keypath.SanityKeyPath)
	standaloneTerraformOptions := framework.Setup(t.T(), t.terraformConfig, t.terratestConfig, keyPath)
	t.standaloneTerraformOptions = standaloneTerraformOptions

	resources.CreateMainTF(t.T(), t.standaloneTerraformOptions, keyPath, t.terraformConfig, t.terratestConfig)
}

func (t *TfpSanityTestSuite) TfpSetupSuite(terratestConfig *config.TerratestConfig, terraformConfig *config.TerraformConfig) {
	testSession := session.NewSession()
	t.session = testSession

	rancherConfig := new(rancher.Config)
	ranchFrame.LoadConfig(configs.Rancher, rancherConfig)

	t.rancherConfig = rancherConfig

	adminUser := &management.User{
		Username: "admin",
		Password: rancherConfig.AdminPassword,
	}

	t.adminUser = adminUser

	userToken, err := token.GenerateUserToken(adminUser, t.rancherConfig.Host)
	require.NoError(t.T(), err)

	rancherConfig.AdminToken = userToken.Token

	client, err := rancher.NewClient(rancherConfig.AdminToken, testSession)
	require.NoError(t.T(), err)

	t.client = client
	t.client.RancherConfig.AdminToken = rancherConfig.AdminToken

	keyPath := rancher2.SetKeyPath(keypath.RancherKeyPath)
	terraformOptions := framework.Setup(t.T(), terraformConfig, terratestConfig, keyPath)
	t.terraformOptions = terraformOptions
}

func (t *TfpSanityTestSuite) TestTfpProvisioningSanity() {
	nodeRolesDedicated := []config.Nodepool{config.EtcdNodePool, config.ControlPlaneNodePool, config.WorkerNodePool}

	tests := []struct {
		name      string
		nodeRoles []config.Nodepool
		module    string
	}{
		{"RKE1", nodeRolesDedicated, "linode_rke1"},
		{"RKE2", nodeRolesDedicated, "linode_rke2"},
		{"K3S", nodeRolesDedicated, "linode_k3s"},
	}

	for _, tt := range tests {
		terratestConfig := *t.terratestConfig
		terratestConfig.Nodepools = tt.nodeRoles
		terraformConfig := *t.terraformConfig
		terraformConfig.Module = tt.module

		t.TfpSetupSuite(&terratestConfig, &terraformConfig)

		provisioning.GetK8sVersion(t.T(), t.client, &terratestConfig, &terraformConfig, configs.DefaultK8sVersion)

		tt.name = tt.name + " Kubernetes version: " + terratestConfig.KubernetesVersion
		testUser, testPassword, clusterName, poolName := configs.CreateTestCredentials()

		t.Run((tt.name), func() {
			keyPath := rancher2.SetKeyPath(keypath.RancherKeyPath)
			defer cleanup.Cleanup(t.T(), t.terraformOptions, keyPath)

			clusterIDs := provisioning.Provision(t.T(), t.client, t.rancherConfig, &terraformConfig, &terratestConfig, testUser, testPassword, clusterName, poolName, t.terraformOptions, nil)
			provisioning.VerifyClustersState(t.T(), t.client, clusterIDs)
			provisioning.VerifyWorkloads(t.T(), t.client, clusterIDs)
		})
	}

	if t.terratestConfig.LocalQaseReporting {
		qase.ReportTest()
	}
}

func TestTfpSanityTestSuite(t *testing.T) {
	suite.Run(t, new(TfpSanityTestSuite))
}
