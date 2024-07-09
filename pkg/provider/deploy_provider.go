package provider

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	taggingTypes "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/capillariesio/capillaries-deploy/pkg/cld"
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
	ListDeployments() (map[string]int, l.LogMsg, error)
	ListDeploymentResources() ([]*cld.Resource, l.LogMsg, error)
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
	RoleArn    string `json:"role_arn"`
	ExternalId string `json:"external_id"`
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
func (p *AwsDeployProvider) ListDeployments() (map[string]int, l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)
	resources, err := cldaws.GetResourcesByTag(p.GetCtx().Aws.TaggingClient, p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, p.GetCtx().Aws.Config.Region,
		[]taggingTypes.TagFilter{{Key: aws.String(cld.DeploymentOperatorTagName), Values: []string{cld.DeploymentOperatorTagValue}}}, false)
	if err != nil {
		logMsg, err := lb.Complete(err)
		return nil, logMsg, err
	}
	deploymentResCount := map[string]int{}
	for _, res := range resources {
		if deploymentNameCount, ok := deploymentResCount[res.DeploymentName]; ok {
			deploymentResCount[res.DeploymentName] = deploymentNameCount + 1
		} else {
			deploymentResCount[res.DeploymentName] = 1
		}
	}
	logMsg, _ := lb.Complete(nil)
	return deploymentResCount, logMsg, nil
}

func (p *AwsDeployProvider) ListDeploymentResources() ([]*cld.Resource, l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)
	resources, err := cldaws.GetResourcesByTag(p.GetCtx().Aws.TaggingClient, p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, p.GetCtx().Aws.Config.Region,
		[]taggingTypes.TagFilter{
			{Key: aws.String(cld.DeploymentOperatorTagName), Values: []string{cld.DeploymentOperatorTagValue}},
			{Key: aws.String(cld.DeploymentNameTagName), Values: []string{p.Ctx.Project.DeploymentName}}}, true)
	if err != nil {
		logMsg, err := lb.Complete(err)
		return nil, logMsg, err
	}
	logMsg, _ := lb.Complete(nil)
	return resources, logMsg, nil
}
