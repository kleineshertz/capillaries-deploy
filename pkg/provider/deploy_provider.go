package provider

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
	"github.com/capillariesio/capillaries-deploy/pkg/prj"
	"github.com/capillariesio/capillaries-deploy/pkg/sh"
)

type AwsCtx struct {
	Config    aws.Config
	Ec2Client *ec2.Client
}
type DeployCtx struct {
	PrjPair   *prj.ProjectPair
	GoCtx     context.Context
	IsVerbose bool
	Aws       *AwsCtx
}
type DeployProvider interface {
	GetCtx() *DeployCtx
	BuildArtifacts() (l.LogMsg, error)
	CreateFloatingIps() (l.LogMsg, error)
	DeleteFloatingIps() (l.LogMsg, error)
	CreateSecurityGroups() (l.LogMsg, error)
	DeleteSecurityGroups() (l.LogMsg, error)
	CreateNetworking() (l.LogMsg, error)
	DeleteNetworking() (l.LogMsg, error)
	HarvestInstanceTypesByFlavorNames(flavorMap map[string]string) (l.LogMsg, error)
	HarvestImageIdsByImageNames(imageMap map[string]string) (l.LogMsg, error)
	VerifyKeypairs(keypairMap map[string]struct{}) (l.LogMsg, error)
	CreateInstanceAndWaitForCompletion(iNickname string, flavorId string, imageId string) (l.LogMsg, error)
	DeleteInstance(iNickname string) (l.LogMsg, error)
	CreateVolume(iNickname string, volNickname string) (l.LogMsg, error)
	AttachVolume(iNickname string, volNickname string) (l.LogMsg, error)
	DeleteVolume(iNickname string, volNickname string) (l.LogMsg, error)
}

type OpenstackDeployProvider struct{}

type AwsDeployProvider struct {
	Ctx *DeployCtx
}

func (p *AwsDeployProvider) GetCtx() *DeployCtx {
	return p.Ctx
}
func (p *AwsDeployProvider) BuildArtifacts() (l.LogMsg, error) {
	lb := l.NewLogBuilder("BuildArtifacts", p.GetCtx().IsVerbose)
	for _, cmd := range p.GetCtx().PrjPair.Live.Artifacts.Cmd {
		err := sh.ExecEmbeddedScriptLocally(lb, cmd, []string{},
			p.GetCtx().PrjPair.Live.Artifacts.Env,
			p.GetCtx().IsVerbose,
			p.GetCtx().PrjPair.Live.Timeouts.LocalCommand)
		if err != nil {
			return lb.Complete(err)
		}
	}
	return lb.Complete(nil)
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
				Aws: &AwsCtx{
					Ec2Client: ec2.NewFromConfig(cfg),
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
