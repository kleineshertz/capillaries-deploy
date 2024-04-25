package cldaws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
)

func GetPublicIpAllocation(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, publicIp string) (string, error) {
	out, err := client.DescribeAddresses(goCtx, &ec2.DescribeAddressesInput{PublicIps: []string{publicIp}})
	lb.AddObject(out)
	if err != nil {
		return "", fmt.Errorf("cannot get public IP %s allocation id: %s", publicIp, err.Error())
	}
	if len(out.Addresses) == 0 {
		return "", nil
	}

	return *out.Addresses[0].AllocationId, nil
}

func GetPublicIpAssoiatedInstance(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, publicIp string) (string, error) {
	out, err := client.DescribeAddresses(goCtx, &ec2.DescribeAddressesInput{Filters: []types.Filter{types.Filter{
		Name: aws.String("public-ip"), Values: []string{publicIp}}}})
	lb.AddObject(out)
	if err != nil {
		return "", fmt.Errorf("cannot check public IP instance id %s:%s", publicIp, err.Error())
	}

	if len(out.Addresses) > 0 && *out.Addresses[0].InstanceId != "" {
		return *out.Addresses[0].InstanceId, nil
	}

	return "", nil
}

func AllocateFloatingIp(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, publicIpDesc string) (string, error) {
	out, err := client.AllocateAddress(goCtx, &ec2.AllocateAddressInput{TagSpecifications: []types.TagSpecification{{
		ResourceType: types.ResourceTypeElasticIp,
		Tags: []types.Tag{
			{Key: aws.String("Name"), Value: aws.String(publicIpDesc)}}}}})
	lb.AddObject(out)
	if err != nil {
		return "", fmt.Errorf("cannot allocate %s IP address:%s", publicIpDesc, err.Error())
	}

	return *out.PublicIp, nil
}

func ReleaseFloatingIp(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, publicIp string, publicIpDesc string) error {
	if publicIp == "" {
		lb.Add(fmt.Sprintf("%s IP is already empty, nothing to delete", publicIpDesc))
		return nil
	}

	allocationId, err := GetPublicIpAllocation(client, goCtx, lb, publicIp)
	if err != nil {
		return fmt.Errorf("cannot find %s IP address %s to delete:%s", publicIpDesc, publicIp, err.Error())
	}

	if allocationId != "" {
		outDel, err := client.ReleaseAddress(goCtx, &ec2.ReleaseAddressInput{AllocationId: aws.String(allocationId)})
		lb.AddObject(outDel)
		if err != nil {
			return fmt.Errorf("cannot release %s IP address %s to delete:%s", publicIpDesc, publicIp, err.Error())
		}
	}
	return nil
}
