package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/capillariesio/capillaries-deploy/pkg/cld"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
	"github.com/capillariesio/capillaries-deploy/pkg/prj"
	"github.com/capillariesio/capillaries-deploy/pkg/rexec"
)

const (
	CmdDeploymentCreate                  string = "deployment_create"
	CmdDeploymentCreateImages            string = "deployment_create_images"
	CmdDeploymentRestoreInstances        string = "deployment_restore_instances"
	CmdDeploymentDeleteImages            string = "deployment_delete_images"
	CmdDeploymentDelete                  string = "deployment_delete"
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
	CmdCheckCassStatus                   string = "check_cassandra_status"
)

type StopOnFailType int

const (
	StopOnFail StopOnFailType = iota
	IgnoreFail
)

type ExecArgs struct {
	IgnoreAttachedVolumes bool
	Verbosity             bool
	NumberOfRepetitions   int
	ShowProjectDetails    bool
}

type CombinedCmdCall struct {
	Cmd       string
	Nicknames string
	OnFail    StopOnFailType
}

var combinedCmdCallSeqMap map[string][]CombinedCmdCall = map[string][]CombinedCmdCall{
	CmdDeploymentCreate: {
		{CmdCreateFloatingIps, "", StopOnFail},
		{CmdCreateNetworking, "", StopOnFail},
		{CmdCreateSecurityGroups, "", StopOnFail},
		{CmdCreateVolumes, "*", StopOnFail},
		{CmdCreateInstances, "*", StopOnFail},
		{CmdPingInstances, "*", StopOnFail},
		{CmdAttachVolumes, "bastion", StopOnFail},
		{CmdInstallServices, "bastion", StopOnFail},
		{CmdInstallServices, "rabbitmq,prometheus,daemon*,cass*", StopOnFail},
		{CmdStopServices, "cass*", StopOnFail},
		{CmdConfigServices, "cass*", StopOnFail},
		{CmdConfigServices, "bastion,rabbitmq,prometheus,daemon*", StopOnFail},
		{CmdCheckCassStatus, "", StopOnFail}},
	CmdDeploymentCreateImages: {
		{CmdStopServices, "*", IgnoreFail},
		{CmdDetachVolumes, "bastion", StopOnFail},
		{CmdCreateSnapshotImages, "*", StopOnFail},
		{CmdDeleteInstances, "*", StopOnFail}},
	CmdDeploymentRestoreInstances: {
		{CmdCreateInstancesFromSnapshotImages, "*", StopOnFail},
		{CmdPingInstances, "*", StopOnFail},
		{CmdAttachVolumes, "bastion", StopOnFail},
		{CmdStartServices, "*", StopOnFail},
		{CmdStopServices, "cass*", StopOnFail},
		{CmdConfigServices, "cass*", StopOnFail}},
	CmdDeploymentDeleteImages: {
		{CmdDeleteSnapshotImages, "*", StopOnFail}},
	CmdDeploymentDelete: {
		{CmdDeleteSnapshotImages, "*", StopOnFail},
		{CmdStopServices, "*", IgnoreFail},
		{CmdDetachVolumes, "bastion", StopOnFail},
		{CmdDeleteInstances, "*", IgnoreFail},
		{CmdDeleteVolumes, "*", IgnoreFail},
		{CmdDeleteSecurityGroups, "*", IgnoreFail},
		{CmdDeleteNetworking, "*", IgnoreFail},
		{CmdDeleteFloatingIps, "*", IgnoreFail}}}

func IsCmdRequiresNicknames(cmd string) bool {
	return cmd == CmdCreateVolumes ||
		cmd == CmdDeleteVolumes ||
		cmd == CmdCreateInstances ||
		cmd == CmdDeleteInstances ||
		cmd == CmdAttachVolumes ||
		cmd == CmdDetachVolumes ||
		cmd == CmdUploadFiles ||
		cmd == CmdDownloadFiles ||
		cmd == CmdInstallServices ||
		cmd == CmdConfigServices ||
		cmd == CmdStartServices ||
		cmd == CmdStopServices ||
		cmd == CmdPingInstances ||
		cmd == CmdCreateSnapshotImages ||
		cmd == CmdCreateInstancesFromSnapshotImages ||
		cmd == CmdDeleteSnapshotImages
}

type DeployCtx struct {
	Project   *prj.Project
	GoCtx     context.Context
	IsVerbose bool
	Tags      map[string]string
	// AWS members:
	Aws *AwsCtx
	// Azure members:
}

type DeployProvider interface {
	ListDeployments(cOut chan<- string, cErr chan<- string) (map[string]int, error)
	ListDeploymentResources(cOut chan<- string, cErr chan<- string) ([]*cld.Resource, error)
	ExecCmdWithNoResult(cmd string, nicknames string, execArgs *ExecArgs, cOut chan<- string, cErr chan<- string) error
}

func genericListDeployments(p deployProviderImpl, cOut chan<- string, cErr chan<- string) (map[string]int, error) {
	mapResourceCount, logMsg, err := p.listDeployments()
	cOut <- string(logMsg)
	if err != nil {
		cErr <- err.Error()
	}
	return mapResourceCount, err
}

func genericListDeploymentResources(p deployProviderImpl, cOut chan<- string, cErr chan<- string) ([]*cld.Resource, error) {
	resources, logMsg, err := p.listDeploymentResources()
	cOut <- string(logMsg)
	if err != nil {
		cErr <- err.Error()
	}
	return resources, err
}

func genericExecCmdWithNoResult(p deployProviderImpl, cmd string, nicknames string, execArgs *ExecArgs, cOut chan<- string, cErr chan<- string) error {
	if combinedCmdCallSeq, ok := combinedCmdCallSeqMap[cmd]; ok {
		for _, combinedCmdCallSeq := range combinedCmdCallSeq {
			err := execSimpleParallelCmd(p, combinedCmdCallSeq.Cmd, combinedCmdCallSeq.Nicknames, execArgs, cOut, cErr)
			if err != nil && combinedCmdCallSeq.OnFail == StopOnFail {
				return err
			}
		}
		return nil
	} else {
		return execSimpleParallelCmd(p, cmd, nicknames, execArgs, cOut, cErr)
	}
}

type AssumeRoleConfig struct {
	// AWS members:
	RoleArn    string `json:"role_arn"`
	ExternalId string `json:"external_id"`
	// Azure members:
}

func DeployProviderFactory(project *prj.Project, goCtx context.Context, assumeRoleCfg *AssumeRoleConfig, isVerbose bool, cOut chan<- string, cErr chan<- string) (DeployProvider, error) {
	if project.DeployProviderName == prj.DeployProviderAws {
		cfg, err := config.LoadDefaultConfig(goCtx)
		if err != nil {
			cErr <- err.Error()
			return nil, err
		}

		callerIdentityOutBefore, err := sts.NewFromConfig(cfg).GetCallerIdentity(goCtx, &sts.GetCallerIdentityInput{})
		if err != nil {
			err = fmt.Errorf("cannot get caller identity before assuming role %s: %s", assumeRoleCfg.RoleArn, err.Error())
			cErr <- err.Error()
			return nil, err
		}

		if assumeRoleCfg != nil && assumeRoleCfg.RoleArn != "" {
			creds := stscreds.NewAssumeRoleProvider(sts.NewFromConfig(cfg), assumeRoleCfg.RoleArn,
				func(o *stscreds.AssumeRoleOptions) {
					o.ExternalID = aws.String(assumeRoleCfg.ExternalId)
					o.RoleSessionName = "third-party-capideploy-assumes-role-provided-by-customer"
					o.Duration = time.Duration(1 * time.Hour)
				})
			cfg.Credentials = aws.NewCredentialsCache(creds)

			callerIdentityOutAfter, err := sts.NewFromConfig(cfg).GetCallerIdentity(goCtx, &sts.GetCallerIdentityInput{})
			if err != nil {
				err = fmt.Errorf("cannot get caller identity after assuming role %s: %s", assumeRoleCfg.RoleArn, err.Error())
				cErr <- err.Error()
				return nil, err
			}

			if *callerIdentityOutBefore.Arn == *callerIdentityOutAfter.Arn {
				err = fmt.Errorf("cannot proceed with the same caller identity after assuming role %s: %s", assumeRoleCfg.RoleArn, *callerIdentityOutAfter.Arn)
				cErr <- err.Error()
				return nil, err
			}
			cOut <- fmt.Sprintf("Caller identity (role assumed): %s", *callerIdentityOutAfter.Arn)
		} else {
			cOut <- fmt.Sprintf("Caller identity (no role assumed): %s", *callerIdentityOutBefore.Arn)
		}

		return &AwsDeployProvider{
			DeployCtx: &DeployCtx{
				Project:   project,
				GoCtx:     goCtx,
				IsVerbose: isVerbose,
				Tags: map[string]string{
					cld.DeploymentNameTagName:     project.DeploymentName,
					cld.DeploymentOperatorTagName: cld.DeploymentOperatorTagValue},
				Aws: &AwsCtx{
					Ec2Client:     ec2.NewFromConfig(cfg),
					TaggingClient: resourcegroupstaggingapi.NewFromConfig(cfg),
				},
			},
		}, nil
	}
	return nil, fmt.Errorf("unsupported deploy provider %s", project.DeployProviderName)
}

const MaxWorkerThreads int = 50

type SingleThreadCmdHandler func() (l.LogMsg, error)

func pingOneHost(sshConfig *rexec.SshConfigDef, ipAddress string, verbosity bool, numberOfRepetitions int) (l.LogMsg, error) {
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

func execSimpleParallelCmd(deployProvider deployProviderImpl, cmd string, nicknames string, execArgs *ExecArgs, cOut chan<- string, cErr chan<- string) error {
	cmdStartTs := time.Now()
	throttle := time.NewTicker(time.Second) // One call per second, to avoid error 429 on openstack/aws/azure calls
	var sem = make(chan int, MaxWorkerThreads)
	var errChan chan error
	var errorsExpected int

	singleThreadNoResultCommands := map[string]SingleThreadCmdHandler{
		CmdCreateFloatingIps:    deployProvider.CreateFloatingIps,
		CmdDeleteFloatingIps:    deployProvider.DeleteFloatingIps,
		CmdCreateSecurityGroups: deployProvider.CreateSecurityGroups,
		CmdDeleteSecurityGroups: deployProvider.DeleteSecurityGroups,
		CmdCreateNetworking:     deployProvider.CreateNetworking,
		CmdDeleteNetworking:     deployProvider.DeleteNetworking,
		CmdCheckCassStatus:      deployProvider.CheckCassStatus,
	}

	if cmdHandler, ok := singleThreadNoResultCommands[cmd]; ok {
		errorsExpected = 1
		errChan = make(chan error, errorsExpected)
		sem <- 1
		go func() {
			logMsg, err := cmdHandler()
			cOut <- string(logMsg)
			errChan <- err
			<-sem
		}()
	} else if cmd == CmdCreateInstances ||
		cmd == CmdDeleteInstances ||
		cmd == CmdCreateSnapshotImages ||
		cmd == CmdCreateInstancesFromSnapshotImages ||
		cmd == CmdDeleteSnapshotImages {
		if len(nicknames) == 0 {
			err := fmt.Errorf("not enough args, expected comma-separated list of instances or '*'")
			cErr <- err.Error()
			return err
		}

		instances, err := filterByNickname(nicknames, deployProvider.getDeployCtx().Project.Instances, "instance")
		if err != nil {
			cErr <- err.Error()
			return err
		}

		errorsExpected = len(instances)
		errChan = make(chan error, errorsExpected)

		usedFlavors := map[string]string{}
		usedImages := map[string]bool{}
		if cmd == CmdCreateInstances ||
			cmd == CmdCreateInstancesFromSnapshotImages {
			logMsgBastionIp, err := deployProvider.PopulateInstanceExternalAddressByName()
			cOut <- string(logMsgBastionIp)
			if err != nil {
				cErr <- err.Error()
				return err
			}

			// Make sure image/flavor is supported
			usedKeypairs := map[string]struct{}{}
			for _, instDef := range instances {
				usedFlavors[instDef.FlavorName] = ""
				usedImages[instDef.ImageId] = false
				usedKeypairs[instDef.RootKeyName] = struct{}{}
			}
			logMsg, err := deployProvider.HarvestInstanceTypesByFlavorNames(usedFlavors)
			cOut <- string(logMsg)
			if err != nil {
				cErr <- err.Error()
				return err
			}

			logMsg, err = deployProvider.HarvestImageIds(usedImages)
			cOut <- string(logMsg)
			if err != nil {
				cErr <- err.Error()
				return err
			}

			// Make sure the keypairs are there
			logMsg, err = deployProvider.VerifyKeypairs(usedKeypairs)
			cOut <- string(logMsg)
			if err != nil {
				cErr <- err.Error()
				return err
			}

			cOut <- "Creating instances, consider clearing known_hosts to avoid ssh complaints:"
			for _, i := range instances {
				cOut <- fmt.Sprintf("ssh-keygen -f ~/.ssh/known_hosts -R %s;", i.BestIpAddress())
			}
		}

		switch cmd {
		case CmdCreateInstances:
			logMsgBastionIp, err := deployProvider.PopulateInstanceExternalAddressByName()
			cOut <- string(logMsgBastionIp)
			if err != nil {
				cErr <- err.Error()
				return err
			}
			for iNickname := range instances {
				<-throttle.C
				sem <- 1
				go func(project *prj.Project, logChan chan<- string, errChan chan<- error, iNickname string) {
					logMsg, err := deployProvider.CreateInstanceAndWaitForCompletion(
						iNickname,
						usedFlavors[deployProvider.getDeployCtx().Project.Instances[iNickname].FlavorName],
						deployProvider.getDeployCtx().Project.Instances[iNickname].ImageId)
					logChan <- string(logMsg)
					errChan <- err
					<-sem
				}(deployProvider.getDeployCtx().Project, cOut, errChan, iNickname)
			}
		case CmdDeleteInstances:
			logMsgBastionIp, err := deployProvider.PopulateInstanceExternalAddressByName()
			cOut <- string(logMsgBastionIp)
			if err != nil {
				cErr <- err.Error()
				return err
			}
			for iNickname := range instances {
				<-throttle.C
				sem <- 1
				go func(project *prj.Project, logChan chan<- string, errChan chan<- error, iNickname string) {
					logMsg, err := deployProvider.DeleteInstance(iNickname, execArgs.IgnoreAttachedVolumes)
					logChan <- string(logMsg)
					errChan <- err
					<-sem
				}(deployProvider.getDeployCtx().Project, cOut, errChan, iNickname)
			}
		case CmdCreateSnapshotImages:
			for iNickname := range instances {
				<-throttle.C
				sem <- 1
				go func(project *prj.Project, logChan chan<- string, errChan chan<- error, iNickname string) {
					logMsg, err := deployProvider.CreateSnapshotImage(iNickname)
					logChan <- string(logMsg)
					errChan <- err
					<-sem
				}(deployProvider.getDeployCtx().Project, cOut, errChan, iNickname)
			}
		case CmdCreateInstancesFromSnapshotImages:
			for iNickname := range instances {
				<-throttle.C
				sem <- 1
				go func(project *prj.Project, logChan chan<- string, errChan chan<- error, iNickname string) {
					logMsg, err := deployProvider.CreateInstanceFromSnapshotImageAndWaitForCompletion(iNickname,
						usedFlavors[deployProvider.getDeployCtx().Project.Instances[iNickname].FlavorName])
					logChan <- string(logMsg)
					errChan <- err
					<-sem
				}(deployProvider.getDeployCtx().Project, cOut, errChan, iNickname)
			}
		case CmdDeleteSnapshotImages:
			for iNickname := range instances {
				<-throttle.C
				sem <- 1
				go func(project *prj.Project, logChan chan<- string, errChan chan<- error, iNickname string) {
					logMsg, err := deployProvider.DeleteSnapshotImage(iNickname)
					logChan <- string(logMsg)
					errChan <- err
					<-sem
				}(deployProvider.getDeployCtx().Project, cOut, errChan, iNickname)
			}
		default:
			err := fmt.Errorf("unknown create/delete instance command %s", cmd)
			cErr <- err.Error()
			return err
		}
	} else if cmd == CmdPingInstances ||
		cmd == CmdInstallServices ||
		cmd == CmdConfigServices ||
		cmd == CmdStartServices ||
		cmd == CmdStopServices {
		if len(nicknames) == 0 {
			err := fmt.Errorf("not enough args, expected comma-separated list of instances or '*'")
			cErr <- err.Error()
			return err
		}

		instances, err := filterByNickname(nicknames, deployProvider.getDeployCtx().Project.Instances, "instance")
		if err != nil {
			cErr <- err.Error()
			return err
		}

		logMsgBastionIp, err := deployProvider.PopulateInstanceExternalAddressByName()
		cOut <- string(logMsgBastionIp)
		if err != nil {
			cErr <- err.Error()
			return err
		}

		errorsExpected = len(instances)
		errChan = make(chan error, len(instances))
		for _, iDef := range instances {
			<-throttle.C
			sem <- 1
			go func(prj *prj.Project, logChan chan<- string, errChan chan<- error, iDef *prj.InstanceDef) {
				var logMsg l.LogMsg
				var err error
				switch cmd {
				case CmdPingInstances:
					logMsg, err = pingOneHost(deployProvider.getDeployCtx().Project.SshConfig, iDef.BestIpAddress(), execArgs.Verbosity, execArgs.NumberOfRepetitions)

				case CmdInstallServices:
					// Make sure ping passes
					logMsg, err = pingOneHost(deployProvider.getDeployCtx().Project.SshConfig, iDef.BestIpAddress(), execArgs.Verbosity, 5)

					// If ping passed, it's ok to move on
					if err == nil {
						logMsg, err = rexec.ExecEmbeddedScriptsOnInstance(deployProvider.getDeployCtx().Project.SshConfig, iDef.BestIpAddress(), iDef.Service.Cmd.Install, iDef.Service.Env, execArgs.Verbosity)
					}

				case CmdConfigServices:
					logMsg, err = rexec.ExecEmbeddedScriptsOnInstance(deployProvider.getDeployCtx().Project.SshConfig, iDef.BestIpAddress(), iDef.Service.Cmd.Config, iDef.Service.Env, execArgs.Verbosity)

				case CmdStartServices:
					logMsg, err = rexec.ExecEmbeddedScriptsOnInstance(deployProvider.getDeployCtx().Project.SshConfig, iDef.BestIpAddress(), iDef.Service.Cmd.Start, iDef.Service.Env, execArgs.Verbosity)

				case CmdStopServices:
					logMsg, err = rexec.ExecEmbeddedScriptsOnInstance(deployProvider.getDeployCtx().Project.SshConfig, iDef.BestIpAddress(), iDef.Service.Cmd.Stop, iDef.Service.Env, execArgs.Verbosity)

				default:
					err = fmt.Errorf("unknown service command:%s", cmd)
				}

				logChan <- string(logMsg)
				errChan <- err
				<-sem
			}(deployProvider.getDeployCtx().Project, cOut, errChan, iDef)
		}

	} else if cmd == CmdCreateVolumes || cmd == CmdAttachVolumes || cmd == CmdDetachVolumes || cmd == CmdDeleteVolumes {
		if len(nicknames) == 0 {
			err := fmt.Errorf("not enough args, expected comma-separated list of instances or '*'")
			cErr <- err.Error()
			return err
		}

		instances, err := filterByNickname(nicknames, deployProvider.getDeployCtx().Project.Instances, "instance")
		if err != nil {
			cErr <- err.Error()
			return err
		}

		volCount := 0
		for _, iDef := range instances {
			volCount += len(iDef.Volumes)
		}
		if volCount == 0 {
			fmt.Printf("No volumes to create/attach/detach/delete")
			return nil
		}
		errorsExpected = volCount
		errChan = make(chan error, volCount)
		for iNickname, iDef := range instances {
			for volNickname := range iDef.Volumes {
				<-throttle.C
				sem <- 1
				switch cmd {
				case CmdCreateVolumes:
					go func(project *prj.Project, logChan chan<- string, errChan chan<- error, iNickname string, volNickname string) {
						logMsg, err := deployProvider.CreateVolume(iNickname, volNickname)
						logChan <- string(logMsg)
						errChan <- err
						<-sem
					}(deployProvider.getDeployCtx().Project, cOut, errChan, iNickname, volNickname)
				case CmdAttachVolumes:
					logMsgBastionIp, err := deployProvider.PopulateInstanceExternalAddressByName()
					cOut <- string(logMsgBastionIp)
					if err != nil {
						cErr <- err.Error()
						return err
					}
					go func(project *prj.Project, logChan chan<- string, errChan chan<- error, iNickname string, volNickname string) {
						logMsg, err := deployProvider.AttachVolume(iNickname, volNickname)
						logChan <- string(logMsg)
						errChan <- err
						<-sem
					}(deployProvider.getDeployCtx().Project, cOut, errChan, iNickname, volNickname)
				case CmdDetachVolumes:
					logMsgBastionIp, err := deployProvider.PopulateInstanceExternalAddressByName()
					cOut <- string(logMsgBastionIp)
					if err != nil {
						cErr <- err.Error()
						return err
					}
					go func(project *prj.Project, logChan chan<- string, errChan chan<- error, iNickname string, volNickname string) {
						logMsg, err := deployProvider.DetachVolume(iNickname, volNickname)
						logChan <- string(logMsg)
						errChan <- err
						<-sem
					}(deployProvider.getDeployCtx().Project, cOut, errChan, iNickname, volNickname)
				case CmdDeleteVolumes:
					go func(project *prj.Project, logChan chan<- string, errChan chan<- error, iNickname string, volNickname string) {
						logMsg, err := deployProvider.DeleteVolume(iNickname, volNickname)
						logChan <- string(logMsg)
						errChan <- err
						<-sem
					}(deployProvider.getDeployCtx().Project, cOut, errChan, iNickname, volNickname)
				default:
					err := fmt.Errorf("unknown cmd %s", cmd)
					cErr <- err.Error()
					return err
				}
			}
		}
	} else {
		err := fmt.Errorf("unknown cmd %s", cmd)
		cErr <- err.Error()
		return err
	}

	// Wait for all workers to finish

	var finalCmdErr error
	for errorsExpected > 0 {
		cmdErr := <-errChan
		if cmdErr != nil {
			cErr <- cmdErr.Error()
			finalCmdErr = cmdErr
		}
		errorsExpected--
	}

	if execArgs.ShowProjectDetails {
		prjJsonBytes, err := json.MarshalIndent(deployProvider.getDeployCtx().Project, "", "    ")
		if err != nil {
			return fmt.Errorf("cannot show project json: %s", err.Error())
		}
		cOut <- string(prjJsonBytes)
	}

	if finalCmdErr != nil {
		cOut <- fmt.Sprintf("%s %sERROR%s, elapsed %.3fs", cmd, l.LogColorRed, l.LogColorReset, time.Since(cmdStartTs).Seconds())
	} else {
		cOut <- fmt.Sprintf("%s %sOK%s, elapsed %.3fs", cmd, l.LogColorGreen, l.LogColorReset, time.Since(cmdStartTs).Seconds())
	}

	return finalCmdErr
}
