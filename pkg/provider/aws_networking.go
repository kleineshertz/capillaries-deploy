package provider

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/capillariesio/capillaries-deploy/pkg/cld/cldaws"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
)

func ensureAwsVpc(p *AwsDeployProvider) (l.LogMsg, error) {
	lb := l.NewLogBuilder(cldaws.CurAwsFuncName(), p.GetCtx().IsVerbose)

	vpcName := p.GetCtx().PrjPair.Live.Network.Name
	foundVpcIdByName, err := cldaws.GetVpcIdByName(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, vpcName)
	if err != nil {
		return lb.Complete(err)
	}

	if p.GetCtx().PrjPair.Live.Network.Id == "" {
		// If it was already created, save it for future use, but do not create
		if foundVpcIdByName != "" {
			lb.Add(fmt.Sprintf("vpc (network) %s(%s) already there, updating project", vpcName, foundVpcIdByName))
			p.GetCtx().PrjPair.SetNetworkId(foundVpcIdByName)
		}
	} else {
		if foundVpcIdByName == "" {
			// It was supposed to be there, but it's not present, complain
			return lb.Complete(fmt.Errorf("requested vpc (network)  %s(%s) not present, consider removing this id from the project file", vpcName, p.GetCtx().PrjPair.Live.Network.Id))
		} else if p.GetCtx().PrjPair.Live.Network.Id != foundVpcIdByName {
			// It is already there, but has different id, complain
			return lb.Complete(fmt.Errorf("requested vpc (network) %s(%s) not matching existing vpc id %s", vpcName, p.GetCtx().PrjPair.Live.Network.Id, foundVpcIdByName))
		}
	}

	if p.GetCtx().PrjPair.Live.Network.Id == "" {
		newVpcId, err := cldaws.CreateVpc(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb,
			vpcName,
			p.GetCtx().PrjPair.Live.Network.Cidr,
			p.GetCtx().PrjPair.Live.Timeouts.CreateNetwork)
		if err != nil {
			return lb.Complete(err)
		}
		p.GetCtx().PrjPair.SetNetworkId(newVpcId)
	}

	return lb.Complete(nil)
}

func ensureAwsPrivateSubnet(p *AwsDeployProvider) (l.LogMsg, error) {
	lb := l.NewLogBuilder(cldaws.CurAwsFuncName(), p.GetCtx().IsVerbose)

	newId, err := cldaws.EnsureSubnet(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb,
		p.GetCtx().PrjPair.Live.Network.Id,
		p.GetCtx().PrjPair.Live.Network.PrivateSubnet.Name,
		p.GetCtx().PrjPair.Live.Network.PrivateSubnet.Cidr,
		p.GetCtx().PrjPair.Live.Network.PrivateSubnet.Id,
		p.GetCtx().PrjPair.Live.Network.PrivateSubnet.AvailabilityZone)
	if err != nil {
		return lb.Complete(fmt.Errorf("cannot create private subnet: %s", err.Error()))
	}

	p.GetCtx().PrjPair.SetPrivateSubnetId(newId)

	return lb.Complete(nil)
}

func ensureAwsPublicSubnet(p *AwsDeployProvider) (l.LogMsg, error) {
	lb := l.NewLogBuilder(cldaws.CurAwsFuncName(), p.GetCtx().IsVerbose)

	newId, err := cldaws.EnsureSubnet(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb,
		p.GetCtx().PrjPair.Live.Network.Id,
		p.GetCtx().PrjPair.Live.Network.PublicSubnet.Name,
		p.GetCtx().PrjPair.Live.Network.PublicSubnet.Cidr,
		p.GetCtx().PrjPair.Live.Network.PublicSubnet.Id,
		p.GetCtx().PrjPair.Live.Network.PublicSubnet.AvailabilityZone)
	if err != nil {
		return lb.Complete(fmt.Errorf("cannot create public subnet: %s", err.Error()))
	}

	p.GetCtx().PrjPair.SetPublicSubnetId(newId)

	return lb.Complete(nil)
}

func ensureNatGatewayAndRoutePrivateSubnet(p *AwsDeployProvider) (l.LogMsg, error) {
	lb := l.NewLogBuilder(cldaws.CurAwsFuncName(), p.GetCtx().IsVerbose)

	// Get NAT gateway public IP allocation id

	natGatewayPublicIp := p.GetCtx().PrjPair.Live.Network.PublicSubnet.NatGatewayPublicIp

	if natGatewayPublicIp == "" {
		return lb.Complete(fmt.Errorf("natgw public IP cannot be empty, have you reserved a floating ip for it?"))
	}

	natGatewayPublicIpAllocationId, err := cldaws.GetPublicIpAllocation(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, natGatewayPublicIp)
	if err != nil {
		return lb.Complete(err)
	}

	// Get NAT gateway by name

	natGatewayName := p.GetCtx().PrjPair.Live.Network.PublicSubnet.NatGatewayName
	if natGatewayName == "" {
		return lb.Complete(fmt.Errorf("natgw name cannot be empty"))
	}

	foundNatGatewayIdByName, err := cldaws.GetNatGatewayIdByName(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, natGatewayName)
	if err != nil {
		return lb.Complete(err)
	}

	if p.GetCtx().PrjPair.Live.Network.PublicSubnet.NatGatewayId == "" {
		// If it was already created, save it for future use, but do not create
		if foundNatGatewayIdByName != "" {
			lb.Add(fmt.Sprintf("nat gateway %s(%s) already there, updating project", natGatewayName, foundNatGatewayIdByName))
			p.GetCtx().PrjPair.SetNatGatewayId(foundNatGatewayIdByName)
		}
	} else {
		if foundNatGatewayIdByName == "" {
			// It was supposed to be there, but it's not present, complain
			return lb.Complete(fmt.Errorf("requested nat gateway %s(%s) not present, consider removing this id from the project file", natGatewayName, p.GetCtx().PrjPair.Live.Network.PublicSubnet.NatGatewayId))
		} else if p.GetCtx().PrjPair.Live.Network.PublicSubnet.NatGatewayId != foundNatGatewayIdByName {
			// It is already there, but has different id, complain
			return lb.Complete(fmt.Errorf("requested nat gateway %s(%s) not matching existing nat gateway id %s", natGatewayName, p.GetCtx().PrjPair.Live.Network.PublicSubnet.NatGatewayId, foundNatGatewayIdByName))
		}
	}

	// Create NAT gateway in the public subnet if needed

	if p.GetCtx().PrjPair.Live.Network.PublicSubnet.NatGatewayId == "" {
		newNatGatewayId, err := cldaws.CreateNatGateway(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb,
			natGatewayName,
			p.GetCtx().PrjPair.Live.Network.PublicSubnet.Id,
			natGatewayPublicIpAllocationId,
			p.GetCtx().PrjPair.Live.Timeouts.CreateNatGateway)
		if err != nil {
			return lb.Complete(err)
		}
		p.GetCtx().PrjPair.SetNatGatewayId(newNatGatewayId)
	}

	// Create new route table id for this vpc

	routeTableName := p.GetCtx().PrjPair.Live.Network.PrivateSubnet.Name + "_rt_to_natgw"
	routeTableId, err := cldaws.CreateRouteTableForVpc(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, routeTableName, p.GetCtx().PrjPair.Live.Network.Id)
	if err != nil {
		return lb.Complete(err)
	}
	p.GetCtx().PrjPair.SetRouteTableToNat(routeTableId)

	// Associate this route table with the private subnet

	rtAssocId, err := cldaws.AssociateRouteTableWithSubnet(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, routeTableId, p.GetCtx().PrjPair.Live.Network.PrivateSubnet.Id)
	if err != nil {
		return lb.Complete(err)
	}

	lb.Add(fmt.Sprintf("associated route table %s with private subnet %s: %s", routeTableId, p.GetCtx().PrjPair.Live.Network.PrivateSubnet.Id, rtAssocId))

	// Add a record to a route table: tell all outbound 0.0.0.0/0 traffic to go through this nat gateway:

	if err := cldaws.CreateNatGatewayRoute(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb,
		routeTableId, "0.0.0.0/0",
		p.GetCtx().PrjPair.Live.Network.PublicSubnet.NatGatewayId); err != nil {
		return lb.Complete(err)
	}

	lb.Add(fmt.Sprintf("route table %s in private subnet %s points to nat gateway %s", routeTableId, p.GetCtx().PrjPair.Live.Network.PrivateSubnet.Id, p.GetCtx().PrjPair.Live.Network.PublicSubnet.NatGatewayId))

	return lb.Complete(nil)
}

func ensureInternetGatewayAndRoutePublicSubnet(p *AwsDeployProvider) (l.LogMsg, error) {
	lb := l.NewLogBuilder(cldaws.CurAwsFuncName(), p.GetCtx().IsVerbose)

	routerName := p.GetCtx().PrjPair.Live.Network.Router.Name
	if routerName == "" {
		return lb.Complete(fmt.Errorf("internet gateway (router) name cannot be empty"))
	}

	// Get internet gateway (router) by name

	foundRouterIdByName, err := cldaws.GetInternetGatewayIdByName(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, routerName)
	if err != nil {
		return lb.Complete(err)
	}

	if p.GetCtx().PrjPair.Live.Network.Router.Id == "" {
		// If it was already created, save it for future use, but do not create
		if foundRouterIdByName != "" {
			lb.Add(fmt.Sprintf("internet gateway (router) %s(%s) already there, updating project", p.GetCtx().PrjPair.Live.Network.Router.Name, foundRouterIdByName))
			p.GetCtx().PrjPair.SetRouterId(foundRouterIdByName)
		}
	} else {
		if foundRouterIdByName == "" {
			// It was supposed to be there, but it's not present, complain
			return lb.Complete(fmt.Errorf("requested internet gateway (router) id %s not present, consider removing this id from the project file", p.GetCtx().PrjPair.Live.Network.Router.Id))
		} else if p.GetCtx().PrjPair.Live.Network.Router.Id != foundRouterIdByName {
			// It is already there, but has different id, complain
			return lb.Complete(fmt.Errorf("requested internet gateway (router) id %s not matching existing internet gateway (router) id %s", p.GetCtx().PrjPair.Live.Network.Router.Id, foundRouterIdByName))
		}
	}

	// Create internet gateway (router) if needed

	if p.GetCtx().PrjPair.Live.Network.Router.Id == "" {
		newRouterId, err := cldaws.CreateInternetGateway(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, routerName)
		if err != nil {
			return lb.Complete(err)
		}
		p.GetCtx().PrjPair.SetRouterId(newRouterId)
	}

	// Is this internet gateway (router) attached to a vpc?

	attachedVpcId, _, err := cldaws.GetInternetGatewayVpcAttachmentById(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, p.GetCtx().PrjPair.Live.Network.Router.Id)
	if err != nil {
		return lb.Complete(err)
	}

	// Attach if needed

	if attachedVpcId == "" {
		if err := cldaws.AttachInternetGatewayToVpc(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb,
			p.GetCtx().PrjPair.Live.Network.Router.Id,
			p.GetCtx().PrjPair.Live.Network.Id); err != nil {
			return lb.Complete(err)
		}
	} else if attachedVpcId != p.GetCtx().PrjPair.Live.Network.Id {
		return lb.Complete(fmt.Errorf("internet gateway (router) %s seems to be attached to a wrong vpc %s already", p.GetCtx().PrjPair.Live.Network.Router.Name, attachedVpcId))
	} else {
		lb.Add(fmt.Sprintf("internet gateway (router) %s seems to be attached to vpc already", p.GetCtx().PrjPair.Live.Network.Router.Name))
	}

	// Obtain route table id for this vpc (it was automatically created for us and marked as 'main')

	routeTableId, err := cldaws.GetVpcDefaultRouteTable(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, p.GetCtx().PrjPair.Live.Network.Id)
	if err != nil {
		return lb.Complete(err)
	}

	// (optional) tag this route table for operator's convenience

	routeTableName := p.GetCtx().PrjPair.Live.Network.PublicSubnet.Name + "_rt_to_igw"
	if err := cldaws.TagResourceName(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, routeTableId, routeTableName); err != nil {
		return lb.Complete(err)
	}

	// Associate this default (main) route table with the public subnet

	assocId, err := cldaws.AssociateRouteTableWithSubnet(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, routeTableId, p.GetCtx().PrjPair.Live.Network.PublicSubnet.Id)
	if err != nil {
		return lb.Complete(err)
	}
	lb.Add(fmt.Sprintf("associated route table %s with public subnet %s: %s", routeTableId, p.GetCtx().PrjPair.Live.Network.PublicSubnet.Id, assocId))

	// Add a record to a route table: tell all outbound 0.0.0.0/0 traffic to go through this internet gateway:

	if err := cldaws.CreateInternetGatewayRoute(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb,
		routeTableId, "0.0.0.0/0",
		p.GetCtx().PrjPair.Live.Network.Router.Id); err != nil {
		return lb.Complete(err)
	}
	lb.Add(fmt.Sprintf("route table %s in public subnet %s points to internet gateway (router) %s", routeTableId, p.GetCtx().PrjPair.Live.Network.PublicSubnet.Id, p.GetCtx().PrjPair.Live.Network.Router.Id))

	return lb.Complete(nil)
}

func detachAndDeleteInternetGateway(p *AwsDeployProvider) (l.LogMsg, error) {
	lb := l.NewLogBuilder(cldaws.CurAwsFuncName(), p.GetCtx().IsVerbose)

	internetGatewayName := p.GetCtx().PrjPair.Live.Network.Router.Name
	if internetGatewayName == "" {
		lb.Add("internet gateway (router) name empty, nothing to delete")
		return lb.Complete(nil)
	}

	// Check if it's there

	foundRouterIdByName, err := cldaws.GetInternetGatewayIdByName(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, internetGatewayName)
	if err != nil {
		return lb.Complete(err)
	}

	if foundRouterIdByName == "" {
		lb.Add(fmt.Sprintf("network gateway (router) %s not found, nothing to delete", internetGatewayName))
		p.GetCtx().PrjPair.SetRouterId("")
		return lb.Complete(err)
	}

	if p.GetCtx().PrjPair.Live.Network.Router.Id != "" &&
		foundRouterIdByName != p.GetCtx().PrjPair.Live.Network.Router.Id {
		return lb.Complete(fmt.Errorf("network gateway (router) %s not found, but another network gateway with this name found (id %s), not sure what to delete", p.GetCtx().PrjPair.Live.Network.Router.Name, foundRouterIdByName))
	}

	// Is it attached to a vpc? If yes, detach it.

	attachedVpcId, attachmentState, err := cldaws.GetInternetGatewayVpcAttachmentById(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, foundRouterIdByName)
	if err != nil {
		return lb.Complete(err)
	}

	if attachedVpcId != "" &&
		(attachmentState == types.AttachmentStatusAttached || attachmentState == types.AttachmentStatusAttaching) {

		if attachedVpcId != p.GetCtx().PrjPair.Live.Network.Id {
			return lb.Complete(fmt.Errorf("will not detach internet gateway (router) %s from vpc (network) %s: this is not the original vpc it is supposed to be attached to - %s", foundRouterIdByName, p.GetCtx().PrjPair.Live.Network.Id))
		}

		// This may potentially throw:
		// Network vpc-... has some mapped public address(es). Please unmap those public address(es) before detaching the gateway.
		// if we do not wait for NAT gateway to be deleted completely
		err := cldaws.DetachInternetGatewayFromVpc(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, foundRouterIdByName, attachedVpcId)
		if err != nil {
			return lb.Complete(err)
		}
		lb.Add(fmt.Sprintf("detached internet gateway (router) %s from vpc %s", foundRouterIdByName, attachedVpcId))
	} else {
		lb.Add(fmt.Sprintf("internet gateway (router) %s was not attached, no need to detach", foundRouterIdByName))
	}

	// Delete

	err = cldaws.DeleteInternetGateway(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, foundRouterIdByName)
	if err != nil {
		return lb.Complete(err)
	}

	p.GetCtx().PrjPair.SetRouterId("")
	return lb.Complete(nil)
}

func checkAndDeleteNatGateway(p *AwsDeployProvider) (l.LogMsg, error) {
	lb := l.NewLogBuilder(cldaws.CurAwsFuncName(), p.GetCtx().IsVerbose)

	// Are we about to delete a proper NAT gateway?

	natGatewayName := p.GetCtx().PrjPair.Live.Network.PublicSubnet.NatGatewayName
	if natGatewayName == "" {
		return lb.Complete(fmt.Errorf(fmt.Sprintf("nat gateway name cannot be empty")))
	}
	natGatewayIdByName, err := cldaws.GetNatGatewayIdByName(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, natGatewayName)
	if err != nil {
		return lb.Complete(err)
	}

	if natGatewayIdByName == "" {
		if p.GetCtx().PrjPair.Live.Network.PublicSubnet.NatGatewayId != "" {
			lb.Add(fmt.Sprintf("nat gateway %s does not exist, updating the project to clean stored id for it", natGatewayName))
			p.GetCtx().PrjPair.SetNatGatewayId("")
		} else {
			lb.Add(fmt.Sprintf("nat gateway %s does not exist, and the stored id for it is empty, no need to delete", natGatewayName))
		}
		return lb.Complete(nil)
	} else {
		if p.GetCtx().PrjPair.Live.Network.PublicSubnet.NatGatewayId != natGatewayIdByName {
			return lb.Complete(fmt.Errorf("nat gateway %s has id %s, but the projet has id %s, not sure what to delete", natGatewayName, natGatewayIdByName, p.GetCtx().PrjPair.Live.Network.PublicSubnet.NatGatewayId))
		}
	}

	// Delete

	err = cldaws.DeleteNatGateway(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, natGatewayIdByName, p.GetCtx().PrjPair.Live.Timeouts.DeleteNatGateway)
	if err != nil {
		return lb.Complete(err)
	}

	return lb.Complete(nil)
}

func deleteAwsPrivateSubnet(p *AwsDeployProvider) (l.LogMsg, error) {
	lb := l.NewLogBuilder(cldaws.CurAwsFuncName(), p.GetCtx().IsVerbose)

	subnetName := p.GetCtx().PrjPair.Live.Network.PrivateSubnet.Name
	foundSubnetIdByName, err := cldaws.GetSubnetIdByName(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, subnetName)
	if err != nil {
		return lb.Complete(err)
	}

	if foundSubnetIdByName == "" {
		lb.Add(fmt.Sprintf("private subnet %s not found, nothing to delete", subnetName))
		p.GetCtx().PrjPair.SetPrivateSubnetId("")
		return lb.Complete(nil)
	}

	if p.GetCtx().PrjPair.Live.Network.PrivateSubnet.Id != "" && foundSubnetIdByName != p.GetCtx().PrjPair.Live.Network.PrivateSubnet.Id {
		return lb.Complete(fmt.Errorf("private subnet with name %s and id %d found, but it does not match the id in the project - %s, not sure what to delete", subnetName, foundSubnetIdByName, p.GetCtx().PrjPair.Live.Network.PrivateSubnet.Id))
	}

	err = cldaws.DeleteSubnet(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, foundSubnetIdByName)
	if err != nil {
		return lb.Complete(err)
	}

	p.GetCtx().PrjPair.SetPrivateSubnetId("")

	return lb.Complete(nil)
}

func deleteAwsPublicSubnet(p *AwsDeployProvider) (l.LogMsg, error) {
	lb := l.NewLogBuilder(cldaws.CurAwsFuncName(), p.GetCtx().IsVerbose)

	subnetName := p.GetCtx().PrjPair.Live.Network.PrivateSubnet.Name
	foundSubnetIdByName, err := cldaws.GetSubnetIdByName(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, subnetName)
	if err != nil {
		return lb.Complete(err)
	}

	if foundSubnetIdByName == "" {
		lb.Add(fmt.Sprintf("public subnet %s not found, nothing to delete", subnetName))
		p.GetCtx().PrjPair.SetPublicSubnetId("")
		return lb.Complete(nil)
	}

	if p.GetCtx().PrjPair.Live.Network.PublicSubnet.Id != "" && foundSubnetIdByName != p.GetCtx().PrjPair.Live.Network.PublicSubnet.Id {
		return lb.Complete(fmt.Errorf("public subnet with name %s and id %d found, but it does not match the id in the project - %s, not sure what to delete", subnetName, foundSubnetIdByName, p.GetCtx().PrjPair.Live.Network.PublicSubnet.Id))
	}

	err = cldaws.DeleteSubnet(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, foundSubnetIdByName)
	if err != nil {
		return lb.Complete(err)
	}

	p.GetCtx().PrjPair.SetPublicSubnetId("")

	return lb.Complete(nil)
}

func checkAndDeleteAwsVpcWithRouteTable(p *AwsDeployProvider) (l.LogMsg, error) {
	lb := l.NewLogBuilder(cldaws.CurAwsFuncName(), p.GetCtx().IsVerbose)

	vpcName := p.GetCtx().PrjPair.Live.Network.Name
	foundVpcIdByName, err := cldaws.GetVpcIdByName(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, vpcName)
	if err != nil {
		return lb.Complete(err)
	}

	if foundVpcIdByName == "" {
		lb.Add(fmt.Sprintf("vpc (network) %s not found, nothing to delete", vpcName))
		p.GetCtx().PrjPair.SetNetworkId("")
		return lb.Complete(nil)
	}

	if p.GetCtx().PrjPair.Live.Network.Id != "" && foundVpcIdByName != p.GetCtx().PrjPair.Live.Network.Id {
		return lb.Complete(fmt.Errorf("vpc (network) %s with id %s found, but the project has id %s, not sure what to delete", vpcName, foundVpcIdByName, p.GetCtx().PrjPair.Live.Network.Id))
	}

	if p.GetCtx().PrjPair.Live.Network.PrivateSubnet.RouteTableToNat != "" {
		// Delete the route table pointing to natgw (if we don't, AWS will consider them as dependencies and will not delete vpc)
		err := cldaws.DeleteRouteTable(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, p.GetCtx().PrjPair.Live.Network.PrivateSubnet.RouteTableToNat)
		if err != nil {
			return lb.Complete(err)
		}
	}

	if err = cldaws.DeleteVpc(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, foundVpcIdByName); err != nil {
		return lb.Complete(err)
	}

	p.GetCtx().PrjPair.SetNetworkId("")

	return lb.Complete(nil)
}

func (p *AwsDeployProvider) CreateNetworking() (l.LogMsg, error) {
	sb := strings.Builder{}

	logMsg, err := ensureAwsVpc(p)
	l.AddLogMsg(&sb, logMsg)
	if err != nil {
		return l.LogMsg(sb.String()), err
	}

	logMsg, err = ensureAwsPrivateSubnet(p)
	l.AddLogMsg(&sb, logMsg)
	if err != nil {
		return l.LogMsg(sb.String()), err
	}

	logMsg, err = ensureAwsPublicSubnet(p)
	l.AddLogMsg(&sb, logMsg)
	if err != nil {
		return l.LogMsg(sb.String()), err
	}

	logMsg, err = ensureInternetGatewayAndRoutePublicSubnet(p)
	l.AddLogMsg(&sb, logMsg)
	if err != nil {
		return l.LogMsg(sb.String()), err
	}

	logMsg, err = ensureNatGatewayAndRoutePrivateSubnet(p)
	l.AddLogMsg(&sb, logMsg)
	if err != nil {
		return l.LogMsg(sb.String()), err
	}

	return l.LogMsg(sb.String()), nil
}

func (p *AwsDeployProvider) DeleteNetworking() (l.LogMsg, error) {
	sb := strings.Builder{}

	logMsg, err := checkAndDeleteNatGateway(p)
	l.AddLogMsg(&sb, logMsg)
	if err != nil {
		return l.LogMsg(sb.String()), err
	}

	logMsg, err = detachAndDeleteInternetGateway(p)
	l.AddLogMsg(&sb, logMsg)
	if err != nil {
		return l.LogMsg(sb.String()), err
	}

	logMsg, err = deleteAwsPublicSubnet(p)
	l.AddLogMsg(&sb, logMsg)
	if err != nil {
		return l.LogMsg(sb.String()), err
	}

	logMsg, err = deleteAwsPrivateSubnet(p)
	l.AddLogMsg(&sb, logMsg)
	if err != nil {
		return l.LogMsg(sb.String()), err
	}

	logMsg, err = checkAndDeleteAwsVpcWithRouteTable(p)
	l.AddLogMsg(&sb, logMsg)
	if err != nil {
		return l.LogMsg(sb.String()), err
	}

	return l.LogMsg(sb.String()), nil
}
