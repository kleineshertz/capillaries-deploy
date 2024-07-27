package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/capillariesio/capillaries-deploy/pkg/cld/cldaws"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
)

func ensureFloatingIp(ec2Client *ec2.Client, goCtx context.Context, tags map[string]string, lb *l.LogBuilder, ipName string) (string, error) {
	existingIp, _, _, err := cldaws.GetPublicIpAddressAllocationAssociatedInstanceByName(ec2Client, goCtx, lb, ipName)
	if err != nil {
		return "", err
	}
	if existingIp != "" {
		return existingIp, nil
	}
	return cldaws.AllocateFloatingIpByName(ec2Client, goCtx, tags, lb, ipName)
}

func (p *AwsDeployProvider) CreateFloatingIps() (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.DeployCtx.IsVerbose)

	bastionIpName := p.DeployCtx.Project.SshConfig.BastionExternalIpAddressName
	bastionIpAddress, err := ensureFloatingIp(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, p.DeployCtx.Tags, lb, bastionIpName)
	if err != nil {
		return lb.Complete(err)
	}

	p.DeployCtx.Project.SshConfig.BastionExternalIp = bastionIpAddress

	// Tell the user about the bastion IP
	lb.AddAlways(fmt.Sprintf(`
Public IP reserved, now you can use it for SSH jumphost in your ~/.ssh/config:

Host %s
User %s
StrictHostKeyChecking=no
UserKnownHostsFile=/dev/null
IdentityFile <private key path>

Also, you may find it convenient to use in your commands:

export BASTION_IP=%s

`,
		p.DeployCtx.Project.SshConfig.BastionExternalIp,
		p.DeployCtx.Project.SshConfig.User,
		p.DeployCtx.Project.SshConfig.BastionExternalIp))

	natgwIpName := p.DeployCtx.Project.Network.PublicSubnet.NatGatewayExternalIpName
	_, err = ensureFloatingIp(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, p.DeployCtx.Tags, lb, natgwIpName)
	if err != nil {
		return lb.Complete(err)
	}

	return lb.Complete(nil)
}

func releaseFloatingIpIfNotAllocated(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, ipName string) error {
	existingIp, existingIpAllocationId, existingIpAssociatedInstance, err := cldaws.GetPublicIpAddressAllocationAssociatedInstanceByName(ec2Client, goCtx, lb, ipName)
	if err != nil {
		return err
	}
	if existingIp == "" {
		return fmt.Errorf("cannot release ip named %s, it was not allocated", ipName)
	}
	if existingIpAssociatedInstance != "" {
		return fmt.Errorf("cannot release ip named %s, it is associated with instance %s", ipName, existingIpAssociatedInstance)
	}
	return cldaws.ReleaseFloatingIpByAllocationId(ec2Client, goCtx, lb, existingIpAllocationId)
}

func (p *AwsDeployProvider) DeleteFloatingIps() (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.DeployCtx.IsVerbose)

	bastionIpName := p.DeployCtx.Project.SshConfig.BastionExternalIpAddressName
	err := releaseFloatingIpIfNotAllocated(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, bastionIpName)
	if err != nil {
		return lb.Complete(err)
	}

	err = releaseFloatingIpIfNotAllocated(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, p.DeployCtx.Project.Network.PublicSubnet.NatGatewayExternalIpName)
	if err != nil {
		return lb.Complete(err)
	}
	//p.GetCtx().PrjPair.SetPublicSubnetNatGatewayExternalIp("")

	return lb.Complete(nil)
}

// func (p *Project) SetSshBastionExternalIp(ipName string, newIp string) {
// 	//prjPair.Template.SshConfig.BastionExternalIp = newIp
// 	p.SshConfig.BastionExternalIp = newIp

// 	// for _, iDef := range prjPair.Template.Instances {
// 	// 	if iDef.ExternalIpAddressName == ipName {
// 	// 		iDef.ExternalIpAddress = newIp
// 	// 	}
// 	// }
// 	for _, iDef := range p.Instances {
// 		if iDef.ExternalIpAddressName == ipName {
// 			iDef.ExternalIpAddress = newIp
// 		}

// 		// In env variables
// 		replaceMap := map[string]string{}
// 		for varName, varValue := range iDef.Service.Env {
// 			if strings.Contains(varValue, "{CAPIDEPLOY.INTERNAL.BASTION_EXTERNAL_IP_ADDRESS}") {
// 				replaceMap[varName] = strings.ReplaceAll(varValue, "{CAPIDEPLOY.INTERNAL.BASTION_EXTERNAL_IP_ADDRESS}", newIp)
// 			}
// 		}
// 		for varName, varValue := range replaceMap {
// 			iDef.Service.Env[varName] = varValue
// 		}
// 	}

// }

func (p *AwsDeployProvider) PopulateInstanceExternalAddressByName() (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.DeployCtx.IsVerbose)
	ipAddressName := p.DeployCtx.Project.SshConfig.BastionExternalIpAddressName
	ipAddress, _, _, err := cldaws.GetPublicIpAddressAllocationAssociatedInstanceByName(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, ipAddressName)
	if err != nil {
		return lb.Complete(err)
	}

	if ipAddress == "" {
		return lb.Complete(fmt.Errorf("ip address %s was not allocated, did you call create_public_ips?", ipAddressName))
	}

	// Updates project: ssh config
	p.DeployCtx.Project.SshConfig.BastionExternalIp = ipAddress

	// Updates project: instances
	for _, iDef := range p.DeployCtx.Project.Instances {
		if iDef.ExternalIpAddressName == ipAddressName {
			iDef.ExternalIpAddress = ipAddress
		}

		// In env variables
		replaceMap := map[string]string{}
		for varName, varValue := range iDef.Service.Env {
			if strings.Contains(varValue, "{CAPIDEPLOY.INTERNAL.BASTION_EXTERNAL_IP_ADDRESS}") {
				replaceMap[varName] = strings.ReplaceAll(varValue, "{CAPIDEPLOY.INTERNAL.BASTION_EXTERNAL_IP_ADDRESS}", ipAddress)
			}
		}
		for varName, varValue := range replaceMap {
			iDef.Service.Env[varName] = varValue
		}
	}

	return lb.Complete(nil)
}
