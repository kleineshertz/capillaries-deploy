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

func GetSubnetIdByName(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, subnetName string) (string, error) {
	if subnetName == "" {
		return "", fmt.Errorf("subnet name cannot be empty")
	}
	out, err := client.DescribeSubnets(goCtx, &ec2.DescribeSubnetsInput{Filters: []types.Filter{{
		Name: aws.String("tag:Name"), Values: []string{subnetName}}}})
	lb.AddObject(out)
	if err != nil {
		return "", fmt.Errorf("error searching for subnet %s: %s", subnetName, err.Error())
	}
	if len(out.Subnets) == 0 {
		return "", nil
	}
	return *out.Subnets[0].SubnetId, nil
}

func EnsureSubnet(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, vpcId string, name string, cidr string, id string, availabilityZone string) (string, error) {
	if name == "" || cidr == "" || id == "" || availabilityZone == "" {
		return "", fmt.Errorf("subnet name(%s), cidr(%s), vpc id(%s), availability_zone(%s) cannot be empty", name, cidr, id, availabilityZone)
	}

	// Check if the subnet is already there
	foundSubnetIdByName, err := GetSubnetIdByName(client, goCtx, lb, name)
	if err != nil {
		return "", err
	}

	if id == "" {
		// If it was already created, but was not written to the prj file, save it for future use, but do not create
		if foundSubnetIdByName != "" {
			lb.Add(fmt.Sprintf("subnet %s already there, updating project with new id %s", name, foundSubnetIdByName))
			return foundSubnetIdByName, nil
		}
	} else {
		if foundSubnetIdByName == "" {
			// It was supposed to be there, but it's not present, complain
			return "", fmt.Errorf("requested subnet id %s not present, consider removing this id from the project file", id)
		} else if foundSubnetIdByName != id {
			// It is already there, but has different id, complain
			return "", fmt.Errorf("requested subnet id %s not matching existing id %s", id, foundSubnetIdByName)
		}
	}

	// Existing id matches the found id, nothing to do
	if id != "" {
		lb.Add(fmt.Sprintf("subnet %s(%s) already there, no need to create", name, foundSubnetIdByName))
		return id, nil
	}

	// Create

	outCreate, err := client.CreateSubnet(goCtx, &ec2.CreateSubnetInput{
		VpcId:            aws.String(vpcId),
		CidrBlock:        aws.String(cidr),
		AvailabilityZone: aws.String(availabilityZone),
		TagSpecifications: []types.TagSpecification{{
			ResourceType: types.ResourceTypeSubnet,
			Tags: []types.Tag{
				{Key: aws.String("Name"), Value: aws.String(name)}}}}})
	lb.AddObject(outCreate)
	if err != nil {
		return "", fmt.Errorf("error searching for private subnet %s: %s", name, err.Error())
	}

	// TODO: dhcp options and allocation pools?

	return *outCreate.Subnet.SubnetId, nil
}

func DeleteSubnet(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, subnetId string) error {
	out, err := client.DeleteSubnet(goCtx, &ec2.DeleteSubnetInput{SubnetId: aws.String(subnetId)})
	lb.AddObject(out)
	if err != nil {
		return fmt.Errorf("cannot delete subnet %s: %s", subnetId, err.Error())
	}
	return nil
}

func GetVpcIdByName(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, vpcName string) (string, error) {
	if vpcName == "" {
		return "", fmt.Errorf("vpc (network) name cannot be empty")
	}
	out, err := client.DescribeVpcs(goCtx, &ec2.DescribeVpcsInput{Filters: []types.Filter{types.Filter{
		Name: aws.String("tag:Name"), Values: []string{vpcName}}}})
	lb.AddObject(out)
	if err != nil {
		return "", fmt.Errorf("error searching for vpc (network) %s: %s", vpcName, err.Error())
	}

	if len(out.Vpcs) > 0 {
		return *out.Vpcs[0].VpcId, nil
	}

	return "", nil
}

func CreateVpc(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, vpcName string, cidrBlock string, timeoutSeconds int) (string, error) {
	outCreate, err := client.CreateVpc(goCtx, &ec2.CreateVpcInput{
		CidrBlock: aws.String(cidrBlock),
		TagSpecifications: []types.TagSpecification{{
			ResourceType: types.ResourceTypeVpc,
			Tags:         []types.Tag{{Key: aws.String("Name"), Value: aws.String(vpcName)}}}}})
	lb.AddObject(outCreate)
	if err != nil {
		return "", fmt.Errorf("cannot create vpc (network) %s: %s", vpcName, err.Error())
	}
	if outCreate.Vpc == nil {
		return "", fmt.Errorf("cannot create vpc (network) %s: returned empty vpc", vpcName)
	}

	newVpcId := *outCreate.Vpc.VpcId

	startWaitTs := time.Now()
	for {
		out, err := client.DescribeVpcs(goCtx, &ec2.DescribeVpcsInput{Filters: []types.Filter{types.Filter{
			Name: aws.String("vpc-id"), Values: []string{newVpcId}}}})
		lb.AddObject(out)
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

func DeleteVpc(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, vpcId string) error {
	out, err := client.DeleteVpc(goCtx, &ec2.DeleteVpcInput{VpcId: aws.String(vpcId)})
	lb.AddObject(out)
	if err != nil {
		return fmt.Errorf("cannot delete vpc (network) %s: %s", vpcId, err.Error())
	}
	return nil
}

func CreateInternetGatewayRoute(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, routeTableId string, destinationCidrBlock string, internetGatewayId string) error {
	out, err := client.CreateRoute(goCtx, &ec2.CreateRouteInput{
		RouteTableId:         aws.String(routeTableId),
		DestinationCidrBlock: aws.String(destinationCidrBlock),
		GatewayId:            aws.String(internetGatewayId)})
	lb.AddObject(out)
	if err != nil {
		return fmt.Errorf("cannot create route for internet gateway (router) %s, route table %s: %s", internetGatewayId, routeTableId, err.Error())
	}

	if !*out.Return {
		return fmt.Errorf("cannot create route for internet gateway (router) %s, route table %s: result false", internetGatewayId, routeTableId)
	}

	return nil
}

func CreateNatGatewayRoute(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, routeTableId string, destinationCidrBlock string, natGatewayId string) error {
	out, err := client.CreateRoute(goCtx, &ec2.CreateRouteInput{
		RouteTableId:         aws.String(routeTableId),
		DestinationCidrBlock: aws.String(destinationCidrBlock),
		NatGatewayId:         aws.String(natGatewayId)})
	lb.AddObject(out)
	if err != nil {
		return fmt.Errorf("cannot create route for nat gateway %s, route table %s: %s", natGatewayId, routeTableId, err.Error())
	}

	if !*out.Return {
		return fmt.Errorf("cannot create route for nat gateway %s, route table %s: result false", natGatewayId, routeTableId)
	}

	return nil
}

func GetNatGatewayIdByName(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, natGatewayName string) (string, error) {
	out, err := client.DescribeNatGateways(goCtx, &ec2.DescribeNatGatewaysInput{Filter: []types.Filter{types.Filter{Name: aws.String("tag:Name"), Values: []string{natGatewayName}}}})
	lb.AddObject(out)
	if err != nil {
		return "", fmt.Errorf("cannot describe natgw %s: %s", natGatewayName, err.Error())
	}
	if len(out.NatGateways) > 0 {
		return *out.NatGateways[0].NatGatewayId, nil
	}
	return "", nil
}

func CreateNatGateway(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, natGatewayName string, subnetId string, publicIpAllocationId string, timeoutSeconds int) (string, error) {
	outCreateNatgw, err := client.CreateNatGateway(goCtx, &ec2.CreateNatGatewayInput{
		SubnetId:     aws.String(subnetId),
		AllocationId: aws.String(publicIpAllocationId),
		TagSpecifications: []types.TagSpecification{{
			ResourceType: types.ResourceTypeNatgateway,
			Tags:         []types.Tag{{Key: aws.String("Name"), Value: aws.String(natGatewayName)}}}}})
	lb.AddObject(outCreateNatgw)
	if err != nil {
		return "", fmt.Errorf("cannot create nat gateway %s: %s", natGatewayName, err.Error())
	}

	natGatewayId := *outCreateNatgw.NatGateway.NatGatewayId

	if natGatewayId == "" {
		return "", fmt.Errorf("cannot create nat gateway %s: got empty nat gateway id", natGatewayName)
	}

	startWaitTs := time.Now()
	for {
		outDescribeNatgw, err := client.DescribeNatGateways(goCtx, &ec2.DescribeNatGatewaysInput{Filter: []types.Filter{types.Filter{
			Name: aws.String("nat-gateway-id"), Values: []string{natGatewayId}}}})
		lb.AddObject(outDescribeNatgw)
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
		time.Sleep(1 * time.Second)
	}
	return natGatewayId, nil
}

func DeleteNatGateway(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, natGatewayId string, timeoutSeconds int) error {
	outDeleteNatgw, err := client.DeleteNatGateway(goCtx, &ec2.DeleteNatGatewayInput{
		NatGatewayId: aws.String(natGatewayId)})
	lb.AddObject(outDeleteNatgw)
	if err != nil {
		return fmt.Errorf("cannot delete nat gateway %s: %s", natGatewayId, err.Error())
	}

	// Wait until natgw is trully gone, otherwise internet gateway (router) deletion may choke with
	// Network vpc-... has some mapped public address(es). Please unmap those public address(es) before detaching the gateway.
	startWaitTs := time.Now()
	for {
		outDescribeNatgw, err := client.DescribeNatGateways(goCtx, &ec2.DescribeNatGatewaysInput{Filter: []types.Filter{types.Filter{
			Name: aws.String("nat-gateway-id"), Values: []string{natGatewayId}}}})
		lb.AddObject(outDescribeNatgw)
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
		time.Sleep(1 * time.Second)
	}
	return nil
}

func CreateRouteTableForVpc(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, routeTableName string, vpcId string) (string, error) {
	out, err := client.CreateRouteTable(goCtx, &ec2.CreateRouteTableInput{
		VpcId: aws.String(vpcId),
		TagSpecifications: []types.TagSpecification{{
			ResourceType: types.ResourceTypeRouteTable,
			Tags:         []types.Tag{{Key: aws.String("Name"), Value: aws.String(routeTableName)}}}}})
	lb.AddObject(out)
	if err != nil {
		return "", fmt.Errorf("cannot create route table %s: %s", routeTableName, err.Error())
	}
	return *out.RouteTable.RouteTableId, nil
}

func DeleteRouteTable(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, routeTableId string) error {
	out, err := client.DeleteRouteTable(goCtx, &ec2.DeleteRouteTableInput{RouteTableId: aws.String(routeTableId)})
	lb.AddObject(out)
	if err != nil {
		return fmt.Errorf("cannot delete route table %s: %s", routeTableId, err.Error())
	}
	return nil
}

func AssociateRouteTableWithSubnet(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, routeTableId string, subnetId string) (string, error) {
	out, err := client.AssociateRouteTable(goCtx, &ec2.AssociateRouteTableInput{
		RouteTableId: aws.String(routeTableId),
		SubnetId:     aws.String(subnetId)})
	lb.AddObject(out)
	if err != nil {
		return "", fmt.Errorf("cannot associate route table %s with subnet %s: %s", routeTableId, subnetId, err.Error())
	}
	if *out.AssociationId == "" {
		return "", fmt.Errorf("cannot associate route table %s with subnet %s: got empty association id", routeTableId, subnetId)
	}
	return *out.AssociationId, nil
}

func GetInternetGatewayIdByName(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, internetGatewayName string) (string, error) {
	out, err := client.DescribeInternetGateways(goCtx, &ec2.DescribeInternetGatewaysInput{Filters: []types.Filter{types.Filter{Name: aws.String("tag:Name"), Values: []string{internetGatewayName}}}})
	lb.AddObject(out)
	if err != nil {
		return "", fmt.Errorf("cannot describe internet gateway (router) %s: %s", internetGatewayName, err.Error())
	}
	if len(out.InternetGateways) > 0 {
		return *out.InternetGateways[0].InternetGatewayId, nil
	}
	return "", nil
}

func CreateInternetGateway(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, internetGatewayName string) (string, error) {
	outCreateRouter, err := client.CreateInternetGateway(goCtx, &ec2.CreateInternetGatewayInput{
		TagSpecifications: []types.TagSpecification{{
			ResourceType: types.ResourceTypeInternetGateway,
			Tags: []types.Tag{
				{Key: aws.String("Name"), Value: aws.String(internetGatewayName)}}}}})
	lb.AddObject(outCreateRouter)
	if err != nil {
		return "", fmt.Errorf("cannot create internet gateway (router) %s: %s", internetGatewayName, err.Error())
	}

	if *outCreateRouter.InternetGateway.InternetGatewayId == "" {
		return "", fmt.Errorf("cannot create internet gateway (router) %s: empty id returned", internetGatewayName)
	}

	// No need to wait/verify for creations: a router is created synchronously

	return *outCreateRouter.InternetGateway.InternetGatewayId, nil
}

func DeleteInternetGateway(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, internetGatewayId string) error {
	out, err := client.DeleteInternetGateway(goCtx, &ec2.DeleteInternetGatewayInput{
		InternetGatewayId: aws.String(internetGatewayId)})
	lb.AddObject(out)
	if err != nil {
		return fmt.Errorf("cannot delete internet gateway (router) %s: %s", internetGatewayId, err.Error())
	}
	return nil
}

func GetInternetGatewayVpcAttachmentById(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, internetGatewayId string) (string, types.AttachmentStatus, error) {
	out, err := client.DescribeInternetGateways(goCtx, &ec2.DescribeInternetGatewaysInput{
		Filters: []types.Filter{types.Filter{Name: aws.String("internet-gateway-id"), Values: []string{internetGatewayId}}}})
	lb.AddObject(out)
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

func AttachInternetGatewayToVpc(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, internetGatewayId string, vpcId string) error {
	out, err := client.AttachInternetGateway(goCtx, &ec2.AttachInternetGatewayInput{
		InternetGatewayId: aws.String(internetGatewayId),
		VpcId:             aws.String(vpcId)})
	lb.AddObject(out)
	if err != nil {
		return fmt.Errorf("cannot attach internet gateway (router) %s to vpc %s: %s", internetGatewayId, vpcId, err.Error())
	}
	return nil
}

func DetachInternetGatewayFromVpc(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, internetGatewayId string, vpcId string) error {
	out, err := client.DetachInternetGateway(goCtx, &ec2.DetachInternetGatewayInput{
		InternetGatewayId: aws.String(internetGatewayId),
		VpcId:             aws.String(vpcId)})
	lb.AddObject(out)
	if err != nil {
		return fmt.Errorf("cannot detach internet gateway (router) %s from vpc %s: %s", internetGatewayId, vpcId, err.Error())
	}
	return nil
}

func GetVpcDefaultRouteTable(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, vpcId string) (string, error) {
	out, err := client.DescribeRouteTables(goCtx, &ec2.DescribeRouteTablesInput{
		Filters: []types.Filter{
			types.Filter{Name: aws.String("association.main"), Values: []string{"true"}},
			types.Filter{Name: aws.String("vpc-id"), Values: []string{vpcId}}}})
	lb.AddObject(out)
	if err != nil {
		return "", fmt.Errorf("cannot obtain default (main) route table for vpc %s: %s", vpcId, err.Error())
	}
	if len(out.RouteTables) == 0 {
		return "", fmt.Errorf("cannot obtain default (main) route table for vpc %s: no route tables returned", vpcId)
	}

	return *out.RouteTables[0].RouteTableId, nil
}
