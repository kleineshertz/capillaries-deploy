package provider

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/capillariesio/capillaries-deploy/pkg/cld/cldaws"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
)

func ensureAwsVpc(p *AwsDeployProvider) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)

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
		newVpcId, err := cldaws.CreateVpc(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, p.GetCtx().Tags, lb,
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
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)

	subnetDef := p.GetCtx().PrjPair.Live.Network.PrivateSubnet

	// Check if the subnet is already there
	foundSubnetIdByName, err := cldaws.GetSubnetIdByName(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, subnetDef.Name)
	if err != nil {
		return lb.Complete(err)
	}

	if subnetDef.Id == "" {
		// If it was already created, but was not written to the prj file, save it for future use, but do not create
		if foundSubnetIdByName != "" {
			lb.Add(fmt.Sprintf("private subnet %s already there, updating project with new id %s", subnetDef.Name, foundSubnetIdByName))
			p.GetCtx().PrjPair.SetPrivateSubnetId(foundSubnetIdByName)
			return lb.Complete(nil)
		}
	} else {
		if foundSubnetIdByName == "" {
			// It was supposed to be there, but it's not present, complain
			return "", fmt.Errorf("requested private subnet id %s not present, consider removing this id from the project file", subnetDef.Id)
		} else if foundSubnetIdByName != subnetDef.Id {
			// It is already there, but has different id, complain
			return "", fmt.Errorf("requested private subnet id %s not matching existing id %s", subnetDef.Id, foundSubnetIdByName)
		}
	}

	// Existing id matches the found id, nothing to do
	if subnetDef.Id != "" {
		lb.Add(fmt.Sprintf("private subnet %s(%s) already there, no need to create", subnetDef.Name, foundSubnetIdByName))
		return lb.Complete(nil)
	}

	newId, err := cldaws.CreateSubnet(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, p.GetCtx().Tags, lb,
		p.GetCtx().PrjPair.Live.Network.Id,
		subnetDef.Name,
		subnetDef.Cidr,
		subnetDef.AvailabilityZone)
	if err != nil {
		return lb.Complete(fmt.Errorf("cannot create private subnet: %s", err.Error()))
	}
	p.GetCtx().PrjPair.SetPrivateSubnetId(newId)

	return lb.Complete(nil)
}

func ensureAwsPublicSubnet(p *AwsDeployProvider) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)

	subnetDef := p.GetCtx().PrjPair.Live.Network.PublicSubnet

	// Check if the subnet is already there
	foundSubnetIdByName, err := cldaws.GetSubnetIdByName(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, subnetDef.Name)
	if err != nil {
		return lb.Complete(err)
	}

	if subnetDef.Id == "" {
		// If it was already created, but was not written to the prj file, save it for future use, but do not create
		if foundSubnetIdByName != "" {
			lb.Add(fmt.Sprintf("public subnet %s already there, updating project with new id %s", subnetDef.Name, foundSubnetIdByName))
			p.GetCtx().PrjPair.SetPublicSubnetId(foundSubnetIdByName)
			return lb.Complete(nil)
		}
	} else {
		if foundSubnetIdByName == "" {
			// It was supposed to be there, but it's not present, complain
			return "", fmt.Errorf("requested public subnet id %s not present, consider removing this id from the project file", subnetDef.Id)
		} else if foundSubnetIdByName != subnetDef.Id {
			// It is already there, but has different id, complain
			return "", fmt.Errorf("requested public subnet id %s not matching existing id %s", subnetDef.Id, foundSubnetIdByName)
		}
	}

	// Existing id matches the found id, nothing to do
	if subnetDef.Id != "" {
		lb.Add(fmt.Sprintf("public subnet %s(%s) already there, no need to create", subnetDef.Name, foundSubnetIdByName))
		return lb.Complete(nil)
	}

	newId, err := cldaws.CreateSubnet(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, p.GetCtx().Tags, lb,
		p.GetCtx().PrjPair.Live.Network.Id,
		subnetDef.Name,
		subnetDef.Cidr,
		subnetDef.AvailabilityZone)
	if err != nil {
		return lb.Complete(fmt.Errorf("cannot create public subnet: %s", err.Error()))
	}
	p.GetCtx().PrjPair.SetPublicSubnetId(newId)

	return lb.Complete(nil)
}

func ensureNatGatewayAndRoutePrivateSubnet(p *AwsDeployProvider) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)

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
		return lb.Complete(fmt.Errorf("empty parameter not allowed: natGatewayName (%s)", natGatewayName))
	}

	foundNatGatewayIdByName, foundNatGatewayStateByName, err := cldaws.GetNatGatewayIdAndStateByName(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, natGatewayName)
	if err != nil {
		return lb.Complete(err)
	}

	if p.GetCtx().PrjPair.Live.Network.PublicSubnet.NatGatewayId == "" {
		// If it was already created, save it for future use, but do not create
		if foundNatGatewayIdByName != "" && foundNatGatewayStateByName == types.NatGatewayStateAvailable {
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

	if foundNatGatewayStateByName != types.NatGatewayStateAvailable {
		p.GetCtx().PrjPair.SetNatGatewayId("")
		lb.Add(fmt.Sprintf("nat gateway %s(%s) has bad state, re-creating", natGatewayName, foundNatGatewayIdByName))
	}

	// Create NAT gateway in the public subnet if needed

	if p.GetCtx().PrjPair.Live.Network.PublicSubnet.NatGatewayId == "" {
		newNatGatewayId, err := cldaws.CreateNatGateway(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, p.GetCtx().Tags, lb,
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
	routeTableId, err := cldaws.CreateRouteTableForVpc(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, p.GetCtx().Tags, lb, routeTableName, p.GetCtx().PrjPair.Live.Network.Id)
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
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)

	routerName := p.GetCtx().PrjPair.Live.Network.Router.Name
	if routerName == "" {
		return lb.Complete(fmt.Errorf("empty parameter not allowed: routerName (%s)", routerName))
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
		newRouterId, err := cldaws.CreateInternetGateway(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, p.GetCtx().Tags, lb, routerName)
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
	if err := cldaws.TagResource(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, routeTableId, routeTableName, nil); err != nil {
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
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)

	internetGatewayName := p.GetCtx().PrjPair.Live.Network.Router.Name
	if internetGatewayName == "" {
		return lb.Complete(fmt.Errorf("empty parameter not allowed: internetGatewayName (%s)", internetGatewayName))
	}

	foundId, err := cldaws.GetInternetGatewayIdByName(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, internetGatewayName)
	if err != nil {
		return lb.Complete(err)
	}

	if p.GetCtx().PrjPair.Live.Network.Router.Id == "" {
		if foundId != "" {
			// Update project, delete found
			p.GetCtx().PrjPair.SetRouterId(foundId)
		}
	} else {
		if foundId == "" {
			// Already deleted, update project
			p.GetCtx().PrjPair.SetRouterId("")
		} else if p.GetCtx().PrjPair.Live.Network.Router.Id != foundId {
			// It is already there, but has different id, complain
			return lb.Complete(fmt.Errorf("requested router id %s not matching existing router id %s", p.GetCtx().PrjPair.Live.Network.Router, foundId))
		}
	}

	// At this point, the project contains relevant resource id
	if p.GetCtx().PrjPair.Live.Network.Router.Id == "" {
		lb.Add(fmt.Sprintf("will not delete router %s, nothing to delete", internetGatewayName))
		return lb.Complete(nil)
	}

	// Is it attached to a vpc? If yes, detach it.

	attachedVpcId, attachmentState, err := cldaws.GetInternetGatewayVpcAttachmentById(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, foundId)
	if err != nil {
		return lb.Complete(err)
	}

	// NOTE: for unknown reason, I am getting "available" instead of "attached" here, so let's embrace it
	if attachedVpcId != "" &&
		(attachmentState == types.AttachmentStatusAttached || attachmentState == types.AttachmentStatusAttaching || string(attachmentState) == "available") {

		if attachedVpcId != p.GetCtx().PrjPair.Live.Network.Id {
			return lb.Complete(fmt.Errorf("will not detach internet gateway (router) %s from vpc (network) %s: this is not the original vpc it is supposed to be attached to - %s", foundId, attachedVpcId, p.GetCtx().PrjPair.Live.Network.Id))
		}

		// This may potentially throw:
		// Network vpc-... has some mapped public address(es). Please unmap those public address(es) before detaching the gateway.
		// if we do not wait for NAT gateway to be deleted completely
		err := cldaws.DetachInternetGatewayFromVpc(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, foundId, attachedVpcId)
		if err != nil {
			return lb.Complete(err)
		}
		lb.Add(fmt.Sprintf("detached internet gateway (router) %s from vpc %s", foundId, attachedVpcId))
	} else {
		lb.Add(fmt.Sprintf("internet gateway (router) %s was not attached, no need to detach", foundId))
	}

	// Delete

	err = cldaws.DeleteInternetGateway(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, foundId)
	if err != nil {
		return lb.Complete(err)
	}

	p.GetCtx().PrjPair.SetRouterId("")
	return lb.Complete(nil)
}

func checkAndDeleteNatGateway(p *AwsDeployProvider) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)

	// Are we about to delete a proper NAT gateway?

	natGatewayName := p.GetCtx().PrjPair.Live.Network.PublicSubnet.NatGatewayName
	if natGatewayName == "" {
		return lb.Complete(fmt.Errorf("empty parameter not allowed: natGatewayName (%s)", natGatewayName))
	}
	foundId, foundState, err := cldaws.GetNatGatewayIdAndStateByName(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, natGatewayName)
	if err != nil {
		return lb.Complete(err)
	}

	if p.GetCtx().PrjPair.Live.Network.PublicSubnet.NatGatewayId == "" {
		if foundId != "" &&
			(foundState == types.NatGatewayStateAvailable ||
				foundState == types.NatGatewayStateFailed ||
				foundState == types.NatGatewayStatePending) {
			// Update project, delete found
			p.GetCtx().PrjPair.SetNatGatewayId(foundId)
		}
	} else {
		if foundId == "" {
			// Already deleted, update project
			p.GetCtx().PrjPair.SetNatGatewayId("")
		} else if p.GetCtx().PrjPair.Live.Network.PublicSubnet.NatGatewayId != foundId {
			// It is already there, but has different id, complain
			return lb.Complete(fmt.Errorf("requested nat gateway id %s not matching existing nat gateway id %s", p.GetCtx().PrjPair.Live.Network.PublicSubnet.NatGatewayId, foundId))
		}
	}

	// At this point, the project contains relevant resource id
	if p.GetCtx().PrjPair.Live.Network.PublicSubnet.NatGatewayId == "" {
		lb.Add(fmt.Sprintf("will not delete nat gateway %s, nothing to delete", natGatewayName))
		return lb.Complete(nil)
	}

	err = cldaws.DeleteNatGateway(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, foundId, p.GetCtx().PrjPair.Live.Timeouts.DeleteNatGateway)
	if err != nil {
		return lb.Complete(err)
	}

	p.GetCtx().PrjPair.SetNatGatewayId("")

	return lb.Complete(nil)
}

func deleteAwsPrivateSubnet(p *AwsDeployProvider) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)

	subnetName := p.GetCtx().PrjPair.Live.Network.PrivateSubnet.Name
	if subnetName == "" {
		return lb.Complete(fmt.Errorf("empty parameter not allowed: subnetName (%s)", subnetName))
	}
	foundId, err := cldaws.GetSubnetIdByName(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, subnetName)
	if err != nil {
		return lb.Complete(err)
	}

	if p.GetCtx().PrjPair.Live.Network.PrivateSubnet.Id == "" {
		if foundId != "" {
			// Update project, delete found
			p.GetCtx().PrjPair.SetPrivateSubnetId(foundId)
		}
	} else {
		if foundId == "" {
			// Already deleted, update project
			p.GetCtx().PrjPair.SetPrivateSubnetId("")
		} else if p.GetCtx().PrjPair.Live.Network.PrivateSubnet.Id != foundId {
			// It is already there, but has different id, complain
			return lb.Complete(fmt.Errorf("requested private subnet id %s not matching existing private subnet id %s", p.GetCtx().PrjPair.Live.Network.PrivateSubnet.Id, foundId))
		}
	}
	// At this point, the project contains relevant resource id
	if p.GetCtx().PrjPair.Live.Network.PrivateSubnet.Id == "" {
		lb.Add(fmt.Sprintf("will not delete private subnet %s, nothing to delete", subnetName))
		return lb.Complete(nil)
	}

	err = cldaws.DeleteSubnet(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, foundId)
	if err != nil {
		return lb.Complete(err)
	}

	p.GetCtx().PrjPair.SetPrivateSubnetId("")

	return lb.Complete(nil)
}

func deleteAwsPublicSubnet(p *AwsDeployProvider) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)

	subnetName := p.GetCtx().PrjPair.Live.Network.PublicSubnet.Name
	if subnetName == "" {
		return lb.Complete(fmt.Errorf("empty parameter not allowed: subnetName (%s)", subnetName))
	}

	foundId, err := cldaws.GetSubnetIdByName(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, subnetName)
	if err != nil {
		return lb.Complete(err)
	}

	if p.GetCtx().PrjPair.Live.Network.PublicSubnet.Id == "" {
		if foundId != "" {
			// Update project, delete found
			p.GetCtx().PrjPair.SetPublicSubnetId(foundId)
		}
	} else {
		if foundId == "" {
			// Already deleted, update project
			p.GetCtx().PrjPair.SetPublicSubnetId("")
		} else if p.GetCtx().PrjPair.Live.Network.PublicSubnet.Id != foundId {
			// It is already there, but has different id, complain
			return lb.Complete(fmt.Errorf("requested public subnet id %s not matching existing public subnet id %s", p.GetCtx().PrjPair.Live.Network.PublicSubnet.Id, foundId))
		}
	}

	// At this point, the project contains relevant resource id
	if p.GetCtx().PrjPair.Live.Network.PublicSubnet.Id == "" {
		lb.Add(fmt.Sprintf("will not delete public subnet %s, nothing to delete", subnetName))
		return lb.Complete(nil)
	}

	err = cldaws.DeleteSubnet(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, foundId)
	if err != nil {
		return lb.Complete(err)
	}

	p.GetCtx().PrjPair.SetPublicSubnetId("")

	return lb.Complete(nil)
}

func checkAndDeleteAwsVpcWithRouteTable(p *AwsDeployProvider) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)

	vpcName := p.GetCtx().PrjPair.Live.Network.Name
	if vpcName == "" {
		return lb.Complete(fmt.Errorf("empty parameter not allowed: vpcName (%s)", vpcName))
	}
	foundId, err := cldaws.GetVpcIdByName(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, vpcName)
	if err != nil {
		return lb.Complete(err)
	}

	if p.GetCtx().PrjPair.Live.Network.Id == "" {
		if foundId != "" {
			// Update project, delete found
			p.GetCtx().PrjPair.SetNetworkId(foundId)
		}
	} else {
		if foundId == "" {
			// Already deleted, update project
			p.GetCtx().PrjPair.SetNetworkId("")
		} else if p.GetCtx().PrjPair.Live.Network.Id != foundId {
			// It is already there, but has different id, complain
			return lb.Complete(fmt.Errorf("requested vpc id %s not matching existing vpc id %s", p.GetCtx().PrjPair.Live.Network.Id, foundId))
		}
	}

	// At this point, the project contains relevant resource id
	if p.GetCtx().PrjPair.Live.Network.Id == "" {
		lb.Add(fmt.Sprintf("will not delete vpc %s, nothing to delete", vpcName))
		return lb.Complete(nil)
	}

	if p.GetCtx().PrjPair.Live.Network.PrivateSubnet.RouteTableToNat != "" {
		// Delete the route table pointing to natgw (if we don't, AWS will consider them as dependencies and will not delete vpc)
		err := cldaws.DeleteRouteTable(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, p.GetCtx().PrjPair.Live.Network.PrivateSubnet.RouteTableToNat)
		if err != nil {
			return lb.Complete(err)
		}
	}

	if err = cldaws.DeleteVpc(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, foundId); err != nil {
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
