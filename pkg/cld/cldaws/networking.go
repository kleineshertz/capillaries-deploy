package cldaws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
)

func GetSubnetIdByName(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, subnetName string) (string, error) {
	out, err := ec2Client.DescribeSubnets(goCtx, &ec2.DescribeSubnetsInput{Filters: []types.Filter{{
		Name: aws.String("tag:Name"), Values: []string{subnetName}}}})
	lb.AddObject(fmt.Sprintf("DescribeSubnets(tag:Name=%s)", subnetName), out)
	if err != nil {
		return "", fmt.Errorf("cannot describe subnet %s: %s", subnetName, err.Error())
	}
	if len(out.Subnets) == 0 {
		return "", nil
	}
	return *out.Subnets[0].SubnetId, nil
}

func CreateSubnet(ec2Client *ec2.Client, goCtx context.Context, tags map[string]string, lb *l.LogBuilder, vpcId string, subnetName string, cidr string, availabilityZone string) (string, error) {
	if vpcId == "" || subnetName == "" || cidr == "" || availabilityZone == "" {
		return "", fmt.Errorf("empty parameter not allowed: vpcId (%s), subnetName (%s), cidr (%s), availabilityZone (%s)", vpcId, subnetName, cidr, availabilityZone)
	}
	outCreate, err := ec2Client.CreateSubnet(goCtx, &ec2.CreateSubnetInput{
		VpcId:            aws.String(vpcId),
		CidrBlock:        aws.String(cidr),
		AvailabilityZone: aws.String(availabilityZone),
		TagSpecifications: []types.TagSpecification{{
			ResourceType: types.ResourceTypeSubnet,
			Tags:         mapToTags(subnetName, tags)}}})
	lb.AddObject(fmt.Sprintf("CreateSubnet(vpcId=%s,subnetName=%s,cidr=%s,availabilityZone=%s)", vpcId, subnetName, cidr, availabilityZone), outCreate)
	if err != nil {
		return "", fmt.Errorf("cannot create subnet %s: %s", subnetName, err.Error())
	}

	// TODO: dhcp options and allocation pools?

	return *outCreate.Subnet.SubnetId, nil
}

func DeleteSubnet(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, subnetId string) error {
	out, err := ec2Client.DeleteSubnet(goCtx, &ec2.DeleteSubnetInput{SubnetId: aws.String(subnetId)})
	lb.AddObject(fmt.Sprintf("DeleteSubnet(subnetId=%s)", subnetId), out)
	if err != nil {
		return fmt.Errorf("cannot delete subnet %s: %s", subnetId, err.Error())
	}
	return nil
}

func GetVpcIdByName(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, vpcName string) (string, error) {
	if vpcName == "" {
		return "", fmt.Errorf("empty parameter not allowed: vpcName (%s)", vpcName)
	}
	out, err := ec2Client.DescribeVpcs(goCtx, &ec2.DescribeVpcsInput{Filters: []types.Filter{{
		Name: aws.String("tag:Name"), Values: []string{vpcName}}}})
	lb.AddObject(fmt.Sprintf("DescribeVpcs(tag:Name=%s)", vpcName), out)
	if err != nil {
		return "", fmt.Errorf("cannot describe vpc (network) %s: %s", vpcName, err.Error())
	}

	if len(out.Vpcs) > 0 {
		return *out.Vpcs[0].VpcId, nil
	}

	return "", nil
}

func CreateVpc(ec2Client *ec2.Client, goCtx context.Context, tags map[string]string, lb *l.LogBuilder, vpcName string, cidrBlock string, timeoutSeconds int) (string, error) {
	if vpcName == "" || cidrBlock == "" {
		return "", fmt.Errorf("empty parameter not allowed: vpcName (%s), cidrBlock (%s)", vpcName, cidrBlock)
	}
	outCreate, err := ec2Client.CreateVpc(goCtx, &ec2.CreateVpcInput{
		CidrBlock: aws.String(cidrBlock),
		TagSpecifications: []types.TagSpecification{{
			ResourceType: types.ResourceTypeVpc,
			Tags:         mapToTags(vpcName, tags)}}})

	lb.AddObject(fmt.Sprintf("CreateVpc(vpcName=%s,cidr=%s)", vpcName, cidrBlock), outCreate)
	if err != nil {
		return "", fmt.Errorf("cannot create vpc (network) %s: %s", vpcName, err.Error())
	}
	if outCreate.Vpc == nil {
		return "", fmt.Errorf("cannot create vpc (network) %s: returned empty vpc", vpcName)
	}

	newVpcId := *outCreate.Vpc.VpcId

	startWaitTs := time.Now()
	for {
		out, err := ec2Client.DescribeVpcs(goCtx, &ec2.DescribeVpcsInput{Filters: []types.Filter{{
			Name: aws.String("vpc-id"), Values: []string{newVpcId}}}})
		lb.AddObject(fmt.Sprintf("DescribeVpcs(vpc-id=%s)", newVpcId), out)
		if err != nil {
			return "", fmt.Errorf("cannot query for newly created vpc (network) by id %s: %s", newVpcId, err.Error())
		}
		if len(out.Vpcs) == 0 {
			return "", fmt.Errorf("cannot query for newly created vpc (network) by id %s: returned zero vpcs", newVpcId)
		}

		status := out.Vpcs[0].State

		if status == types.VpcStateAvailable {
			break
		}
		if status != types.VpcStatePending {
			return "", fmt.Errorf("vpc (network) %s was created, but has unexpected status %s", newVpcId, status)
		}
		if time.Since(startWaitTs).Seconds() > float64(timeoutSeconds) {
			return "", fmt.Errorf("giving up after waiting for vpc (network) %s to be created after %ds", newVpcId, timeoutSeconds)
		}
		time.Sleep(1 * time.Second)
	}

	return newVpcId, nil
}

func DeleteVpc(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, vpcId string) error {
	out, err := ec2Client.DeleteVpc(goCtx, &ec2.DeleteVpcInput{VpcId: aws.String(vpcId)})
	lb.AddObject(fmt.Sprintf("DeleteVpc(vpcId=%s)", vpcId), out)
	if err != nil {
		return fmt.Errorf("cannot delete vpc (network) %s: %s", vpcId, err.Error())
	}
	return nil
}

func CreateInternetGatewayRoute(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, routeTableId string, destinationCidrBlock string, internetGatewayId string) error {
	if routeTableId == "" || destinationCidrBlock == "" || internetGatewayId == "" {
		return fmt.Errorf("empty parameter not allowed: routeTableId (%s), destinationCidrBlock (%s), internetGatewayId (%s)", routeTableId, destinationCidrBlock, internetGatewayId)
	}
	out, err := ec2Client.CreateRoute(goCtx, &ec2.CreateRouteInput{
		RouteTableId:         aws.String(routeTableId),
		DestinationCidrBlock: aws.String(destinationCidrBlock),
		GatewayId:            aws.String(internetGatewayId)})
	lb.AddObject(fmt.Sprintf("CreateRoute(routeTableId=%s,destinationCidrBlock=%s,internetGatewayId=%s)", routeTableId, destinationCidrBlock, internetGatewayId), out)
	if err != nil {
		return fmt.Errorf("cannot create route for internet gateway (router) %s, route table %s: %s", internetGatewayId, routeTableId, err.Error())
	}

	if !*out.Return {
		return fmt.Errorf("cannot create route for internet gateway (router) %s, route table %s: result false", internetGatewayId, routeTableId)
	}

	return nil
}

func CreateNatGatewayRoute(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, routeTableId string, destinationCidrBlock string, natGatewayId string) error {
	if routeTableId == "" || destinationCidrBlock == "" || natGatewayId == "" {
		return fmt.Errorf("empty parameter not allowed: routeTableId (%s), destinationCidrBlock (%s), natGatewayId (%s)", routeTableId, destinationCidrBlock, natGatewayId)
	}
	out, err := ec2Client.CreateRoute(goCtx, &ec2.CreateRouteInput{
		RouteTableId:         aws.String(routeTableId),
		DestinationCidrBlock: aws.String(destinationCidrBlock),
		NatGatewayId:         aws.String(natGatewayId)})
	lb.AddObject(fmt.Sprintf("CreateRoute(routeTableId=%s,destinationCidrBlock=%s,natGatewayId=%s)", routeTableId, destinationCidrBlock, natGatewayId), out)
	if err != nil {
		return fmt.Errorf("cannot create route for nat gateway %s, route table %s: %s", natGatewayId, routeTableId, err.Error())
	}

	if !*out.Return {
		return fmt.Errorf("cannot create route for nat gateway %s, route table %s: result false", natGatewayId, routeTableId)
	}

	return nil
}

func GetNatGatewayIdAndStateByName(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, natGatewayName string) (string, types.NatGatewayState, error) {
	out, err := ec2Client.DescribeNatGateways(goCtx, &ec2.DescribeNatGatewaysInput{Filter: []types.Filter{{Name: aws.String("tag:Name"), Values: []string{natGatewayName}}}})
	lb.AddObject(fmt.Sprintf("DescribeNatGateways(tag:Name=%s)", natGatewayName), out)
	if err != nil {
		return "", types.NatGatewayStateDeleted, fmt.Errorf("cannot describe natgw %s: %s", natGatewayName, err.Error())
	}
	if len(out.NatGateways) == 0 {
		return "", types.NatGatewayStateDeleted, nil
	}

	var natGatewayId string
	stateName := types.NatGatewayStateFailed
	for resIdx := 0; resIdx < len(out.NatGateways); resIdx++ {
		if out.NatGateways[resIdx].State == types.NatGatewayStateAvailable {
			return *out.NatGateways[resIdx].NatGatewayId, out.NatGateways[resIdx].State, nil
		}

		if out.NatGateways[resIdx].State == types.NatGatewayStatePending {
			natGatewayId = *out.NatGateways[resIdx].NatGatewayId
			stateName = out.NatGateways[resIdx].State
		} else if stateName != types.NatGatewayStatePending {
			natGatewayId = *out.NatGateways[resIdx].NatGatewayId
			stateName = out.NatGateways[resIdx].State
		}
	}

	return natGatewayId, stateName, nil
}

func CreateNatGateway(ec2Client *ec2.Client, goCtx context.Context, tags map[string]string, lb *l.LogBuilder, natGatewayName string, subnetId string, publicIpAllocationId string, timeoutSeconds int) (string, error) {
	if natGatewayName == "" || subnetId == "" || publicIpAllocationId == "" {
		return "", fmt.Errorf("empty parameter not allowed: natGatewayName (%s), subnetId (%s), publicIpAllocationId (%s)", natGatewayName, subnetId, publicIpAllocationId)
	}
	outCreateNatgw, err := ec2Client.CreateNatGateway(goCtx, &ec2.CreateNatGatewayInput{
		SubnetId:     aws.String(subnetId),
		AllocationId: aws.String(publicIpAllocationId),
		TagSpecifications: []types.TagSpecification{{
			ResourceType: types.ResourceTypeNatgateway,
			Tags:         mapToTags(natGatewayName, tags)}}})
	lb.AddObject(fmt.Sprintf("CreateNatGateway(natGatewayName=%s,subnetId=%s,publicIpAllocationId=%s)", natGatewayName, subnetId, publicIpAllocationId), outCreateNatgw)
	if err != nil {
		return "", fmt.Errorf("cannot create nat gateway %s: %s", natGatewayName, err.Error())
	}

	natGatewayId := *outCreateNatgw.NatGateway.NatGatewayId

	if natGatewayId == "" {
		return "", fmt.Errorf("cannot create nat gateway %s: got empty nat gateway id", natGatewayName)
	}

	startWaitTs := time.Now()
	for {
		outDescribeNatgw, err := ec2Client.DescribeNatGateways(goCtx, &ec2.DescribeNatGatewaysInput{Filter: []types.Filter{{
			Name: aws.String("nat-gateway-id"), Values: []string{natGatewayId}}}})
		lb.AddObject(fmt.Sprintf("DescribeNatGateways(nat-gateway-id=%s)", natGatewayId), outDescribeNatgw)
		if err != nil {
			return "", fmt.Errorf("cannot query for newly created nat gateway %s(%s): %s", natGatewayName, natGatewayId, err.Error())
		}

		if len(outDescribeNatgw.NatGateways) == 0 {
			return "", fmt.Errorf("cannot query for newly created nat gateway %s(%s): no nat gateways returned", natGatewayName, natGatewayId)
		}

		status := outDescribeNatgw.NatGateways[0].State

		if status == types.NatGatewayStateAvailable {
			break
		}
		if status != types.NatGatewayStatePending {
			return "", fmt.Errorf("nat gateway %s was created, but has unexpected status %s", natGatewayId, status)
		}
		if time.Since(startWaitTs).Seconds() > float64(timeoutSeconds) {
			return "", fmt.Errorf("giving up after waiting for nat gateway %s to be created after %ds", natGatewayId, timeoutSeconds)
		}
		time.Sleep(3 * time.Second)
	}
	return natGatewayId, nil
}

func DeleteNatGateway(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, natGatewayId string, timeoutSeconds int) error {
	outDeleteNatgw, err := ec2Client.DeleteNatGateway(goCtx, &ec2.DeleteNatGatewayInput{
		NatGatewayId: aws.String(natGatewayId)})
	lb.AddObject(fmt.Sprintf("DeleteNatGateway(natGatewayId=%s)", natGatewayId), outDeleteNatgw)
	if err != nil {
		return fmt.Errorf("cannot delete nat gateway %s: %s", natGatewayId, err.Error())
	}

	// Wait until natgw is trully gone, otherwise internet gateway (router) deletion may choke with
	// Network vpc-... has some mapped public address(es). Please unmap those public address(es) before detaching the gateway.
	startWaitTs := time.Now()
	for {
		outDescribeNatgw, err := ec2Client.DescribeNatGateways(goCtx, &ec2.DescribeNatGatewaysInput{Filter: []types.Filter{{
			Name: aws.String("nat-gateway-id"), Values: []string{natGatewayId}}}})
		lb.AddObject(fmt.Sprintf("DescribeNatGateways(nat-gateway-id=%s)", natGatewayId), outDescribeNatgw)
		if err != nil {
			return fmt.Errorf("cannot query for deleted nat gateway %s: %s", natGatewayId, err.Error())
		}

		if len(outDescribeNatgw.NatGateways) == 0 {
			return fmt.Errorf("cannot query for deleted nat gateway %s: no nat gateways returned", natGatewayId)
		}

		status := outDescribeNatgw.NatGateways[0].State

		if status == types.NatGatewayStateDeleted {
			break
		}
		if status != types.NatGatewayStateDeleting {
			return fmt.Errorf("nat gateway %s was deleted, but has unexpected status %s", natGatewayId, status)
		}
		if time.Since(startWaitTs).Seconds() > float64(timeoutSeconds) {
			return fmt.Errorf("giving up after waiting for nat gateway %s to be deleted after %ds", natGatewayId, timeoutSeconds)
		}
		time.Sleep(3 * time.Second)
	}
	return nil
}

func CreateRouteTableForVpc(ec2Client *ec2.Client, goCtx context.Context, tags map[string]string, lb *l.LogBuilder, routeTableName string, vpcId string) (string, error) {
	if routeTableName == "" || vpcId == "" {
		return "", fmt.Errorf("empty parameter not allowed: routeTableName (%s), vpcId (%s)", routeTableName, vpcId)
	}
	out, err := ec2Client.CreateRouteTable(goCtx, &ec2.CreateRouteTableInput{
		VpcId: aws.String(vpcId),
		TagSpecifications: []types.TagSpecification{{
			ResourceType: types.ResourceTypeRouteTable,
			Tags:         mapToTags(routeTableName, tags)}}})
	lb.AddObject(fmt.Sprintf("CreateRouteTable(routeTableName=%s,vpcId=%s)", routeTableName, vpcId), out)
	if err != nil {
		return "", fmt.Errorf("cannot create route table %s: %s", routeTableName, err.Error())
	}
	return *out.RouteTable.RouteTableId, nil
}

func GetRouteTableByName(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, routeTableName string) (string, string, error) {
	out, err := ec2Client.DescribeRouteTables(goCtx, &ec2.DescribeRouteTablesInput{
		Filters: []types.Filter{{Name: aws.String("tag:Name"), Values: []string{routeTableName}}}})
	lb.AddObject(fmt.Sprintf("DescribeRouteTable(tag:Name=%s)", routeTableName), out)
	if err != nil {
		return "", "", fmt.Errorf("cannot find route table %s: %s", routeTableName, err.Error())
	}
	if len(out.RouteTables) == 0 {
		return "", "", nil
	}
	return *out.RouteTables[0].RouteTableId, *out.RouteTables[0].VpcId, nil
}

func DeleteRouteTable(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, routeTableId string) error {
	out, err := ec2Client.DeleteRouteTable(goCtx, &ec2.DeleteRouteTableInput{RouteTableId: aws.String(routeTableId)})
	lb.AddObject(fmt.Sprintf("DeleteRouteTable(RouteTableId=%s)", routeTableId), out)
	if err != nil {
		return fmt.Errorf("cannot delete route table %s: %s", routeTableId, err.Error())
	}
	return nil
}

func AssociateRouteTableWithSubnet(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, routeTableId string, subnetId string) (string, error) {
	if routeTableId == "" || subnetId == "" {
		return "", fmt.Errorf("empty parameter not allowed: routeTableId (%s), subnetId (%s)", routeTableId, subnetId)
	}
	out, err := ec2Client.AssociateRouteTable(goCtx, &ec2.AssociateRouteTableInput{
		RouteTableId: aws.String(routeTableId),
		SubnetId:     aws.String(subnetId)})
	lb.AddObject(fmt.Sprintf("AssociateRouteTable(routeTableId=%s,subnetId=%s)", routeTableId, subnetId), out)
	if err != nil {
		return "", fmt.Errorf("cannot associate route table %s with subnet %s: %s", routeTableId, subnetId, err.Error())
	}
	if *out.AssociationId == "" {
		return "", fmt.Errorf("cannot associate route table %s with subnet %s: got empty association id", routeTableId, subnetId)
	}
	return *out.AssociationId, nil
}

func GetInternetGatewayIdByName(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, internetGatewayName string) (string, error) {
	out, err := ec2Client.DescribeInternetGateways(goCtx, &ec2.DescribeInternetGatewaysInput{Filters: []types.Filter{{Name: aws.String("tag:Name"), Values: []string{internetGatewayName}}}})
	lb.AddObject(fmt.Sprintf("DescribeInternetGateways(tag:Name=%s)", internetGatewayName), out)
	if err != nil {
		return "", fmt.Errorf("cannot describe internet gateway (router) %s: %s", internetGatewayName, err.Error())
	}
	if len(out.InternetGateways) > 0 {
		return *out.InternetGateways[0].InternetGatewayId, nil
	}
	return "", nil
}

func CreateInternetGateway(ec2Client *ec2.Client, goCtx context.Context, tags map[string]string, lb *l.LogBuilder, internetGatewayName string) (string, error) {
	if internetGatewayName == "" {
		return "", fmt.Errorf("empty parameter not allowed: internetGatewayName (%s)", internetGatewayName)
	}
	outCreateRouter, err := ec2Client.CreateInternetGateway(goCtx, &ec2.CreateInternetGatewayInput{
		TagSpecifications: []types.TagSpecification{{
			ResourceType: types.ResourceTypeInternetGateway,
			Tags:         mapToTags(internetGatewayName, tags)}}})
	lb.AddObject(fmt.Sprintf("CreateInternetGateway(tag:Name=%s)", internetGatewayName), outCreateRouter)
	if err != nil {
		return "", fmt.Errorf("cannot create internet gateway (router) %s: %s", internetGatewayName, err.Error())
	}

	if *outCreateRouter.InternetGateway.InternetGatewayId == "" {
		return "", fmt.Errorf("cannot create internet gateway (router) %s: empty id returned", internetGatewayName)
	}

	// No need to wait/verify for creations: a router is created synchronously

	return *outCreateRouter.InternetGateway.InternetGatewayId, nil
}

func DeleteInternetGateway(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, internetGatewayId string) error {
	out, err := ec2Client.DeleteInternetGateway(goCtx, &ec2.DeleteInternetGatewayInput{
		InternetGatewayId: aws.String(internetGatewayId)})
	lb.AddObject(fmt.Sprintf("DeleteInternetGateway(internetGatewayId=%s)", internetGatewayId), out)
	if err != nil {
		return fmt.Errorf("cannot delete internet gateway (router) %s: %s", internetGatewayId, err.Error())
	}
	return nil
}

func GetInternetGatewayVpcAttachmentById(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, internetGatewayId string) (string, types.AttachmentStatus, error) {
	if internetGatewayId == "" {
		return "", types.AttachmentStatusDetached, fmt.Errorf("empty parameter not allowed: internetGatewayId (%s)", internetGatewayId)
	}
	out, err := ec2Client.DescribeInternetGateways(goCtx, &ec2.DescribeInternetGatewaysInput{
		Filters: []types.Filter{{Name: aws.String("internet-gateway-id"), Values: []string{internetGatewayId}}}})
	lb.AddObject(fmt.Sprintf("DescribeInternetGateways(internet-gateway-id=%s)", internetGatewayId), out)
	if err != nil {
		return "", types.AttachmentStatusDetached, fmt.Errorf("cannot verify internet gateway (router) %s: %s", internetGatewayId, err.Error())
	}
	if len(out.InternetGateways) == 0 {
		return "", types.AttachmentStatusDetached, fmt.Errorf("cannot verify internet gateway (router) %s: zero internet gateways returned", internetGatewayId)
	}
	if len(out.InternetGateways[0].Attachments) == 0 {
		return "", types.AttachmentStatusDetached, nil
	}
	return *out.InternetGateways[0].Attachments[0].VpcId, out.InternetGateways[0].Attachments[0].State, nil
}

func AttachInternetGatewayToVpc(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, internetGatewayId string, vpcId string) error {
	if internetGatewayId == "" || vpcId == "" {
		return fmt.Errorf("empty parameter not allowed: internetGatewayId (%s), vpcId (%s)", internetGatewayId, vpcId)
	}
	out, err := ec2Client.AttachInternetGateway(goCtx, &ec2.AttachInternetGatewayInput{
		InternetGatewayId: aws.String(internetGatewayId),
		VpcId:             aws.String(vpcId)})
	lb.AddObject(fmt.Sprintf("AttachInternetGateway(internetGatewayId=%s,vpcId=%s)", internetGatewayId, vpcId), out)
	if err != nil {
		return fmt.Errorf("cannot attach internet gateway (router) %s to vpc %s: %s", internetGatewayId, vpcId, err.Error())
	}
	return nil
}

func DetachInternetGatewayFromVpc(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, internetGatewayId string, vpcId string) error {
	if internetGatewayId == "" || vpcId == "" {
		return fmt.Errorf("empty parameter not allowed: internetGatewayId (%s), vpcId (%s)", internetGatewayId, vpcId)
	}
	out, err := ec2Client.DetachInternetGateway(goCtx, &ec2.DetachInternetGatewayInput{
		InternetGatewayId: aws.String(internetGatewayId),
		VpcId:             aws.String(vpcId)})
	lb.AddObject(fmt.Sprintf("DetachInternetGateway(internetGatewayId=%s,vpcId=%s)", internetGatewayId, vpcId), out)
	if err != nil {
		return fmt.Errorf("cannot detach internet gateway (router) %s from vpc %s: %s", internetGatewayId, vpcId, err.Error())
	}
	return nil
}

func GetVpcDefaultRouteTable(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, vpcId string) (string, error) {
	if vpcId == "" {
		return "", fmt.Errorf("empty parameter not allowed: vpcId (%s)", vpcId)
	}
	out, err := ec2Client.DescribeRouteTables(goCtx, &ec2.DescribeRouteTablesInput{
		Filters: []types.Filter{
			{Name: aws.String("association.main"), Values: []string{"true"}},
			{Name: aws.String("vpc-id"), Values: []string{vpcId}}}})
	lb.AddObject(fmt.Sprintf("DescribeRouteTables(association.main=true,vpc-id=%s)", vpcId), out)
	if err != nil {
		return "", fmt.Errorf("cannot obtain default (main) route table for vpc %s: %s", vpcId, err.Error())
	}
	if len(out.RouteTables) == 0 {
		return "", fmt.Errorf("cannot obtain default (main) route table for vpc %s: no route tables returned", vpcId)
	}

	return *out.RouteTables[0].RouteTableId, nil
}
