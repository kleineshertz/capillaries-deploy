package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/capillariesio/capillaries-deploy/pkg/cld"
	"github.com/capillariesio/capillaries-deploy/pkg/prj"
	"github.com/capillariesio/capillaries-deploy/pkg/provider"
)

func usage(flagset *flag.FlagSet) {
	fmt.Printf(`
Capillaries deploy
Usage: capideploy <command> [command parameters] [optional parameters]

Commands:
  %s -p <jsonnet project file>
  %s -p <jsonnet project file>
  %s -p <jsonnet project file>
  %s -p <jsonnet project file>
  %s -p <jsonnet project file>

  %s -p <jsonnet project file>
  %s -p <jsonnet project file>

  %s -p <jsonnet project file>
  %s -p <jsonnet project file>
  %s -p <jsonnet project file>
  %s -p <jsonnet project file>
  %s -p <jsonnet project file>
  %s -p <jsonnet project file>

  %s <comma-separated list of instances to create volumes on, or *> -p <jsonnet project file>
  %s <comma-separated list of instances to attach volumes on, or *> -p <jsonnet project file>
  %s <comma-separated list of instances to detach volumes on, or *> -p <jsonnet project file>
  %s <comma-separated list of instances to delete volumes on, or *> -p <jsonnet project file>
  %s <comma-separated list of instances to create, or *> -p <jsonnet project file>
  %s <comma-separated list of instances to delete, or *> -p <jsonnet project file>
  %s <comma-separated list of instances to ping, or *> -p <jsonnet project file> -n <number of repetitions, default 1>
  %s <comma-separated list of instances to install services on, or *> -p <jsonnet project file>
  %s <comma-separated list of instances to config services on, or *> -p <jsonnet project file>
  %s <comma-separated list of instances to start services on, or *> -p <jsonnet project file>
  %s <comma-separated list of instances to stop services on, or *> -p <jsonnet project file>
  %s <comma-separated list of instances to create snapshot images for, or *> -p <jsonnet project file>
  %s <comma-separated list of instances to create from snapshot images, or *> -p <jsonnet project file>
  %s <comma-separated list of instances to delete snapshot images for, or *> -p <jsonnet project file>

  %s -p <jsonnet project file>
`,
		provider.CmdDeploymentCreate,
		provider.CmdDeploymentCreateImages,
		provider.CmdDeploymentRestoreInstances,
		provider.CmdDeploymentDeleteImages,
		provider.CmdDeploymentDelete,

		provider.CmdListDeployments,
		provider.CmdListDeploymentResources,

		provider.CmdCreateFloatingIps,
		provider.CmdDeleteFloatingIps,
		provider.CmdCreateSecurityGroups,
		provider.CmdDeleteSecurityGroups,
		provider.CmdCreateNetworking,
		provider.CmdDeleteNetworking,

		provider.CmdCreateVolumes,
		provider.CmdAttachVolumes,
		provider.CmdDetachVolumes,
		provider.CmdDeleteVolumes,

		provider.CmdCreateInstances,
		provider.CmdDeleteInstances,
		provider.CmdPingInstances,

		provider.CmdInstallServices,
		provider.CmdConfigServices,
		provider.CmdStartServices,
		provider.CmdStopServices,

		provider.CmdCreateSnapshotImages,
		provider.CmdCreateInstancesFromSnapshotImages,
		provider.CmdDeleteSnapshotImages,

		provider.CmdCheckCassStatus,
	)
	if flagset != nil {
		fmt.Printf("\nParameters:\n")
		flagset.PrintDefaults()
	}
}

func main() {
	if len(os.Args) <= 1 {
		usage(nil)
		os.Exit(1)
	}

	commonArgs := flag.NewFlagSet("run prj args", flag.ExitOnError)
	argPrjFile := commonArgs.String("p", "capideploy.jsonnet", "Capideploy project jsonnet file path")
	argAssumeRole := commonArgs.String("r", "", "A role from another AWS account to assume, act like a third-party service")
	argAssumeRoleExternalId := commonArgs.String("e", "", "When a role from another AWS account is assumed, use this external-id (optional, but encouraged)")
	argVerbosity := commonArgs.Bool("v", false, "Verbose debug output")
	argNumberOfRepetitions := commonArgs.Int("n", 1, "Number of repetitions")
	argShowProjectDetails := commonArgs.Bool("s", false, "Show project details (may contain sensitive info)")
	argIgnoreAttachedVolumes := commonArgs.Bool("i", false, "Ignore attached volumes on instance delete")

	cmd := os.Args[1]
	nicknames := ""
	parseFromArgIdx := 2
	if provider.IsCmdRequiresNicknames(cmd) {
		if len(os.Args) <= 2 {
			usage(commonArgs)
			os.Exit(1)
		}
		nicknames = os.Args[2]
		parseFromArgIdx = 3
	}
	parseErr := commonArgs.Parse(os.Args[parseFromArgIdx:])
	if parseErr != nil {
		log.Fatalf(parseErr.Error())
	}

	var project *prj.Project
	var prjErr error
	project, prjErr = prj.LoadProject(*argPrjFile)
	if prjErr != nil {
		log.Fatalf(prjErr.Error())
	}

	// Unbuffered channels: write immediately to stdout/stderr/file/whatever
	cOut := make(chan string)
	cErr := make(chan string)
	cDone := make(chan int)
	go func(cOut <-chan string, cErr <-chan string, cDone <-chan int) {
		for {
			select {
			case strOut := <-cOut:
				fmt.Fprintf(os.Stdout, "%s\n", strOut)
			case strErr := <-cErr:
				fmt.Fprintf(os.Stderr, "%s\n", strErr)
			case <-cDone:
				return
			}
		}
	}(cOut, cErr, cDone)

	deployProvider, deployProviderErr := provider.DeployProviderFactory(project, context.TODO(), &provider.AssumeRoleConfig{RoleArn: *argAssumeRole, ExternalId: *argAssumeRoleExternalId}, *argVerbosity, cOut, cErr)
	if deployProviderErr != nil {
		cDone <- 0
		log.Fatalf(deployProviderErr.Error())
	}

	if len(os.Args) >= 3 {
		nicknames = os.Args[2]
	}

	var finalErr error
	if cmd == provider.CmdListDeployments {
		mapResourceCount, err := deployProvider.ListDeployments(cOut, cErr)
		if err == nil {
			sb := strings.Builder{}
			totalDeployments := 0
			totalResources := 0
			for deploymentName, resCount := range mapResourceCount {
				sb.WriteString(fmt.Sprintf("%s,%d\n", deploymentName, resCount))
				totalDeployments++
				totalResources += resCount
			}
			sb.WriteString(fmt.Sprintf("Deployments: %d, resources %d", totalDeployments, totalResources))
			cOut <- sb.String()
		}
		finalErr = err
	} else if cmd == provider.CmdListDeploymentResources {
		resources, err := deployProvider.ListDeploymentResources(cOut, cErr)
		if err == nil {
			sb := strings.Builder{}
			billedResources := 0
			for _, res := range resources {
				sb.WriteString(fmt.Sprintf("%s\n", res.String()))
				if res.BilledState == cld.ResourceBilledStateActive {
					billedResources++
				}
			}
			sb.WriteString(fmt.Sprintf("Resources: %d, billed %d", len(resources), billedResources))
			cOut <- sb.String()
		}
		finalErr = err
	} else {
		finalErr = deployProvider.ExecCmdWithNoResult(cmd, nicknames, &provider.ExecArgs{IgnoreAttachedVolumes: *argIgnoreAttachedVolumes, Verbosity: *argVerbosity, NumberOfRepetitions: *argNumberOfRepetitions, ShowProjectDetails: *argShowProjectDetails}, cOut, cErr)
	}

	cDone <- 0

	if finalErr != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

/*
TODO:
-r and -e go to env variables
ssh key from file to env variable and NewSshClientConfig changed accordingly
*/
