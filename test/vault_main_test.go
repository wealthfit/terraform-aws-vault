package test

import (
	"fmt"
	"testing"

	"github.com/gruntwork-io/terratest/modules/aws"
	"github.com/gruntwork-io/terratest/modules/packer"
	"github.com/gruntwork-io/terratest/modules/test-structure"
)

const AMI_EXAMPLE_PATH = "../examples/vault-consul-ami/vault-consul.json"

type testCase struct {
	Name       string                                   // Name of the test
	Func       func(*testing.T, string, string, string) // Function that runs test. Receives(t, amiId, awsRegion, sshUserName)
	Enterprise bool                                     // Run on ami with enterprise vault installed
}

type amiData struct {
	Name            string // Name of the ami
	PackerBuildName string // Build name of ami
	SshUserName     string // ssh user name of ami
	Enterprise      bool   // Install vault enterprise on ami
}

var amisData = []amiData{
	{"vaultEnterpriseUbuntu", "ubuntu16-ami", "ubuntu", true},
	{"vaultEnterpriseAmazonLinux", "amazon-linux-ami", "ec2-user", true},
	{"vaultUbuntu", "ubuntu16-ami", "ubuntu", false},
	{"vaultAmazonLinux", "amazon-linux-ami", "ec2-user", false},
}

var testCases = []testCase{
	{
		"TestVaultAutoUnseal",
		runVaultAutoUnsealTest,
		true,
	},
	{
		"TestEnterpriseInstallation",
		runVaultEnterpriseClusterTest,
		true,
	},
	{
		"TestVaultEC2Auth",
		runVaultEC2AuthTest,
		false,
	},
	{
		"TestVaultIAMAuth",
		runVaultIAMAuthTest,
		false,
	},
	{
		"TestVaultWithS3Backend",
		runVaultWithS3BackendClusterTest,
		false,
	},
	{
		"TestVaultPrivateCluster",
		runVaultPrivateClusterTest,
		false,
	},
	{
		"TestVaultPublicCluster",
		runVaultPublicClusterTest,
		false,
	},
}

func TestMainVaultCluster(t *testing.T) {
	t.Parallel()

	test_structure.RunTestStage(t, "setup_amis", func() {
		tlsCert := generateSelfSignedTlsCert(t)
		saveTlsCert(t, WORK_DIR, tlsCert)

		amisPackerOptions := map[string]*packer.Options{}
		for _, ami := range amisData {
			awsRegion := aws.GetRandomRegion(t, nil, nil)
			test_structure.SaveString(t, WORK_DIR, fmt.Sprintf("awsRegion-%s", ami.Name), awsRegion)

			if ami.Enterprise {
				amisPackerOptions[ami.Name] = composeAmiOptions(t, AMI_EXAMPLE_PATH, ami.PackerBuildName, tlsCert, awsRegion, getUrlFromEnv(t))
			} else {
				amisPackerOptions[ami.Name] = composeAmiOptions(t, AMI_EXAMPLE_PATH, ami.PackerBuildName, tlsCert, awsRegion, "")
			}
		}

		amiIds := packer.BuildArtifacts(t, amisPackerOptions)
		for key, amiId := range amiIds {
			test_structure.SaveString(t, WORK_DIR, fmt.Sprintf("amiId-%s", key), amiId)
		}
	})

	defer test_structure.RunTestStage(t, "delete_amis", func() {
		for _, ami := range amisData {
			awsRegion := test_structure.LoadString(t, WORK_DIR, fmt.Sprintf("awsRegion-%s", ami.Name))
			amiId := test_structure.LoadString(t, WORK_DIR, fmt.Sprintf("amiId-%s", ami.Name))
			aws.DeleteAmi(t, awsRegion, amiId)
		}

		tlsCert := loadTlsCert(t, WORK_DIR)
		cleanupTlsCertFiles(tlsCert)
	})

	t.Run("group", func(t *testing.T) {
		runTestsOnDifferentPlatforms(t)
	})

}

func runTestsOnDifferentPlatforms(t *testing.T) {
	for _, testCase := range testCases {
		// This re-assignment necessary, because the variable testCase is defined and set outside the forloop.
		// As such, it gets overwritten on each iteration of the forloop. This is fine if you don't have concurrent code in the loop,
		// but in this case, because you have a t.Parallel, the t.Run completes before the test function exits,
		// which means that the value of testCase might change.
		// More information at:
		// "Be Careful with Table Driven Tests and t.Parallel()"
		// https://gist.github.com/posener/92a55c4cd441fc5e5e85f27bca008721
		testCase := testCase
		for _, ami := range amisData {
			ami := ami
			if testCase.Enterprise == ami.Enterprise {
				t.Run(fmt.Sprintf("%sWith%sAmi", testCase.Name, ami.Name), func(t *testing.T) {
					t.Parallel()
					awsRegion := test_structure.LoadString(t, WORK_DIR, fmt.Sprintf("awsRegion-%s", ami.Name))
					amiId := test_structure.LoadString(t, WORK_DIR, fmt.Sprintf("amiId-%s", ami.Name))
					testCase.Func(t, amiId, awsRegion, ami.SshUserName)
				})
			}
		}
	}
}
