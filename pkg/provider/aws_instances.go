package provider

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/capillariesio/capillaries-deploy/pkg/cld/cldaws"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
)

func (p *AwsDeployProvider) HarvestInstanceTypesByFlavorNames(flavorMap map[string]string) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)

	for flavorName := range flavorMap {
		instanceType, err := cldaws.GetInstanceType(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, flavorName)
		if err != nil {
			return lb.Complete(err)
		}
		flavorMap[flavorName] = instanceType
	}
	return lb.Complete(nil)
}

func (p *AwsDeployProvider) HarvestImageIdsByImageNames(imageMap map[string]string) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)

	for imageId := range imageMap {
		checkedImageId, err := cldaws.VerifyImageId(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, imageId)
		if err != nil {
			return lb.Complete(err)
		}
		imageMap[imageId] = checkedImageId
	}
	return lb.Complete(nil)
}

func (p *AwsDeployProvider) VerifyKeypairs(keypairMap map[string]struct{}) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)

	for keypairName := range keypairMap {
		err := cldaws.VerifyKeypair(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, keypairName)
		if err != nil {
			return lb.Complete(err)
		}
	}
	return lb.Complete(nil)
}

func (p *AwsDeployProvider) CreateInstanceAndWaitForCompletion(iNickname string, instanceTypeString string, imageId string) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName()+":"+iNickname, p.GetCtx().IsVerbose)

	hostName := p.GetCtx().PrjPair.Live.Instances[iNickname].HostName
	externalIpAddress := p.GetCtx().PrjPair.Live.Instances[iNickname].ExternalIpAddress

	// If floating ip is being requested (it's a bastion instance), but it's already assigned, fail

	if externalIpAddress != "" {
		associatedInstanceId, err := cldaws.GetPublicIpAssoiatedInstance(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, externalIpAddress)
		if err != nil {
			return lb.Complete(err)
		}
		if associatedInstanceId != "" {
			return lb.Complete(fmt.Errorf("cannot create instance %s, floating ip %s is already assigned, see instance %s", hostName, externalIpAddress, associatedInstanceId))
		}
	}

	// Check if the instance already exists

	foundInstanceIdByName, foundInstanceStateByName, err := cldaws.GetInstanceIdAndStateByHostName(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, hostName)
	if err != nil {
		return lb.Complete(err)
	}

	if p.GetCtx().PrjPair.Live.Instances[iNickname].Id == "" {
		// If it was already created, save it for future use, but do not create
		if foundInstanceIdByName != "" && (foundInstanceStateByName == types.InstanceStateNameRunning || foundInstanceStateByName == types.InstanceStateNamePending) {
			lb.Add(fmt.Sprintf("instance %s(%s) already there, updating project", hostName, foundInstanceIdByName))
			p.GetCtx().PrjPair.SetInstanceId(iNickname, foundInstanceIdByName)
			return lb.Complete(nil)
		}
	} else {
		if foundInstanceIdByName == "" {
			// It was supposed to be there, but it's not present, complain
			return lb.Complete(fmt.Errorf("requested instance id %s not present, consider removing this id from the project file", p.GetCtx().PrjPair.Live.Instances[iNickname].Id))
		} else if p.GetCtx().PrjPair.Live.Instances[iNickname].Id != foundInstanceIdByName {
			// It is already there, but has different id, complain
			return lb.Complete(fmt.Errorf("requested instance id %s not matching existing instance id %s", p.GetCtx().PrjPair.Live.Instances[iNickname].Id, foundInstanceIdByName))
		}
	}

	if p.GetCtx().PrjPair.Live.Instances[iNickname].Id != "" {
		lb.Add(fmt.Sprintf("instance %s(%s) already there, no need to create", hostName, foundInstanceIdByName))
		return lb.Complete(nil)
	}

	// Verify instance's subnet

	subnetId := ""
	if p.GetCtx().PrjPair.Live.Instances[iNickname].SubnetType == "public" {
		if p.GetCtx().PrjPair.Live.Network.PublicSubnet.Id == "" {
			return lb.Complete(fmt.Errorf("requested instance %s is supposed to be in public subnet, but public subnet was not initialized yet", hostName))
		}
		subnetId = p.GetCtx().PrjPair.Live.Network.PublicSubnet.Id
	} else if p.GetCtx().PrjPair.Live.Instances[iNickname].SubnetType == "private" {
		if p.GetCtx().PrjPair.Live.Network.PrivateSubnet.Id == "" {
			return lb.Complete(fmt.Errorf("requested instance %s is supposed to be in private subnet, but private subnet was not initialized yet", hostName))
		}
		subnetId = p.GetCtx().PrjPair.Live.Network.PrivateSubnet.Id
	} else {
		return lb.Complete(fmt.Errorf("requested instance %s is supposed to be in subnet of unknown type %s", hostName, p.GetCtx().PrjPair.Live.Instances[iNickname].SubnetType))
	}

	// Verify/convert instance type

	instanceId, err := cldaws.CreateInstance(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb,
		instanceTypeString,
		imageId,
		hostName,
		p.GetCtx().PrjPair.Live.Instances[iNickname].IpAddress,
		p.GetCtx().PrjPair.Live.SecurityGroups[p.GetCtx().PrjPair.Live.Instances[iNickname].SecurityGroupNickname].Id,
		p.GetCtx().PrjPair.Live.Instances[iNickname].RootKeyName,
		subnetId,
		p.GetCtx().PrjPair.Live.Timeouts.CreateInstance)
	if err != nil {
		return lb.Complete(err)
	}

	p.GetCtx().PrjPair.SetInstanceId(iNickname, instanceId)

	if p.GetCtx().PrjPair.Live.Instances[iNickname].ExternalIpAddress != "" {
		_, err = cldaws.AssignAwsFloatingIp(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb,
			instanceId, p.GetCtx().PrjPair.Live.Instances[iNickname].ExternalIpAddress)
		if err != nil {
			return lb.Complete(err)
		}
	}

	return lb.Complete(nil)
}

func (p *AwsDeployProvider) DeleteInstance(iNickname string) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName()+":"+iNickname, p.GetCtx().IsVerbose)

	hostName := p.GetCtx().PrjPair.Live.Instances[iNickname].HostName
	if hostName == "" {
		return lb.Complete(fmt.Errorf("empty parameter not allowed: hostName (%s)", hostName))
	}

	foundId, foundState, err := cldaws.GetInstanceIdAndStateByHostName(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, hostName)
	if err != nil {
		return lb.Complete(err)
	}

	if p.GetCtx().PrjPair.Live.Instances[iNickname].Id == "" {
		if foundId != "" && (foundState == types.InstanceStateNameRunning || foundState == types.InstanceStateNamePending) {
			// Update project, delete found
			p.GetCtx().PrjPair.SetInstanceId(iNickname, foundId)
		}
	} else {
		if foundId == "" {
			// Already deleted, update project
			p.GetCtx().PrjPair.CleanInstance(iNickname)
		} else if p.GetCtx().PrjPair.Live.Instances[iNickname].Id != foundId {
			// It is already there, but has different id, complain
			return lb.Complete(fmt.Errorf("requested instance id %s not matching existing instance id %s", p.GetCtx().PrjPair.Live.Instances[iNickname].Id, foundId))
		}
	}

	// At this point, the project contains relevant resource id
	if p.GetCtx().PrjPair.Live.Instances[iNickname].Id == "" {
		lb.Add(fmt.Sprintf("will not delete instance %s, nothing to delete", iNickname))
		return lb.Complete(nil)
	}

	err = cldaws.DeleteInstance(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, p.GetCtx().PrjPair.Live.Instances[iNickname].Id, p.GetCtx().PrjPair.Live.Timeouts.DeleteInstance)
	if err != nil {
		return lb.Complete(err)
	}
	p.GetCtx().PrjPair.CleanInstance(iNickname)

	return lb.Complete(nil)
}
