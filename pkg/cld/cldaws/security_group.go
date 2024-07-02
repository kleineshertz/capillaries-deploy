package cldaws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
)

func GetSecurityGroupIdByName(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, securityGroupName string) (string, error) {
	out, err := ec2Client.DescribeSecurityGroups(goCtx, &ec2.DescribeSecurityGroupsInput{Filters: []types.Filter{{
		Name: aws.String("tag:Name"), Values: []string{securityGroupName}}}})
	lb.AddObject(fmt.Sprintf("DescribeSecurityGroups(tag:Name=%s)", securityGroupName), out)
	if err != nil {
		return "", fmt.Errorf("cannot describe security group %s: %s", securityGroupName, err.Error())
	}
	if len(out.SecurityGroups) > 0 {
		return *out.SecurityGroups[0].GroupId, nil
	}
	return "", nil
}

func CreateSecurityGroup(ec2Client *ec2.Client, goCtx context.Context, tags map[string]string, lb *l.LogBuilder, securityGroupName string, vpcId string) (string, error) {
	if securityGroupName == "" || vpcId == "" {
		return "", fmt.Errorf("empty parameter not allowed: securityGroupName (%s), vpcId (%s)", securityGroupName, vpcId)
	}
	out, err := ec2Client.CreateSecurityGroup(goCtx, &ec2.CreateSecurityGroupInput{
		VpcId:       aws.String(vpcId),
		GroupName:   aws.String(securityGroupName),
		Description: aws.String(securityGroupName),
		TagSpecifications: []types.TagSpecification{{
			ResourceType: types.ResourceTypeSecurityGroup,
			Tags:         mapToTags(securityGroupName, tags)}}})
	lb.AddObject(fmt.Sprintf("CreateSecurityGroup(securityGroupName=%s,vpcId=%s)", securityGroupName, vpcId), out)
	if err != nil {
		return "", fmt.Errorf("cannot create security group %s: %s", securityGroupName, err.Error())
	}
	return *out.GroupId, nil
}

func AuthorizeSecurityGroupIngress(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, securityGroupId string, ipProtocol string, port int32, cidr string) error {
	if securityGroupId == "" || ipProtocol == "" || port == 0 || cidr == "" {
		return fmt.Errorf("empty parameter not allowed: securityGroupId (%s), ipProtocol (%s), port (%d), cidr (%s)", securityGroupId, ipProtocol, port, cidr)
	}
	out, err := ec2Client.AuthorizeSecurityGroupIngress(goCtx, &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:    aws.String(securityGroupId),
		IpProtocol: aws.String(ipProtocol),
		FromPort:   aws.Int32(port),
		ToPort:     aws.Int32(port),
		CidrIp:     aws.String(cidr)})
	lb.AddObject(fmt.Sprintf("AuthorizeSecurityGroupIngress(securityGroupId=%s,ipProtocol=%s,port=%d,cidr=%s)", securityGroupId, ipProtocol, port, cidr), out)
	if err != nil {
		return fmt.Errorf("cannot authorize security group %s ingress: %s", securityGroupId, err.Error())
	}
	if !*out.Return {
		return fmt.Errorf("cannot authorize security group %s ingress: aws returned false", securityGroupId)
	}
	return nil
}

func DeleteSecurityGroup(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, securityGroupId string) error {
	out, err := ec2Client.DeleteSecurityGroup(goCtx, &ec2.DeleteSecurityGroupInput{GroupId: aws.String(securityGroupId)})
	lb.AddObject(fmt.Sprintf("DeleteSecurityGroup(GroupId=%s)", securityGroupId), out)
	if err != nil {
		return fmt.Errorf("cannot delete security group %s: %s", securityGroupId, err.Error())
	}
	return nil
}
