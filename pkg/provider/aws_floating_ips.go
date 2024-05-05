package provider

import (
	"github.com/capillariesio/capillaries-deploy/pkg/cld/cldaws"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
)

func (p *AwsDeployProvider) CreateFloatingIps() (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)

	bastionIp, err := cldaws.AllocateFloatingIp(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, p.GetCtx().Tags, lb, "bastion")
	if err != nil {
		return lb.Complete(err)
	}
	p.GetCtx().PrjPair.SetSshExternalIp(bastionIp)

	natgwIp, err := cldaws.AllocateFloatingIp(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, p.GetCtx().Tags, lb, "natgw")
	if err != nil {
		return lb.Complete(err)
	}
	p.GetCtx().PrjPair.SetNatGatewayExternalIp(natgwIp)

	// Tell the user about the bastion IP
	reportPublicIp(&p.GetCtx().PrjPair.Live)

	return lb.Complete(nil)
}

func (p *AwsDeployProvider) DeleteFloatingIps() (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)

	err := cldaws.ReleaseFloatingIp(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, p.GetCtx().PrjPair.Live.SshConfig.ExternalIpAddress)
	if err != nil {
		return lb.Complete(err)
	}
	p.GetCtx().PrjPair.SetSshExternalIp("")

	err = cldaws.ReleaseFloatingIp(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, p.GetCtx().PrjPair.Live.Network.PublicSubnet.NatGatewayPublicIp)
	if err != nil {
		return lb.Complete(err)
	}
	p.GetCtx().PrjPair.SetNatGatewayExternalIp("")

	return lb.Complete(nil)
}
