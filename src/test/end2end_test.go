package test

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/terraform"
	test_structure "github.com/gruntwork-io/terratest/modules/test-structure"
	"golang.org/x/crypto/ssh"
)

func TestEndToEndDeploymentScenario(t *testing.T) {
	//t.Parallel()

	fixtureFolder := "../"
	sshKeyPath := "/home/vsts/work/_temp/id_rsa"
	sshKeyPath2 := os.Getenv("TEST_SSH_KEY_PATH")
	fmt.Println("sshPath2: ", sshKeyPath2)
	fmt.Println("sshPath: ", sshKeyPath)

	if sshKeyPath == "" {
		//t.Fatalf("TEST_SSH_KEY_PATH environment variable cannot be empty.")
	} else {
		sshKeyPath = "/home/vsts/work/_temp/id_rsa"
	}

	// User Terratest to deploy the infrastructure
	test_structure.RunTestStage(t, "setup", func() {
		terraformOptions := &terraform.Options{
			// Indicate the directory that contains the Terraform configuration to deploy
			TerraformDir: fixtureFolder,
		}

		// Save options for later test stages
		test_structure.SaveTerraformOptions(t, fixtureFolder, terraformOptions)

		// Triggers the terraform init and terraform apply command
		terraform.InitAndApply(t, terraformOptions)
	})

	test_structure.RunTestStage(t, "validate", func() {
		// run validation checks here
		terraformOptions := test_structure.LoadTerraformOptions(t, fixtureFolder)

		vmLinux1PublicIPAddress := terraform.Output(t, terraformOptions, "vm_linux_1_public_ip_address")
		fmt.Println("publicIP: ", vmLinux1PublicIPAddress)
		
		vmLinux2PrivateIPAddress := terraform.Output(t, terraformOptions, "vm_linux_2_private_ip_address")

		// it takes some time for Azure to assign the public IP address so it's not available in Terraform output after the first apply
		attemptsCount := 0
		for attemptsCount < 2 {
			// add wait time to let Azure assign the public IP address and apply the configuration again, to refresh state.
			time.Sleep(30 * time.Second)
			terraform.Apply(t, terraformOptions)
			vmLinux1PublicIPAddress = terraform.Output(t, terraformOptions, "vm_linux_1_public_ip_address")
			attemptsCount++
			fmt.Println("publicIP: ", vmLinux1PublicIPAddress)
		}

		if vmLinux1PublicIPAddress == "" {
			t.Fatal("Cannot retrieve the public IP address value for the linux vm 1.")
		}
		
		fmt.Println("sshPath: ", sshKeyPath)

		key, err := ioutil.ReadFile(sshKeyPath)
		if err != nil {
			fmt.Println("sshPath: ", sshKeyPath)
			t.Fatalf("Unable to read private key: %v", err)
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			t.Fatalf("Unable to parse private key: %v", err)
		}

		sshConfig := &ssh.ClientConfig{
			User: "azureuser",
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(signer),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}

		sshConnection, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", "52.187.202.49"), sshConfig)
		if err != nil {
			t.Fatalf("Cannot establish SSH connection to vm-linux-1 public IP address: %v", err)
		}

		defer sshConnection.Close()
		sshSession, err := sshConnection.NewSession()
		if err != nil {
			t.Fatalf("Cannot create SSH session to vm-linux-1 public IP address: %v", err)
		}

		defer sshSession.Close()
		err = sshSession.Run(fmt.Sprintf("ping -c 1 %s", "10.0.2.5"))
		if err != nil {
			t.Fatalf("Cannot ping vm-linux-2 from vm-linux-2: %v", err)
		}
	})

	// When the test is completed, teardown the infrastructure by calling terraform destroy
	test_structure.RunTestStage(t, "teardown", func() {
		terraformOptions := test_structure.LoadTerraformOptions(t, fixtureFolder)
		terraform.Destroy(t, terraformOptions)
	})
}
