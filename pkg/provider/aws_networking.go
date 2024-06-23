package provider

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/capillariesio/capillaries-deploy/pkg/cld/cldaws"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
	"github.com/capillariesio/capillaries-deploy/pkg/prj"
)

func ensureAwsVpc(client *ec2.Client, goCtx context.Context, tags map[string]string, lb *l.LogBuilder, networkDef *prj.NetworkDef, timeout int) (string, error) {
	foundVpcIdByName, err := cldaws.GetVpcIdByName(client, goCtx, lb, networkDef.Name)
	if err != nil {
		return "", err
	}
	if foundVpcIdByName != "" {
		return foundVpcIdByName, nil
	}
	return cldaws.CreateVpc(client, goCtx, tags, lb, networkDef.Name, networkDef.Cidr, timeout)
}

func ensureAwsPrivateSubnet(client *ec2.Client, goCtx context.Context, tags map[string]string, lb *l.LogBuilder, networkId string, subnetDef *prj.PrivateSubnetDef) (string, error) {
	foundSubnetIdByName, err := cldaws.GetSubnetIdByName(client, goCtx, lb, subnetDef.Name)
	if err != nil {
		return "", err
	}
	if foundSubnetIdByName != "" {
		return foundSubnetIdByName, nil
	}
	return cldaws.CreateSubnet(client, goCtx, tags, lb, networkId, subnetDef.Name, subnetDef.Cidr, subnetDef.AvailabilityZone)
}

func ensureAwsPublicSubnet(client *ec2.Client, goCtx context.Context, tags map[string]string, lb *l.LogBuilder, networkId string, subnetDef *prj.PublicSubnetDef) (string, error) {
	foundSubnetIdByName, err := cldaws.GetSubnetIdByName(client, goCtx, lb, subnetDef.Name)
	if err != nil {
		return "", err
	}
	if foundSubnetIdByName != "" {
		return foundSubnetIdByName, nil
	}

	return cldaws.CreateSubnet(client, goCtx, tags, lb, networkId, subnetDef.Name, subnetDef.Cidr, subnetDef.AvailabilityZone)
}

func ensureNatGatewayAndRoutePrivateSubnet(client *ec2.Client, goCtx context.Context, tags map[string]string, lb *l.LogBuilder, networkId string, publicSubnetId string, publicSubnetDef *prj.PublicSubnetDef, privateSubnetId string, privateSubnetDef *prj.PrivateSubnetDef, createNatGatewayTimeout int) error {
	_, natGatewayPublicIpAllocationId, _, err := cldaws.GetPublicIpAddressAllocationAssociatedInstanceByName(client, goCtx, lb, publicSubnetDef.NatGatewayExternalIpName)
	if err != nil {
		return err
	}

	// Get NAT gateway by name, create one if needed

	natGatewayName := publicSubnetDef.NatGatewayName
	natGatewayId, foundNatGatewayStateByName, err := cldaws.GetNatGatewayIdAndStateByName(client, goCtx, lb, natGatewayName)
	if err != nil {
		return err
	}

	if natGatewayId != "" && foundNatGatewayStateByName != types.NatGatewayStateDeleted {
		if foundNatGatewayStateByName != types.NatGatewayStateAvailable {
			return fmt.Errorf("cannot create nat gateway %s, it is already created and has invalid state %s", natGatewayName, foundNatGatewayStateByName)
		}
	} else {
		natGatewayId, err = cldaws.CreateNatGateway(client, goCtx, tags, lb, natGatewayName,
			publicSubnetId,
			natGatewayPublicIpAllocationId,
			createNatGatewayTimeout)
		if err != nil {
			return err
		}
	}

	// Create new route table id for this vpc

	routeTableId, err := cldaws.CreateRouteTableForVpc(client, goCtx, tags, lb, privateSubnetDef.RouteTableToNatgwName, networkId)
	if err != nil {
		return err
	}

	// Associate this route table with the private subnet

	rtAssocId, err := cldaws.AssociateRouteTableWithSubnet(client, goCtx, lb, routeTableId, privateSubnetId)
	if err != nil {
		return err
	}

	lb.Add(fmt.Sprintf("associated route table %s with private subnet %s: %s", routeTableId, privateSubnetId, rtAssocId))

	// Add a record to a route table: tell all outbound 0.0.0.0/0 traffic to go through this nat gateway:

	if err := cldaws.CreateNatGatewayRoute(client, goCtx, lb, routeTableId, "0.0.0.0/0", natGatewayId); err != nil {
		return err
	}

	lb.Add(fmt.Sprintf("route table %s in private subnet %s points to nat gateway %s", routeTableId, privateSubnetId, natGatewayId))

	return nil
}

func ensureInternetGatewayAndRoutePublicSubnet(client *ec2.Client, goCtx context.Context, tags map[string]string, lb *l.LogBuilder,
	routerName string,
	networkId string, publicSubnetId string, publicSubnetDef *prj.PublicSubnetDef) error {

	// Get internet gateway (router) by name, create if needed

	var routerId string
	foundRouterIdByName, err := cldaws.GetInternetGatewayIdByName(client, goCtx, lb, routerName)
	if err != nil {
		return err
	}

	if foundRouterIdByName != "" {
		routerId = foundRouterIdByName
	} else {
		routerId, err = cldaws.CreateInternetGateway(client, goCtx, tags, lb, routerName)
		if err != nil {
			return err
		}
	}

	// Is this internet gateway (router) attached to a vpc?

	attachedVpcId, _, err := cldaws.GetInternetGatewayVpcAttachmentById(client, goCtx, lb, routerId)
	if err != nil {
		return err
	}

	// Attach if needed

	if attachedVpcId == "" {
		if err := cldaws.AttachInternetGatewayToVpc(client, goCtx, lb, routerId, networkId); err != nil {
			return err
		}
	} else if attachedVpcId != networkId {
		return fmt.Errorf("internet gateway (router) %s seems to be attached to a wrong vpc %s already", routerName, attachedVpcId)
	}

	// Obtain route table id for this vpc (it was automatically created for us and marked as 'main')

	routeTableId, err := cldaws.GetVpcDefaultRouteTable(client, goCtx, lb, networkId)
	if err != nil {
		return err
	}

	// (optional) tag this route table for operator's convenience

	routeTableName := publicSubnetDef.Name + "_vpc_default_rt"
	if err := cldaws.TagResource(client, goCtx, lb, routeTableId, routeTableName, nil); err != nil {
		return err
	}

	// Associate this default (main) route table with the public subnet

	assocId, err := cldaws.AssociateRouteTableWithSubnet(client, goCtx, lb, routeTableId, publicSubnetId)
	if err != nil {
		return err
	}
	lb.Add(fmt.Sprintf("associated route table %s with public subnet %s: %s", routeTableId, publicSubnetId, assocId))

	// Add a record to a route table: tell all outbound 0.0.0.0/0 traffic to go through this internet gateway:

	if err := cldaws.CreateInternetGatewayRoute(client, goCtx, lb, routeTableId, "0.0.0.0/0", routerId); err != nil {
		return err
	}
	lb.Add(fmt.Sprintf("route table %s in public subnet %s points to internet gateway (router) %s", routeTableId, publicSubnetId, routerId))

	return nil
}

func detachAndDeleteInternetGateway(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, internetGatewayName string) error {
	foundId, err := cldaws.GetInternetGatewayIdByName(client, goCtx, lb, internetGatewayName)
	if err != nil {
		return err
	}

	if foundId == "" {
		lb.Add(fmt.Sprintf("will not delete internet gateway (router) %s, nothing to delete", internetGatewayName))
		return nil
	}

	// Is it attached to a vpc? If yes, detach it.

	attachedVpcId, attachmentState, err := cldaws.GetInternetGatewayVpcAttachmentById(client, goCtx, lb, foundId)
	if err != nil {
		return err
	}

	// NOTE: for unknown reason, I am getting "available" instead of "attached" here, so let's embrace it
	if attachedVpcId != "" &&
		(attachmentState == types.AttachmentStatusAttached || attachmentState == types.AttachmentStatusAttaching || string(attachmentState) == "available") {

		// This may potentially throw:
		// Network vpc-... has some mapped public address(es). Please unmap those public address(es) before detaching the gateway.
		// if we do not wait for NAT gateway to be deleted completely
		if err := cldaws.DetachInternetGatewayFromVpc(client, goCtx, lb, foundId, attachedVpcId); err != nil {
			return err
		}
		lb.Add(fmt.Sprintf("detached internet gateway (router) %s from vpc %s", foundId, attachedVpcId))
	} else {
		lb.Add(fmt.Sprintf("internet gateway (router) %s was not attached, no need to detach", foundId))
	}

	// Delete
	return cldaws.DeleteInternetGateway(client, goCtx, lb, foundId)
}

func checkAndDeleteNatGateway(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, natGatewayName string, timeout int) error {
	foundId, foundState, err := cldaws.GetNatGatewayIdAndStateByName(client, goCtx, lb, natGatewayName)
	if err != nil {
		return err
	}

	if foundId == "" || foundState == types.NatGatewayStateDeleted {
		lb.Add(fmt.Sprintf("will not delete nat gateway %s, nothing to delete", natGatewayName))
		return nil
	}

	return cldaws.DeleteNatGateway(client, goCtx, lb, foundId, timeout)
}

func deleteAwsSubnet(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, subnetName string) error {
	foundId, err := cldaws.GetSubnetIdByName(client, goCtx, lb, subnetName)
	if err != nil {
		return err
	}
	if foundId == "" {
		lb.Add(fmt.Sprintf("will not delete subnet %s, nothing to delete", subnetName))
		return nil
	}

	return cldaws.DeleteSubnet(client, goCtx, lb, foundId)
}

func checkAndDeleteAwsVpcWithRouteTable(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, vpcName string, privateSubnetName string, privateSubnetRouteTableToNatgwName string) error {
	foundVpcId, err := cldaws.GetVpcIdByName(client, goCtx, lb, vpcName)
	if err != nil {
		return err
	}

	if foundVpcId == "" {
		lb.Add(fmt.Sprintf("will not delete vpc %s, nothing to delete", vpcName))
		return nil
	}

	// Delete the route table pointing to natgw (if we don't, AWS will consider them as dependencies and will not delete vpc)
	foundRouteTableId, foundAttachedVpcId, err := cldaws.GetRouteTableByName(client, goCtx, lb, privateSubnetRouteTableToNatgwName)
	if err != nil {
		return err
	}
	if foundRouteTableId != "" {
		if foundAttachedVpcId != "" && foundAttachedVpcId != foundVpcId {
			return fmt.Errorf("cannot delete route table %s, it is attached to an unexpected vpc %s instead of %s", privateSubnetRouteTableToNatgwName, foundAttachedVpcId, foundVpcId)
		}
		if err := cldaws.DeleteRouteTable(client, goCtx, lb, foundRouteTableId); err != nil {
			return err
		}
	}

	return cldaws.DeleteVpc(client, goCtx, lb, foundVpcId)
}

func (p *AwsDeployProvider) CreateNetworking() (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)

	vpcId, err := ensureAwsVpc(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, p.GetCtx().Tags, lb, &p.GetCtx().Project.Network, p.GetCtx().Project.Timeouts.CreateNetwork)
	if err != nil {
		return lb.Complete(err)
	}

	privateSubnetId, err := ensureAwsPrivateSubnet(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, p.GetCtx().Tags, lb, vpcId, &p.GetCtx().Project.Network.PrivateSubnet)
	if err != nil {
		return lb.Complete(err)
	}

	publicSubnetId, err := ensureAwsPublicSubnet(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, p.GetCtx().Tags, lb,
		vpcId, &p.GetCtx().Project.Network.PublicSubnet)
	if err != nil {
		return lb.Complete(err)
	}

	err = ensureInternetGatewayAndRoutePublicSubnet(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, p.GetCtx().Tags, lb,
		p.GetCtx().Project.Network.Router.Name,
		vpcId, publicSubnetId, &p.GetCtx().Project.Network.PublicSubnet)
	if err != nil {
		return lb.Complete(err)
	}

	err = ensureNatGatewayAndRoutePrivateSubnet(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, p.GetCtx().Tags, lb,
		vpcId,
		publicSubnetId, &p.GetCtx().Project.Network.PublicSubnet,
		privateSubnetId, &p.GetCtx().Project.Network.PrivateSubnet,
		p.GetCtx().Project.Timeouts.CreateNatGateway)
	if err != nil {
		return lb.Complete(err)
	}

	return lb.Complete(nil)
}

func (p *AwsDeployProvider) DeleteNetworking() (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)

	err := checkAndDeleteNatGateway(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, p.GetCtx().Project.Network.PublicSubnet.NatGatewayName, p.GetCtx().Project.Timeouts.DeleteNatGateway)
	if err != nil {
		return lb.Complete(err)
	}

	err = detachAndDeleteInternetGateway(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, p.GetCtx().Project.Network.Router.Name)
	if err != nil {
		return lb.Complete(err)
	}

	err = deleteAwsSubnet(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, p.GetCtx().Project.Network.PublicSubnet.Name)
	if err != nil {
		return lb.Complete(err)
	}

	err = deleteAwsSubnet(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, p.GetCtx().Project.Network.PrivateSubnet.Name)
	if err != nil {
		return lb.Complete(err)
	}

	err = checkAndDeleteAwsVpcWithRouteTable(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, p.GetCtx().Project.Network.Name, p.GetCtx().Project.Network.PrivateSubnet.Name, p.GetCtx().Project.Network.PrivateSubnet.RouteTableToNatgwName)
	if err != nil {
		return lb.Complete(err)
	}

	return lb.Complete(nil)
}
