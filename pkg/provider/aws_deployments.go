package provider

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	taggingTypes "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"
	"github.com/capillariesio/capillaries-deploy/pkg/cld"
	"github.com/capillariesio/capillaries-deploy/pkg/cld/cldaws"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
)

func (p *AwsDeployProvider) listDeployments() (map[string]int, l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.DeployCtx.IsVerbose)
	resources, err := cldaws.GetResourcesByTag(p.DeployCtx.Aws.TaggingClient, p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, p.DeployCtx.Aws.Config.Region,
		[]taggingTypes.TagFilter{{Key: aws.String(cld.DeploymentOperatorTagName), Values: []string{cld.DeploymentOperatorTagValue}}}, false)
	if err != nil {
		logMsg, err := lb.Complete(err)
		return nil, logMsg, err
	}
	deploymentResCount := map[string]int{}
	for _, res := range resources {
		if deploymentNameCount, ok := deploymentResCount[res.DeploymentName]; ok {
			deploymentResCount[res.DeploymentName] = deploymentNameCount + 1
		} else {
			deploymentResCount[res.DeploymentName] = 1
		}
	}
	logMsg, _ := lb.Complete(nil)
	return deploymentResCount, logMsg, nil
}

func (p *AwsDeployProvider) listDeploymentResources() ([]*cld.Resource, l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.DeployCtx.IsVerbose)
	resources, err := cldaws.GetResourcesByTag(p.DeployCtx.Aws.TaggingClient, p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, p.DeployCtx.Aws.Config.Region,
		[]taggingTypes.TagFilter{
			{Key: aws.String(cld.DeploymentOperatorTagName), Values: []string{cld.DeploymentOperatorTagValue}},
			{Key: aws.String(cld.DeploymentNameTagName), Values: []string{p.DeployCtx.Project.DeploymentName}}}, true)
	if err != nil {
		logMsg, err := lb.Complete(err)
		return nil, logMsg, err
	}
	logMsg, _ := lb.Complete(nil)
	return resources, logMsg, nil
}
