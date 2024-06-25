package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/capillariesio/capillaries-deploy/pkg/cld/cldaws"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
)

func ensureFloatingIp(client *ec2.Client, goCtx context.Context, tags map[string]string, lb *l.LogBuilder, ipName string) (string, error) {
	existingIp, _, _, err := cldaws.GetPublicIpAddressAllocationAssociatedInstanceByName(client, goCtx, lb, ipName)
	if err != nil {
		return "", err
	}
	if existingIp != "" {
		return existingIp, nil
	}
	return cldaws.AllocateFloatingIpByName(client, goCtx, tags, lb, ipName)
}

func (p *AwsDeployProvider) CreateFloatingIps() (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)

	bastionIpName := p.GetCtx().Project.SshConfig.BastionExternalIpAddressName
	bastionIpAddress, err := ensureFloatingIp(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, p.GetCtx().Tags, lb, bastionIpName)
	if err != nil {
		return lb.Complete(err)
	}

	p.GetCtx().Project.SshConfig.BastionExternalIp = bastionIpAddress

	// Tell the user about the bastion IP
	reportPublicIp(p.GetCtx().Project)

	natgwIpName := p.GetCtx().Project.Network.PublicSubnet.NatGatewayExternalIpName
	_, err = ensureFloatingIp(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, p.GetCtx().Tags, lb, natgwIpName)
	if err != nil {
		return lb.Complete(err)
	}

	return lb.Complete(nil)
}

func releaseFloatingIpIfNotAllocated(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, ipName string) error {
	existingIp, existingIpAllocationId, existingIpAssociatedInstance, err := cldaws.GetPublicIpAddressAllocationAssociatedInstanceByName(client, goCtx, lb, ipName)
	if err != nil {
		return err
	}
	if existingIp == "" {
		return fmt.Errorf("cannot release ip named %s, it was not allocated", ipName)
	}
	if existingIpAssociatedInstance != "" {
		return fmt.Errorf("cannot release ip named %s, it is associated with instance %s", ipName, existingIpAssociatedInstance)
	}
	return cldaws.ReleaseFloatingIpByAllocationId(client, goCtx, lb, existingIpAllocationId)
}

func (p *AwsDeployProvider) DeleteFloatingIps() (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)

	bastionIpName := p.GetCtx().Project.SshConfig.BastionExternalIpAddressName
	err := releaseFloatingIpIfNotAllocated(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, bastionIpName)
	if err != nil {
		return lb.Complete(err)
	}

	err = releaseFloatingIpIfNotAllocated(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, p.GetCtx().Project.Network.PublicSubnet.NatGatewayExternalIpName)
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
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)
	ipAddressName := p.GetCtx().Project.SshConfig.BastionExternalIpAddressName
	ipAddress, _, _, err := cldaws.GetPublicIpAddressAllocationAssociatedInstanceByName(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, ipAddressName)
	if err != nil {
		return lb.Complete(err)
	}

	if ipAddress == "" {
		return lb.Complete(fmt.Errorf("ip address %s was not allocated, did you call create_public_ips?", ipAddressName))
	}

	// Updates project: ssh config
	p.GetCtx().Project.SshConfig.BastionExternalIp = ipAddress

	// Updates project: instances
	for _, iDef := range p.GetCtx().Project.Instances {
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
