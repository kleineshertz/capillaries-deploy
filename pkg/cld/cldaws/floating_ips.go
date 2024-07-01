package cldaws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
)

func GetPublicIpAddressAllocationAssociatedInstanceByName(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, ipName string) (string, string, string, error) {
	out, err := ec2Client.DescribeAddresses(goCtx, &ec2.DescribeAddressesInput{Filters: []types.Filter{{Name: aws.String("tag:Name"), Values: []string{ipName}}}})
	lb.AddObject(fmt.Sprintf("DescribeAddresses(tag:Name=%s)", ipName), out)
	if err != nil {
		return "", "", "", fmt.Errorf("cannot get public ip named %s: %s", ipName, err.Error())
	}
	if len(out.Addresses) == 0 {
		return "", "", "", nil
	}

	var allocationId string
	if out.Addresses[0].AllocationId != nil {
		allocationId = *out.Addresses[0].AllocationId
	}

	var instanceId string
	if out.Addresses[0].InstanceId != nil {
		instanceId = *out.Addresses[0].InstanceId
	}

	return *out.Addresses[0].PublicIp, allocationId, instanceId, nil
}

func AllocateFloatingIpByName(ec2Client *ec2.Client, goCtx context.Context, tags map[string]string, lb *l.LogBuilder, ipName string) (string, error) {
	out, err := ec2Client.AllocateAddress(goCtx, &ec2.AllocateAddressInput{TagSpecifications: []types.TagSpecification{{
		ResourceType: types.ResourceTypeElasticIp,
		Tags:         mapToTags(ipName, tags)}}})
	lb.AddObject(fmt.Sprintf("AllocateAddress(tag:Name=%s)", ipName), out)
	if err != nil {
		return "", fmt.Errorf("cannot allocate %s IP address:%s", ipName, err.Error())
	}

	return *out.PublicIp, nil
}

func ReleaseFloatingIpByAllocationId(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, allocationId string) error {
	out, err := ec2Client.ReleaseAddress(goCtx, &ec2.ReleaseAddressInput{AllocationId: aws.String(allocationId)})
	lb.AddObject(fmt.Sprintf("ReleaseAddress(allocationId=%s)", allocationId), out)
	if err != nil {
		return fmt.Errorf("cannot release IP address allocation id %s: %s", allocationId, err.Error())
	}
	return nil
}
