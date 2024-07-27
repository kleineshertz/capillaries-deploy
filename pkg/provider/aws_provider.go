package provider

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	"github.com/capillariesio/capillaries-deploy/pkg/cld"
)

// AWS-specific

type AwsCtx struct {
	Config        aws.Config
	Ec2Client     *ec2.Client
	TaggingClient *resourcegroupstaggingapi.Client
}

// Everything below is generic. This type will support DeployProvider (public) and deployProviderImpl (internal)

type AwsDeployProvider struct {
	DeployCtx *DeployCtx
}

func (p *AwsDeployProvider) getDeployCtx() *DeployCtx {
	return p.DeployCtx
}

// DeployProvider implementation

func (p *AwsDeployProvider) ListDeployments(cOut chan<- string, cErr chan<- string) (map[string]int, error) {
	return genericListDeployments(p, cOut, cErr)
}

func (p *AwsDeployProvider) ListDeploymentResources(cOut chan<- string, cErr chan<- string) ([]*cld.Resource, error) {
	return genericListDeploymentResources(p, cOut, cErr)
}

func (p *AwsDeployProvider) ExecCmdWithNoResult(cmd string, nicknames string, execArgs *ExecArgs, cOut chan<- string, cErr chan<- string) error {
	return genericExecCmdWithNoResult(p, cmd, nicknames, execArgs, cOut, cErr)
}
