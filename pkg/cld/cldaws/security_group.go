package cldaws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
)

func GetSecurityGroupIdByName(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, securityGroupName string) (string, error) {
	if securityGroupName == "" {
		return "", fmt.Errorf("security group name cannot be empty")
	}
	out, err := client.DescribeSecurityGroups(goCtx, &ec2.DescribeSecurityGroupsInput{Filters: []types.Filter{{
		Name: aws.String("tag:Name"), Values: []string{securityGroupName}}}})
	lb.AddObject(out)
	if err != nil {
		return "", fmt.Errorf("cannot describe security group %s: %s", securityGroupName, err.Error())
	}
	if len(out.SecurityGroups) > 0 {
		return *out.SecurityGroups[0].GroupId, nil
	}
	return "", nil
}

func CreateSecurityGroup(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, securityGroupName string, vpcId string) (string, error) {
	if securityGroupName == "" {
		return "", fmt.Errorf("security group name cannot be empty")
	}
	out, err := client.CreateSecurityGroup(goCtx, &ec2.CreateSecurityGroupInput{
		VpcId:       aws.String(vpcId),
		GroupName:   aws.String(securityGroupName),
		Description: aws.String(securityGroupName),
		TagSpecifications: []types.TagSpecification{{
			ResourceType: types.ResourceTypeSecurityGroup,
			Tags:         []types.Tag{{Key: aws.String("Name"), Value: aws.String(securityGroupName)}}}}})
	lb.AddObject(out)
	if err != nil {
		return "", fmt.Errorf("cannot create security group %s: %s", securityGroupName, err.Error())
	}
	return *out.GroupId, nil
}

func AuthorizeSecurityGroupIngress(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, securityGroupId string, ipProtocol string, port int32, cidr string) error {
	if securityGroupId == "" {
		return fmt.Errorf("security group id cannot be empty")
	}
	out, err := client.AuthorizeSecurityGroupIngress(goCtx, &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:    aws.String(securityGroupId),
		IpProtocol: aws.String(ipProtocol),
		FromPort:   aws.Int32(port),
		ToPort:     aws.Int32(port),
		CidrIp:     aws.String(cidr)})
	lb.AddObject(out)
	if err != nil {
		return fmt.Errorf("cannot authorize security group %s ingress: %s", securityGroupId, err.Error())
	}
	if !*out.Return {
		return fmt.Errorf("cannot authorize security group %s ingress: aws returned false", securityGroupId)
	}
	return nil
}

func DeleteSecurityGroup(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, securityGroupId string) error {
	if securityGroupId == "" {
		return fmt.Errorf("security group id cannot be empty")
	}
	out, err := client.DeleteSecurityGroup(goCtx, &ec2.DeleteSecurityGroupInput{GroupId: aws.String(securityGroupId)})
	lb.AddObject(out)
	if err != nil {
		return fmt.Errorf("cannot delete security group %s: %s", securityGroupId, err.Error())
	}
	return nil
}
