package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudcontrol"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	"github.com/capillariesio/capillaries-deploy/pkg/cld/cldaws"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
	"github.com/capillariesio/capillaries-deploy/pkg/prj"
)

type AwsCtx struct {
	Config             aws.Config
	Ec2Client          *ec2.Client
	TaggingClient      *resourcegroupstaggingapi.Client
	CloudControlClient *cloudcontrol.Client
}

const TagCapiDeploy string = "CapiDeploy"

type DeployCtx struct {
	PrjPair   *prj.ProjectPair
	GoCtx     context.Context
	IsVerbose bool
	Aws       *AwsCtx
	Tags      map[string]string
}
type DeployProvider interface {
	GetCtx() *DeployCtx
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
	DeleteInstance(iNickname string) (l.LogMsg, error)
	CreateSnapshotImage(iNickname string) (l.LogMsg, error)
	CreateInstanceFromSnapshotImageAndWaitForCompletion(iNickname string, flavorId string) (l.LogMsg, error)
	DeleteSnapshotImage(iNickname string) (l.LogMsg, error)
	CreateVolume(iNickname string, volNickname string) (l.LogMsg, error)
	AttachVolume(iNickname string, volNickname string) (l.LogMsg, error)
	DetachVolume(iNickname string, volNickname string) (l.LogMsg, error)
	DeleteVolume(iNickname string, volNickname string) (l.LogMsg, error)
}

type AwsDeployProvider struct {
	Ctx *DeployCtx
}

func (p *AwsDeployProvider) GetCtx() *DeployCtx {
	return p.Ctx
}

func DeployProviderFactory(prjPair *prj.ProjectPair, goCtx context.Context, isVerbose bool) (DeployProvider, error) {
	if prjPair.Live.DeployProviderName == prj.DeployProviderAws {
		cfg, err := config.LoadDefaultConfig(goCtx)
		if err != nil {
			return nil, err
		}

		return &AwsDeployProvider{
			Ctx: &DeployCtx{
				PrjPair:   prjPair,
				GoCtx:     goCtx,
				IsVerbose: isVerbose,
				Tags:      map[string]string{TagCapiDeploy: prjPair.Live.DeploymentName},
				Aws: &AwsCtx{
					Ec2Client:          ec2.NewFromConfig(cfg),
					TaggingClient:      resourcegroupstaggingapi.NewFromConfig(cfg),
					CloudControlClient: cloudcontrol.NewFromConfig(cfg),
				},
			},
		}, nil
	}
	return nil, fmt.Errorf("unsupported deploy provider %s", prjPair.Live.DeployProviderName)
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
		prj.SshConfig.ExternalIpAddress,
		prj.SshConfig.User,
		prj.SshConfig.PrivateKeyPath,
		prj.SshConfig.ExternalIpAddress)
}

func (p *AwsDeployProvider) ListDeploymentResources() (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)
	resources, err := cldaws.GetResourcesByTag(p.GetCtx().Aws.TaggingClient, p.GetCtx().Aws.CloudControlClient, p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, p.GetCtx().Aws.Config.Region, TagCapiDeploy, p.Ctx.PrjPair.Live.DeploymentName)
	if err != nil {
		return lb.Complete(err)
	}
	fmt.Printf("%s\n", strings.Join(resources, "\n"))
	return lb.Complete(nil)
}
