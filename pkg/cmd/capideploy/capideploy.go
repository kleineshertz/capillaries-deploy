package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/capillariesio/capillaries-deploy/pkg/l"
	"github.com/capillariesio/capillaries-deploy/pkg/prj"
	"github.com/capillariesio/capillaries-deploy/pkg/provider"
	"github.com/capillariesio/capillaries-deploy/pkg/rexec"
)

const (
	CmdListDeployments                   string = "list_deployments"
	CmdListDeploymentResources           string = "list_deployment_resources"
	CmdCreateFloatingIps                 string = "create_floating_ips"
	CmdDeleteFloatingIps                 string = "delete_floating_ips"
	CmdCreateSecurityGroups              string = "create_security_groups"
	CmdDeleteSecurityGroups              string = "delete_security_groups"
	CmdCreateNetworking                  string = "create_networking"
	CmdDeleteNetworking                  string = "delete_networking"
	CmdCreateVolumes                     string = "create_volumes"
	CmdDeleteVolumes                     string = "delete_volumes"
	CmdCreateInstances                   string = "create_instances"
	CmdDeleteInstances                   string = "delete_instances"
	CmdAttachVolumes                     string = "attach_volumes"
	CmdDetachVolumes                     string = "detach_volumes"
	CmdUploadFiles                       string = "upload_files"
	CmdDownloadFiles                     string = "download_files"
	CmdInstallServices                   string = "install_services"
	CmdConfigServices                    string = "config_services"
	CmdStartServices                     string = "start_services"
	CmdStopServices                      string = "stop_services"
	CmdPingInstances                     string = "ping_instances"
	CmdCreateSnapshotImages              string = "create_snapshot_images"
	CmdCreateInstancesFromSnapshotImages string = "create_instances_from_snapshot_images"
	CmdDeleteSnapshotImages              string = "delete_snapshot_images"
)

type SingleThreadCmdHandler func() (l.LogMsg, error)

func DumpLogChan(logChan chan l.LogMsg) {
	for len(logChan) > 0 {
		msg := <-logChan
		fmt.Println(string(msg))
	}
}

func getNicknamesArg(entityName string) (string, error) {
	if len(os.Args) < 3 {
		return "", fmt.Errorf("not enough args, expected comma-separated list of %s or '*'", entityName)
	}
	if len(os.Args[2]) == 0 {
		return "", fmt.Errorf("bad arg, expected comma-separated list of %s or '*'", entityName)
	}
	return os.Args[2], nil
}

func filterByNickname[GenericDef prj.InstanceDef](nicknames string, sourceMap map[string]*GenericDef, entityName string) (map[string]*GenericDef, error) {
	var defMap map[string]*GenericDef
	rawNicknames := strings.Split(nicknames, ",")
	defMap = map[string]*GenericDef{}
	for _, rawNickname := range rawNicknames {
		if strings.Contains(rawNickname, "*") {
			matchFound := false
			reNickname := regexp.MustCompile("^" + strings.ReplaceAll(rawNickname, "*", "[a-zA-Z0-9]*") + "$")
			for fgNickname, fgDef := range sourceMap {
				if reNickname.MatchString(fgNickname) {
					matchFound = true
					defMap[fgNickname] = fgDef
				}
			}
			if !matchFound {
				return nil, fmt.Errorf("no match found for %s '%s', available definitions: %s", entityName, rawNickname, reflect.ValueOf(sourceMap).MapKeys())
			}
		} else {
			fgDef, ok := sourceMap[rawNickname]
			if !ok {
				return nil, fmt.Errorf("definition for %s '%s' not found, available definitions: %s", entityName, rawNickname, reflect.ValueOf(sourceMap).MapKeys())
			}
			defMap[rawNickname] = fgDef
		}
	}
	return defMap, nil
}

func waitForWorkers(errorsExpected int, errChan chan error, logChan chan l.LogMsg) int {
	finalCmdErr := 0
	for errorsExpected > 0 {
		select {
		case cmdErr := <-errChan:
			if cmdErr != nil {
				finalCmdErr = 1
				fmt.Fprintf(os.Stderr, "%s\n", cmdErr.Error())
			}
			errorsExpected--
		case msg := <-logChan:
			fmt.Println(msg)
		}
	}

	DumpLogChan(logChan)

	return finalCmdErr
}

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
  %s <comma-separated list of instances to create volumes on, or 'all'> -p <jsonnet project file>
  %s <comma-separated list of instances to attach volumes on, or 'all'> -p <jsonnet project file>
  %s <comma-separated list of instances to detach volumes on, or 'all'> -p <jsonnet project file>
  %s <comma-separated list of instances to delete volumes on, or 'all'> -p <jsonnet project file>
  %s <comma-separated list of instances to create, or 'all'> -p <jsonnet project file>
  %s <comma-separated list of instances to delete, or 'all'> -p <jsonnet project file>
  %s <comma-separated list of instances to ping, or 'all'> -p <jsonnet project file> -n <number of repetitions, default 1>
  %s <comma-separated list of instances to install services on, or 'all'> -p <jsonnet project file>
  %s <comma-separated list of instances to config services on, or 'all'> -p <jsonnet project file>
  %s <comma-separated list of instances to start services on, or 'all'> -p <jsonnet project file>
  %s <comma-separated list of instances to stop services on, or 'all'> -p <jsonnet project file>
  %s <comma-separated list of instances to create snapshot images for, or 'all'> -p <jsonnet project file>
  %s <comma-separated list of instances to create from snapshot images, or 'all'> -p <jsonnet project file>
  %s <comma-separated list of instances to delete snapshot images for, or 'all'> -p <jsonnet project file>
`,
		CmdListDeployments,
		CmdListDeploymentResources,

		CmdCreateFloatingIps,
		CmdDeleteFloatingIps,
		CmdCreateSecurityGroups,
		CmdDeleteSecurityGroups,
		CmdCreateNetworking,
		CmdDeleteNetworking,

		CmdCreateVolumes,
		CmdAttachVolumes,
		CmdDetachVolumes,
		CmdDeleteVolumes,

		CmdCreateInstances,
		CmdDeleteInstances,
		CmdPingInstances,

		CmdInstallServices,
		CmdConfigServices,
		CmdStartServices,
		CmdStopServices,

		CmdCreateSnapshotImages,
		CmdCreateInstancesFromSnapshotImages,
		CmdDeleteSnapshotImages,
	)
	if flagset != nil {
		fmt.Printf("\nParameters:\n")
		flagset.PrintDefaults()
	}
}

// func createProject(templatePath string) error {
// 	vm := jsonnet.MakeVM()
// 	json, err := vm.EvaluateFile(templatePath)
// 	if err != nil {
// 		return err
// 	}
// 	fmt.Println(json)
// 	return nil
// }

func ping(sshConfig *rexec.SshConfigDef, ipAddress string, verbosity bool, numberOfRepetitions int) (l.LogMsg, error) {
	var err error
	var logMsg l.LogMsg

	repetitions := 1
	if numberOfRepetitions > 1 {
		repetitions = numberOfRepetitions
	}

	lb := l.NewLogBuilder(l.CurFuncName()+" "+ipAddress, verbosity)

	for {
		logMsg, err = rexec.ExecCommandOnInstance(sshConfig, ipAddress, "id", verbosity)
		lb.Add(string(logMsg))
		repetitions--
		if err == nil || repetitions == 0 {
			break
		}
		lb.Add(err.Error())
		time.Sleep(5 * time.Second)
	}

	return lb.Complete(err)
}

func main() {
	if len(os.Args) <= 1 {
		usage(nil)
		os.Exit(1)
	}

	// if os.Args[1] == CmdCreateProject {
	// 	createPrjArgs := flag.NewFlagSet("create prj args", flag.ExitOnError)
	// 	argTemplateFile := createPrjArgs.String("t", "capideploy.jsonnet", "Capideploy project template jsonnet file path")

	// 	if len(os.Args) <= 2 {
	// 		usage(createPrjArgs)
	// 		os.Exit(1)
	// 	}
	// 	parseErr := createPrjArgs.Parse(os.Args[2:])
	// 	if parseErr != nil {
	// 		log.Fatalf(parseErr.Error())
	// 	}
	// 	createPrjErr := createProject(*argTemplateFile)
	// 	if createPrjErr != nil {
	// 		log.Fatalf(createPrjErr.Error())
	// 	}
	// 	os.Exit(0)
	// }

	commonArgs := flag.NewFlagSet("run prj args", flag.ExitOnError)
	argPrjFile := commonArgs.String("p", "capideploy.json", "Capideploy project jsonnet file path")
	argVerbosity := commonArgs.Bool("v", false, "Verbose debug output")
	argNumberOfRepetitions := commonArgs.Int("n", 1, "Number of repetitions")
	argShowProjectDetails := commonArgs.Bool("s", false, "Show project details (may contain sensitive info)")
	argIgnoreAttachedVolumes := commonArgs.Bool("i", false, "Ignore attached volumes on instance delete")
	argAssumeRole := commonArgs.String("r", "", "A role from another AWS account to assume, act like a third-party service")
	argAssumeRoleExternalId := commonArgs.String("e", "", "When a role from another AWS account is assumed, use this external-id (optional, but encouraged)")

	cmdStartTs := time.Now()

	throttle := time.Tick(time.Second) // One call per second, to avoid error 429 on openstack/aws/azure calls
	const maxWorkerThreads int = 50
	var logChan = make(chan l.LogMsg, maxWorkerThreads*5)
	var sem = make(chan int, maxWorkerThreads)
	var errChan chan error
	var parseErr error
	errorsExpected := 1
	//var prjPair *prj.ProjectPair
	var project *prj.Project
	//var fullPrjPath string
	var prjErr error

	singleThreadCommands := map[string]SingleThreadCmdHandler{
		CmdListDeployments:         nil,
		CmdListDeploymentResources: nil,
		CmdCreateFloatingIps:       nil,
		CmdDeleteFloatingIps:       nil,
		CmdCreateSecurityGroups:    nil,
		CmdDeleteSecurityGroups:    nil,
		CmdCreateNetworking:        nil,
		CmdDeleteNetworking:        nil,
	}

	if _, ok := singleThreadCommands[os.Args[1]]; ok {
		parseErr = commonArgs.Parse(os.Args[2:])
	} else {
		parseErr = commonArgs.Parse(os.Args[3:])
	}
	if parseErr != nil {
		log.Fatalf(parseErr.Error())
	}

	project, prjErr = prj.LoadProject(*argPrjFile)
	if prjErr != nil {
		log.Fatalf(prjErr.Error())
	}

	deployProvider, deployProviderErr := provider.DeployProviderFactory(project, context.TODO(), &provider.AssumeRoleConfig{RoleArn: *argAssumeRole, ExternalId: *argAssumeRoleExternalId}, *argVerbosity)
	if deployProviderErr != nil {
		log.Fatalf(deployProviderErr.Error())
	}
	singleThreadCommands[CmdListDeployments] = deployProvider.ListDeployments
	singleThreadCommands[CmdListDeploymentResources] = deployProvider.ListDeploymentResources
	singleThreadCommands[CmdCreateFloatingIps] = deployProvider.CreateFloatingIps
	singleThreadCommands[CmdDeleteFloatingIps] = deployProvider.DeleteFloatingIps
	singleThreadCommands[CmdCreateSecurityGroups] = deployProvider.CreateSecurityGroups
	singleThreadCommands[CmdDeleteSecurityGroups] = deployProvider.DeleteSecurityGroups
	singleThreadCommands[CmdCreateNetworking] = deployProvider.CreateNetworking
	singleThreadCommands[CmdDeleteNetworking] = deployProvider.DeleteNetworking

	if cmdHandler, ok := singleThreadCommands[os.Args[1]]; ok {
		errChan = make(chan error, errorsExpected)
		sem <- 1
		go func() {
			logMsg, err := cmdHandler()
			logChan <- logMsg
			errChan <- err
			<-sem
		}()
	} else if os.Args[1] == CmdCreateInstances ||
		os.Args[1] == CmdDeleteInstances ||
		os.Args[1] == CmdCreateSnapshotImages ||
		os.Args[1] == CmdCreateInstancesFromSnapshotImages ||
		os.Args[1] == CmdDeleteSnapshotImages {
		nicknames, err := getNicknamesArg("instances")
		if err != nil {
			log.Fatalf(err.Error())
		}
		instances, err := filterByNickname(nicknames, project.Instances, "instance")
		if err != nil {
			log.Fatalf(err.Error())
		}

		errorsExpected = len(instances)
		errChan = make(chan error, errorsExpected)

		usedFlavors := map[string]string{}
		usedImages := map[string]bool{}
		if os.Args[1] == CmdCreateInstances ||
			os.Args[1] == CmdCreateInstancesFromSnapshotImages {
			logMsgBastionIp, err := deployProvider.PopulateInstanceExternalAddressByName()
			if err != nil {
				log.Fatal(logMsgBastionIp)
			}

			// Make sure image/flavor is supported
			usedKeypairs := map[string]struct{}{}
			for _, instDef := range instances {
				usedFlavors[instDef.FlavorName] = ""
				usedImages[instDef.ImageId] = false
				usedKeypairs[instDef.RootKeyName] = struct{}{}
			}
			logMsg, err := deployProvider.HarvestInstanceTypesByFlavorNames(usedFlavors)
			logChan <- logMsg
			DumpLogChan(logChan)
			if err != nil {
				log.Fatalf(err.Error())
			}

			logMsg, err = deployProvider.HarvestImageIds(usedImages)
			logChan <- logMsg
			DumpLogChan(logChan)
			if err != nil {
				log.Fatalf(err.Error())
			}

			// Make sure the keypairs are there
			logMsg, err = deployProvider.VerifyKeypairs(usedKeypairs)
			logChan <- logMsg
			DumpLogChan(logChan)
			if err != nil {
				log.Fatalf(err.Error())
			}

			fmt.Printf("Creating instances, consider clearing known_hosts to avoid ssh complaints:\n")
			for _, i := range instances {
				fmt.Printf("ssh-keygen -f ~/.ssh/known_hosts -R %s;\n", i.BestIpAddress())
			}
		}

		switch os.Args[1] {
		case CmdCreateInstances:
			logMsgBastionIp, err := deployProvider.PopulateInstanceExternalAddressByName()
			if err != nil {
				log.Fatal(logMsgBastionIp)
			}
			for iNickname := range instances {
				<-throttle
				sem <- 1
				go func(project *prj.Project, logChan chan l.LogMsg, errChan chan error, iNickname string) {
					logMsg, err := deployProvider.CreateInstanceAndWaitForCompletion(
						iNickname,
						usedFlavors[project.Instances[iNickname].FlavorName],
						project.Instances[iNickname].ImageId)
					logChan <- logMsg
					errChan <- err
					<-sem
				}(project, logChan, errChan, iNickname)
			}
		case CmdDeleteInstances:
			logMsgBastionIp, err := deployProvider.PopulateInstanceExternalAddressByName()
			if err != nil {
				log.Fatal(logMsgBastionIp)
			}
			for iNickname := range instances {
				<-throttle
				sem <- 1
				go func(project *prj.Project, logChan chan l.LogMsg, errChan chan error, iNickname string) {
					logMsg, err := deployProvider.DeleteInstance(iNickname, *argIgnoreAttachedVolumes)
					logChan <- logMsg
					errChan <- err
					<-sem
				}(project, logChan, errChan, iNickname)
			}
		case CmdCreateSnapshotImages:
			for iNickname := range instances {
				<-throttle
				sem <- 1
				go func(project *prj.Project, logChan chan l.LogMsg, errChan chan error, iNickname string) {
					logMsg, err := deployProvider.CreateSnapshotImage(iNickname)
					logChan <- logMsg
					errChan <- err
					<-sem
				}(project, logChan, errChan, iNickname)
			}
		case CmdCreateInstancesFromSnapshotImages:
			for iNickname := range instances {
				<-throttle
				sem <- 1
				go func(project *prj.Project, logChan chan l.LogMsg, errChan chan error, iNickname string) {
					logMsg, err := deployProvider.CreateInstanceFromSnapshotImageAndWaitForCompletion(iNickname,
						usedFlavors[project.Instances[iNickname].FlavorName])
					logChan <- logMsg
					errChan <- err
					<-sem
				}(project, logChan, errChan, iNickname)
			}
		case CmdDeleteSnapshotImages:
			for iNickname := range instances {
				<-throttle
				sem <- 1
				go func(project *prj.Project, logChan chan l.LogMsg, errChan chan error, iNickname string) {
					logMsg, err := deployProvider.DeleteSnapshotImage(iNickname)
					logChan <- logMsg
					errChan <- err
					<-sem
				}(project, logChan, errChan, iNickname)
			}
		default:
			log.Fatalf("unknown create/delete instance command:" + os.Args[1])
		}
	} else if os.Args[1] == CmdPingInstances ||
		os.Args[1] == CmdInstallServices ||
		os.Args[1] == CmdConfigServices ||
		os.Args[1] == CmdStartServices ||
		os.Args[1] == CmdStopServices {
		nicknames, err := getNicknamesArg("instances")
		if err != nil {
			log.Fatalf(err.Error())
		}

		instances, err := filterByNickname(nicknames, project.Instances, "instance")
		if err != nil {
			log.Fatalf(err.Error())
		}

		logMsgBastionIp, err := deployProvider.PopulateInstanceExternalAddressByName()
		if err != nil {
			log.Fatal(logMsgBastionIp)
		}

		errorsExpected = len(instances)
		errChan = make(chan error, len(instances))
		for _, iDef := range instances {
			<-throttle
			sem <- 1
			go func(prj *prj.Project, logChan chan l.LogMsg, errChan chan error, iDef *prj.InstanceDef) {
				var logMsg l.LogMsg
				var finalErr error
				switch os.Args[1] {
				case CmdPingInstances:
					logMsg, finalErr = ping(project.SshConfig, iDef.BestIpAddress(), *argVerbosity, *argNumberOfRepetitions)

				case CmdInstallServices:
					// Make sure ping passes
					logMsg, finalErr = ping(project.SshConfig, iDef.BestIpAddress(), *argVerbosity, 5)

					// If ping passed, it's ok to move on
					if finalErr == nil {
						logMsg, finalErr = rexec.ExecEmbeddedScriptsOnInstance(project.SshConfig, iDef.BestIpAddress(), iDef.Service.Cmd.Install, iDef.Service.Env, *argVerbosity)
					}

				case CmdConfigServices:
					logMsg, finalErr = rexec.ExecEmbeddedScriptsOnInstance(project.SshConfig, iDef.BestIpAddress(), iDef.Service.Cmd.Config, iDef.Service.Env, *argVerbosity)

				case CmdStartServices:
					logMsg, finalErr = rexec.ExecEmbeddedScriptsOnInstance(project.SshConfig, iDef.BestIpAddress(), iDef.Service.Cmd.Start, iDef.Service.Env, *argVerbosity)

				case CmdStopServices:
					logMsg, finalErr = rexec.ExecEmbeddedScriptsOnInstance(project.SshConfig, iDef.BestIpAddress(), iDef.Service.Cmd.Stop, iDef.Service.Env, *argVerbosity)

				default:
					log.Fatalf("unknown service command:" + os.Args[1])
				}

				logChan <- logMsg
				errChan <- finalErr
				<-sem
			}(project, logChan, errChan, iDef)
		}

	} else if os.Args[1] == CmdCreateVolumes || os.Args[1] == CmdAttachVolumes || os.Args[1] == CmdDetachVolumes || os.Args[1] == CmdDeleteVolumes {
		nicknames, err := getNicknamesArg("instances")
		if err != nil {
			log.Fatalf(err.Error())
		}

		instances, err := filterByNickname(nicknames, project.Instances, "instance")
		if err != nil {
			log.Fatalf(err.Error())
		}

		volCount := 0
		for _, iDef := range instances {
			volCount += len(iDef.Volumes)
		}
		if volCount == 0 {
			fmt.Printf("No volumes to create/attach/detach/delete")
			os.Exit(0)
		}
		errorsExpected = volCount
		errChan = make(chan error, volCount)
		for iNickname, iDef := range instances {
			for volNickname := range iDef.Volumes {
				<-throttle
				sem <- 1
				switch os.Args[1] {
				case CmdCreateVolumes:
					go func(project *prj.Project, logChan chan l.LogMsg, errChan chan error, iNickname string, volNickname string) {
						logMsg, err := deployProvider.CreateVolume(iNickname, volNickname)
						logChan <- logMsg
						errChan <- err
						<-sem
					}(project, logChan, errChan, iNickname, volNickname)
				case CmdAttachVolumes:
					logMsgBastionIp, err := deployProvider.PopulateInstanceExternalAddressByName()
					if err != nil {
						log.Fatal(logMsgBastionIp)
					}
					go func(project *prj.Project, logChan chan l.LogMsg, errChan chan error, iNickname string, volNickname string) {
						logMsg, err := deployProvider.AttachVolume(iNickname, volNickname)
						logChan <- logMsg
						errChan <- err
						<-sem
					}(project, logChan, errChan, iNickname, volNickname)
				case CmdDetachVolumes:
					logMsgBastionIp, err := deployProvider.PopulateInstanceExternalAddressByName()
					if err != nil {
						log.Fatal(logMsgBastionIp)
					}
					go func(project *prj.Project, logChan chan l.LogMsg, errChan chan error, iNickname string, volNickname string) {
						logMsg, err := deployProvider.DetachVolume(iNickname, volNickname)
						logChan <- logMsg
						errChan <- err
						<-sem
					}(project, logChan, errChan, iNickname, volNickname)
				case CmdDeleteVolumes:
					go func(project *prj.Project, logChan chan l.LogMsg, errChan chan error, iNickname string, volNickname string) {
						logMsg, err := deployProvider.DeleteVolume(iNickname, volNickname)
						logChan <- logMsg
						errChan <- err
						<-sem
					}(project, logChan, errChan, iNickname, volNickname)
				default:
					log.Fatalf("unknown command:" + os.Args[1])
				}
			}
		}
	} else {
		log.Fatalf("unknown command:" + os.Args[1])
	}

	finalCmdErr := waitForWorkers(errorsExpected, errChan, logChan)

	// // Save updated project template, it may have some new ids and timestamps
	// if prjErr = prjPair.Template.SaveProject(fullPrjPath); prjErr != nil {
	// 	log.Fatalf(prjErr.Error())
	// }

	if *argShowProjectDetails {
		prjJsonBytes, err := json.MarshalIndent(project, "", "    ")
		if err != nil {
			log.Fatalf("cannot show project json: %s", err.Error())
		}
		fmt.Printf("%s\n", string(prjJsonBytes))
	}

	if finalCmdErr > 0 {
		os.Exit(finalCmdErr)
	}

	fmt.Printf("%s %sOK%s, elapsed %.3fs\n", os.Args[1], l.LogColorGreen, l.LogColorReset, time.Since(cmdStartTs).Seconds())
}
