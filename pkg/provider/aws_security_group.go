package provider

import (
	"fmt"
	"strings"

	"github.com/capillariesio/capillaries-deploy/pkg/cld/cldaws"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
)

func createAwsSecurityGroup(p *AwsDeployProvider, sgNickname string) (l.LogMsg, error) {
	lb := l.NewLogBuilder(cldaws.CurAwsFuncName(), p.GetCtx().IsVerbose)

	sgDef := p.GetCtx().PrjPair.Live.SecurityGroups[sgNickname]
	foundGroupIdByName, err := cldaws.GetSecurityGroupIdByName(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, sgDef.Name)
	if err != nil {
		return lb.Complete(err)
	}

	if sgDef.Id == "" {
		// If it was already created, save it for future use, but do not create
		if foundGroupIdByName != "" {
			lb.Add(fmt.Sprintf("security group %s(%s) already there, updating project", sgDef.Name, foundGroupIdByName))
			p.GetCtx().PrjPair.SetSecurityGroupId(sgNickname, foundGroupIdByName)
		}
	} else {
		if foundGroupIdByName == "" {
			// It was supposed to be there, but it's not present, complain
			return lb.Complete(fmt.Errorf("requested security group id %s not present, consider removing this id from the project file", sgDef.Id))
		} else if sgDef.Id != foundGroupIdByName {
			// It is already there, but has different id, complain
			return lb.Complete(fmt.Errorf("requested security group id %s not matching existing security group id %s", sgDef.Id, foundGroupIdByName))
		}
	}

	if sgDef.Id == "" {
		newId, err := cldaws.CreateSecurityGroup(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, sgDef.Name, p.GetCtx().PrjPair.Live.Network.Id)
		if err != nil {
			return lb.Complete(err)
		}
		p.GetCtx().PrjPair.SetSecurityGroupId(sgNickname, newId)
	}

	for ruleIdx, rule := range sgDef.Rules {
		err := cldaws.AuthorizeSecurityGroupIngress(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, sgDef.Id, rule.Protocol, int32(rule.Port), rule.RemoteIp)
		if err != nil {
			return lb.Complete(err)
		}

		// AWS does not assign ids to rules, so use the port
		p.GetCtx().PrjPair.SetSecurityGroupRuleId(sgNickname, ruleIdx, fmt.Sprintf("%d", rule.Port))
	}

	return lb.Complete(nil)
}

func (p *AwsDeployProvider) CreateSecurityGroups() (l.LogMsg, error) {
	sb := strings.Builder{}
	for sgNickname := range p.GetCtx().PrjPair.Live.SecurityGroups {
		logMsg, err := createAwsSecurityGroup(p, sgNickname)
		l.AddLogMsg(&sb, logMsg)
		if err != nil {
			return l.LogMsg(sb.String()), err
		}
	}
	return l.LogMsg(sb.String()), nil
}

func deleteAwsSecurityGroup(p *AwsDeployProvider, sgNickname string) (l.LogMsg, error) {
	lb := l.NewLogBuilder(cldaws.CurAwsFuncName(), p.GetCtx().IsVerbose)

	sgDef := p.GetCtx().PrjPair.Live.SecurityGroups[sgNickname]
	foundGroupIdByName, err := cldaws.GetSecurityGroupIdByName(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, sgDef.Name)
	if err != nil {
		return lb.Complete(err)
	}

	if foundGroupIdByName == "" {
		lb.Add(fmt.Sprintf("security group %s not found, nothing to delete", sgDef.Name))
		p.GetCtx().PrjPair.CleanSecurityGroup(sgNickname)
		return lb.Complete(nil)
	}

	err = cldaws.DeleteSecurityGroup(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, sgDef.Id)
	if err != nil {
		return lb.Complete(err)
	}
	p.GetCtx().PrjPair.CleanSecurityGroup(sgNickname)

	return lb.Complete(nil)
}

func (p *AwsDeployProvider) DeleteSecurityGroups() (l.LogMsg, error) {
	sb := strings.Builder{}
	for sgNickname := range p.GetCtx().PrjPair.Live.SecurityGroups {
		logMsg, err := deleteAwsSecurityGroup(p, sgNickname)
		l.AddLogMsg(&sb, logMsg)
		if err != nil {
			return l.LogMsg(sb.String()), err
		}
	}
	return l.LogMsg(sb.String()), nil
}
