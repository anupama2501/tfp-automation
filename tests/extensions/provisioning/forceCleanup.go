package provisioning

import (
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/rancher/tfp-automation/framework/cleanup"
	resources "github.com/rancher/tfp-automation/framework/set/resources/rancher2"
)

// ForceCleanup is a function that will forcibly run terraform destroy and cleanup Terraform resources.
func ForceCleanup(t *testing.T) error {
	keyPath := resources.SetKeyPath()

	terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		TerraformDir: keyPath,
		NoColor:      true,
	})

	terraform.Destroy(t, terraformOptions)
	cleanup.TFFilesCleanup(keyPath)

	return nil
}
