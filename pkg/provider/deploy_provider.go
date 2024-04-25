package provider

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
	"github.com/capillariesio/capillaries-deploy/pkg/prj"
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
	CreateFloatingIp() (l.LogMsg, error)
	DeleteFloatingIps() (l.LogMsg, error)
	CreateSecurityGroups() (l.LogMsg, error)
	DeleteSecurityGroups() (l.LogMsg, error)
	CreateNetworking() (l.LogMsg, error)
	DeleteNetworking() (l.LogMsg, error)
	GetFlavorIds(flavorMap map[string]string) (l.LogMsg, error)
	GetImageIds(imageMap map[string]string) (l.LogMsg, error)
	VerifyKeypairs(keypairMap map[string]struct{}) (l.LogMsg, error)
	CreateInstanceAndWaitForCompletion(iNickname string, flavorId string, imageId string) (l.LogMsg, error)
	DeleteInstance(iNickname string) (l.LogMsg, error)
	CreateVolume(prjPair *ProjectPair, iNickname string, volNickname string, isVerbose bool) (LogMsg, error)
	AttachVolume(prjPair *ProjectPair, iNickname string, volNickname string, isVerbose bool) (LogMsg, error)
	DeleteVolume(prjPair *ProjectPair, iNickname string, volNickname string, isVerbose bool) (LogMsg, error)
}

type OpenstackDeployProvider struct{}

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
