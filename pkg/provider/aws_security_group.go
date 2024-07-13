package provider

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/capillariesio/capillaries-deploy/pkg/cld/cldaws"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
	"github.com/capillariesio/capillaries-deploy/pkg/prj"
)

func createAwsSecurityGroup(ec2Client *ec2.Client, goCtx context.Context, tags map[string]string, lb *l.LogBuilder, sgDef *prj.SecurityGroupDef, vpcId string) error {
	groupId, err := cldaws.GetSecurityGroupIdByName(ec2Client, goCtx, lb, sgDef.Name)
	if err != nil {
		return err
	}

	if groupId == "" {
		groupId, err = cldaws.CreateSecurityGroup(ec2Client, goCtx, tags, lb, sgDef.Name, vpcId)
		if err != nil {
			return err
		}
	}

	for _, rule := range sgDef.Rules {
		err := cldaws.AuthorizeSecurityGroupIngress(ec2Client, goCtx, lb, groupId, rule.Protocol, int32(rule.Port), rule.RemoteIp)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *AwsDeployProvider) CreateSecurityGroups() (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.DeployCtx.IsVerbose)

	vpcId, err := cldaws.GetVpcIdByName(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, p.DeployCtx.Project.Network.Name)
	if err != nil {
		return lb.Complete(err)
	}

	if vpcId == "" {
		return lb.Complete(fmt.Errorf("cannot create security groups, vpc %s does not exist", p.DeployCtx.Project.Network.Name))
	}

	for _, sgDef := range p.DeployCtx.Project.SecurityGroups {
		err := createAwsSecurityGroup(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, p.DeployCtx.Tags, lb, sgDef, vpcId)
		if err != nil {
			return lb.Complete(err)
		}
	}
	return lb.Complete(nil)
}

func deleteAwsSecurityGroup(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, sgDef *prj.SecurityGroupDef) error {
	foundId, err := cldaws.GetSecurityGroupIdByName(ec2Client, goCtx, lb, sgDef.Name)
	if err != nil {
		return err
	}

	if foundId == "" {
		lb.Add(fmt.Sprintf("will not delete security group %s, nothing to delete", sgDef.Name))
		return nil
	}

	return cldaws.DeleteSecurityGroup(ec2Client, goCtx, lb, foundId)
}

func (p *AwsDeployProvider) DeleteSecurityGroups() (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.DeployCtx.IsVerbose)
	for _, sgDef := range p.DeployCtx.Project.SecurityGroups {
		err := deleteAwsSecurityGroup(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, sgDef)
		if err != nil {
			return lb.Complete(err)
		}
	}
	return lb.Complete(nil)
}
