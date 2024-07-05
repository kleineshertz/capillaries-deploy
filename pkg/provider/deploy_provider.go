package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	taggingTypes "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/capillariesio/capillaries-deploy/pkg/cld/cldaws"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
	"github.com/capillariesio/capillaries-deploy/pkg/prj"
)

type AwsCtx struct {
	Config        aws.Config
	Ec2Client     *ec2.Client
	TaggingClient *resourcegroupstaggingapi.Client
}

type DeployCtx struct {
	//PrjPair   *prj.ProjectPair
	Project   *prj.Project
	GoCtx     context.Context
	IsVerbose bool
	Aws       *AwsCtx
	Tags      map[string]string
}
type DeployProvider interface {
	GetCtx() *DeployCtx
	ListDeployments() (l.LogMsg, error)
	ListDeploymentResources() (l.LogMsg, error)
	CreateFloatingIps() (l.LogMsg, error)
	DeleteFloatingIps() (l.LogMsg, error)
	CreateSecurityGroups() (l.LogMsg, error)
	DeleteSecurityGroups() (l.LogMsg, error)
	CreateNetworking() (l.LogMsg, error)
	DeleteNetworking() (l.LogMsg, error)
	HarvestInstanceTypesByFlavorNames(flavorMap map[string]string) (l.LogMsg, error)
	HarvestImageIds(imageMap map[string]bool) (l.LogMsg, error)
	VerifyKeypairs(keypairMap map[string]struct{}) (l.LogMsg, error)
	CreateInstanceAndWaitForCompletion(iNickname string, flavorId string, imageId string) (l.LogMsg, error)
	DeleteInstance(iNickname string, ignoreAttachedVolumes bool) (l.LogMsg, error)
	CreateSnapshotImage(iNickname string) (l.LogMsg, error)
	CreateInstanceFromSnapshotImageAndWaitForCompletion(iNickname string, flavorId string) (l.LogMsg, error)
	DeleteSnapshotImage(iNickname string) (l.LogMsg, error)
	CreateVolume(iNickname string, volNickname string) (l.LogMsg, error)
	AttachVolume(iNickname string, volNickname string) (l.LogMsg, error)
	DetachVolume(iNickname string, volNickname string) (l.LogMsg, error)
	DeleteVolume(iNickname string, volNickname string) (l.LogMsg, error)
	PopulateInstanceExternalAddressByName() (l.LogMsg, error)
}

type AwsDeployProvider struct {
	Ctx *DeployCtx
}

func (p *AwsDeployProvider) GetCtx() *DeployCtx {
	return p.Ctx
}

type AssumeRoleConfig struct {
	RoleArn    string
	ExternalId string
}

func DeployProviderFactory(project *prj.Project, goCtx context.Context, assumeRoleCfg *AssumeRoleConfig, isVerbose bool) (DeployProvider, error) {
	if project.DeployProviderName == prj.DeployProviderAws {
		cfg, err := config.LoadDefaultConfig(goCtx)
		if err != nil {
			return nil, err
		}

		if assumeRoleCfg != nil && assumeRoleCfg.RoleArn != "" {
			creds := stscreds.NewAssumeRoleProvider(sts.NewFromConfig(cfg), assumeRoleCfg.RoleArn,
				func(o *stscreds.AssumeRoleOptions) {
					o.ExternalID = aws.String(assumeRoleCfg.ExternalId)
					o.RoleSessionName = "third-party-capideploy-assumes-role-provided-by-customer"
				})
			cfg.Credentials = aws.NewCredentialsCache(creds)
		}

		return &AwsDeployProvider{
			Ctx: &DeployCtx{
				Project:   project,
				GoCtx:     goCtx,
				IsVerbose: isVerbose,
				Tags: map[string]string{
					cldaws.DeploymentNameTagName:     project.DeploymentName,
					cldaws.DeploymentOperatorTagName: cldaws.DeploymentOperatorTagValue},
				Aws: &AwsCtx{
					Ec2Client:     ec2.NewFromConfig(cfg),
					TaggingClient: resourcegroupstaggingapi.NewFromConfig(cfg),
				},
			},
		}, nil
	}
	return nil, fmt.Errorf("unsupported deploy provider %s", project.DeployProviderName)
}

func reportPublicIp(prj *prj.Project) {
	fmt.Printf(`
Public IP reserved, now you can use it for SSH jumphost in your ~/.ssh/config:

Host %s
  User %s
  StrictHostKeyChecking=no
  UserKnownHostsFile=/dev/null
  IdentityFile %s

Also, you may find it convenient to use in your commands:

export BASTION_IP=%s

`,
		prj.SshConfig.BastionExternalIp,
		prj.SshConfig.User,
		prj.SshConfig.PrivateKeyPath,
		prj.SshConfig.BastionExternalIp)
}
func (p *AwsDeployProvider) ListDeployments() (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)
	resources, err := cldaws.GetResourcesByTag(p.GetCtx().Aws.TaggingClient, p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, p.GetCtx().Aws.Config.Region,
		[]taggingTypes.TagFilter{{Key: aws.String(cldaws.DeploymentOperatorTagName), Values: []string{cldaws.DeploymentOperatorTagValue}}}, false)
	if err != nil {
		return lb.Complete(err)
	}
	deploymentResCount := map[string]int{}
	for _, res := range resources {
		if deploymentNameCount, ok := deploymentResCount[res.DeploymentName]; ok {
			deploymentResCount[res.DeploymentName] = deploymentNameCount + 1
		} else {
			deploymentResCount[res.DeploymentName] = 1
		}
	}
	deploymentStrings := make([]string, len(deploymentResCount))
	deploymentIdx := 0
	totalResourceCount := 0
	for deploymentName, deploymentResCount := range deploymentResCount {
		deploymentStrings[deploymentIdx] = fmt.Sprintf("%s,%d", deploymentName, deploymentResCount)
		deploymentIdx++
		totalResourceCount += deploymentResCount
	}
	fmt.Printf("%s\n", strings.Join(deploymentStrings, "\n"))
	fmt.Printf("Deployments: %d, resources: %d\n", len(deploymentResCount), totalResourceCount)
	return lb.Complete(nil)
}

func (p *AwsDeployProvider) ListDeploymentResources() (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)
	resources, err := cldaws.GetResourcesByTag(p.GetCtx().Aws.TaggingClient, p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, p.GetCtx().Aws.Config.Region,
		[]taggingTypes.TagFilter{
			{Key: aws.String(cldaws.DeploymentOperatorTagName), Values: []string{cldaws.DeploymentOperatorTagValue}},
			{Key: aws.String(cldaws.DeploymentNameTagName), Values: []string{p.Ctx.Project.DeploymentName}}}, true)
	if err != nil {
		return lb.Complete(err)
	}
	resourceStrings := make([]string, len(resources))
	activeCount := 0
	for resIdx, res := range resources {
		resourceStrings[resIdx] = res.String()
		if res.BilledState != cldaws.ResourceBilledStateTerminated {
			activeCount++
		}
	}
	fmt.Printf("%s\n", strings.Join(resourceStrings, "\n"))
	fmt.Printf("Total: %d, potentially billed: %d\n", len(resources), activeCount)
	return lb.Complete(nil)
}
