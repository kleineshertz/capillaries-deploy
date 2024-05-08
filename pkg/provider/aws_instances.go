package provider

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/capillariesio/capillaries-deploy/pkg/cld/cldaws"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
	"github.com/capillariesio/capillaries-deploy/pkg/prj"
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

func (p *AwsDeployProvider) HarvestImageIds(imageMap map[string]bool) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.GetCtx().IsVerbose)

	for imageId := range imageMap {
		_, _, err := cldaws.GetImageInfo(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, imageId)
		if err != nil {
			return lb.Complete(err)
		}
		imageMap[imageId] = true
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

func internalCreate(p *AwsDeployProvider, lb *l.LogBuilder, iNickname string, instanceTypeString string, imageId string, blockDeviceMappings []types.BlockDeviceMapping) error {
	instName := p.GetCtx().PrjPair.Live.Instances[iNickname].InstName
	externalIpAddress := p.GetCtx().PrjPair.Live.Instances[iNickname].ExternalIpAddress

	// If floating ip is being requested (it's a bastion instance), but it's already assigned, fail

	if externalIpAddress != "" {
		associatedInstanceId, err := cldaws.GetPublicIpAssoiatedInstance(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, externalIpAddress)
		if err != nil {
			return err
		}
		if associatedInstanceId != "" {
			return fmt.Errorf("cannot create instance %s, floating ip %s is already assigned, see instance %s", instName, externalIpAddress, associatedInstanceId)
		}
	}

	// Check if the instance already exists

	foundInstanceIdByName, foundInstanceStateByName, err := cldaws.GetInstanceIdAndStateByHostName(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, instName)
	if err != nil {
		return err
	}

	if p.GetCtx().PrjPair.Live.Instances[iNickname].Id == "" {
		// If it was already created, save it for future use, but do not create
		if foundInstanceIdByName != "" && (foundInstanceStateByName == types.InstanceStateNameRunning || foundInstanceStateByName == types.InstanceStateNamePending) {
			lb.Add(fmt.Sprintf("instance %s(%s) already there, updating project", instName, foundInstanceIdByName))
			p.GetCtx().PrjPair.SetInstanceId(iNickname, foundInstanceIdByName)
			return nil
		}
	} else {
		if foundInstanceIdByName == "" {
			// It was supposed to be there, but it's not present, complain
			return fmt.Errorf("requested instance id %s not present, consider removing this id from the project file", p.GetCtx().PrjPair.Live.Instances[iNickname].Id)
		} else if p.GetCtx().PrjPair.Live.Instances[iNickname].Id != foundInstanceIdByName && (foundInstanceStateByName == types.InstanceStateNameRunning || foundInstanceStateByName == types.InstanceStateNamePending) {
			// It is already there, but has different id, complain
			return fmt.Errorf("requested instance id %s not matching existing instance id %s", p.GetCtx().PrjPair.Live.Instances[iNickname].Id, foundInstanceIdByName)
		}
	}

	if p.GetCtx().PrjPair.Live.Instances[iNickname].Id != "" {
		lb.Add(fmt.Sprintf("instance %s(%s) already there, no need to create", instName, foundInstanceIdByName))
		return nil
	}

	// Verify instance's subnet

	subnetId := ""
	if p.GetCtx().PrjPair.Live.Instances[iNickname].SubnetType == "public" {
		if p.GetCtx().PrjPair.Live.Network.PublicSubnet.Id == "" {
			return fmt.Errorf("requested instance %s is supposed to be in public subnet, but public subnet was not initialized yet", instName)
		}
		subnetId = p.GetCtx().PrjPair.Live.Network.PublicSubnet.Id
	} else if p.GetCtx().PrjPair.Live.Instances[iNickname].SubnetType == "private" {
		if p.GetCtx().PrjPair.Live.Network.PrivateSubnet.Id == "" {
			return fmt.Errorf("requested instance %s is supposed to be in private subnet, but private subnet was not initialized yet", instName)
		}
		subnetId = p.GetCtx().PrjPair.Live.Network.PrivateSubnet.Id
	} else {
		return fmt.Errorf("requested instance %s is supposed to be in subnet of unknown type %s", instName, p.GetCtx().PrjPair.Live.Instances[iNickname].SubnetType)
	}

	// Verify/convert instance type

	instanceId, err := cldaws.CreateInstance(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, p.GetCtx().Tags, lb,
		instanceTypeString,
		imageId,
		instName,
		p.GetCtx().PrjPair.Live.Instances[iNickname].IpAddress,
		p.GetCtx().PrjPair.Live.SecurityGroups[p.GetCtx().PrjPair.Live.Instances[iNickname].SecurityGroupNickname].Id,
		p.GetCtx().PrjPair.Live.Instances[iNickname].RootKeyName,
		subnetId,
		blockDeviceMappings,
		p.GetCtx().PrjPair.Live.Timeouts.CreateInstance)
	if err != nil {
		return err
	}

	p.GetCtx().PrjPair.SetInstanceId(iNickname, instanceId)

	if p.GetCtx().PrjPair.Live.Instances[iNickname].ExternalIpAddress != "" {
		_, err = cldaws.AssignAwsFloatingIp(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb,
			instanceId, p.GetCtx().PrjPair.Live.Instances[iNickname].ExternalIpAddress)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *AwsDeployProvider) CreateInstanceAndWaitForCompletion(iNickname string, flavorId string, imageId string) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName()+":"+iNickname, p.GetCtx().IsVerbose)
	return lb.Complete(internalCreate(p, lb, iNickname, flavorId, imageId, nil))
}

func getAttachedVolumes(iDef *prj.InstanceDef) []string {
	attachedVols := make([]string, 0)
	for volNickname, volDef := range iDef.Volumes {
		if volDef.BlockDeviceId != "" || volDef.Device != "" {
			attachedVols = append(attachedVols, volNickname)
		}
	}
	return attachedVols
}

func (p *AwsDeployProvider) DeleteInstance(iNickname string) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName()+":"+iNickname, p.GetCtx().IsVerbose)

	attachedVols := getAttachedVolumes(p.GetCtx().PrjPair.Live.Instances[iNickname])
	if len(attachedVols) > 0 {
		return lb.Complete(fmt.Errorf("cannot delete instance %s, detach volumes first, or fix the project file: %s", iNickname, strings.Join(attachedVols, ",")))
	}

	instName := p.GetCtx().PrjPair.Live.Instances[iNickname].InstName
	if instName == "" {
		return lb.Complete(fmt.Errorf("empty parameter not allowed: instName (%s)", instName))
	}

	foundId, foundState, err := cldaws.GetInstanceIdAndStateByHostName(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, instName)
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
		} else if p.GetCtx().PrjPair.Live.Instances[iNickname].Id != foundId && (foundState == types.InstanceStateNameRunning || foundState == types.InstanceStateNamePending) {
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

func (p *AwsDeployProvider) CreateSnapshotImage(iNickname string) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName()+":"+iNickname, p.GetCtx().IsVerbose)

	if p.GetCtx().PrjPair.Live.Instances[iNickname].SnapshotImageId != "" {
		return lb.Complete(fmt.Errorf("cannot create snaphost image, delete existing %s first", p.GetCtx().PrjPair.Live.Instances[iNickname].SnapshotImageId))
	}

	attachedVols := getAttachedVolumes(p.GetCtx().PrjPair.Live.Instances[iNickname])
	if len(attachedVols) > 0 {
		return lb.Complete(fmt.Errorf("cannot create snapshot image from instance %s, detach volumes first, or fix the project file: %s", iNickname, strings.Join(attachedVols, ",")))
	}

	imageId, err := cldaws.CreateImageFromInstance(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, p.GetCtx().Tags, lb,
		p.GetCtx().PrjPair.Live.Instances[iNickname].InstName,
		p.GetCtx().PrjPair.Live.Instances[iNickname].Id,
		p.GetCtx().PrjPair.Live.Timeouts.CreateImage)
	if err != nil {
		return lb.Complete(err)
	}

	// TODO: do not delete snapshot

	// Delete attached snapshot
	_, blockDeviceMappings, err := cldaws.GetImageInfo(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, imageId)
	if err != nil {
		return lb.Complete(err)
	}

	for _, mapping := range blockDeviceMappings {
		if mapping.Ebs != nil {
			if mapping.Ebs.SnapshotId != nil || *mapping.Ebs.SnapshotId != "" {
				// Tag it just in case we are not able to delete it: at least it will appear in the list of billed items
				cldaws.TagResource(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, *mapping.Ebs.SnapshotId, p.GetCtx().PrjPair.Live.Instances[iNickname].InstName, p.GetCtx().Tags)
				if err != nil {
					return lb.Complete(err)
				}
				// We will not use this snapshot on restore, we will create a new one
				err := cldaws.DeleteSnapshot(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, *mapping.Ebs.SnapshotId)
				if err != nil {
					return lb.Complete(err)
				}
			}
		}
	}

	p.GetCtx().PrjPair.SetInstanceSnapshotImageId(iNickname, imageId)

	return lb.Complete(nil)
}

// aws ec2 run-instances --region "us-east-1" --image-id ami-0bfdcfac85eb09d46 --count 1 --instance-type c7g.large --key-name $CAPIDEPLOY_AWS_SSH_ROOT_KEYPAIR_NAME --subnet-id subnet-09e2ba71bb1a5df94 --security-group-id sg-090b9d1ef7a1d1914 --private-ip-address 10.5.1.10
// aws ec2 associate-address --instance-id i-0c4b32d20a1671b1e --public-ip 54.86.220.208
func (p *AwsDeployProvider) CreateInstanceFromSnapshotImageAndWaitForCompletion(iNickname string, flavorId string) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName()+":"+iNickname, p.GetCtx().IsVerbose)

	imageId := p.GetCtx().PrjPair.Live.Instances[iNickname].SnapshotImageId

	// Verify this image is available
	if imageId == "" {
		return lb.Complete(fmt.Errorf("cannot create instance for %s from snapshot image with empty name, did you create snapshot image for it?", iNickname))
	}
	state, blockDeviceMappings, err := cldaws.GetImageInfo(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, imageId)
	if err != nil {
		return lb.Complete(err)
	}

	if state != types.ImageStateAvailable {
		return lb.Complete(fmt.Errorf("cannot create instance from image %s/%s, image state is %s", iNickname, flavorId, string(state)))
	}

	// TODO: use snapshot id, do not create a new one

	for i, mapping := range blockDeviceMappings {
		if mapping.Ebs != nil {
			if mapping.Ebs.SnapshotId != nil {
				// Do not use EBS volume snapshot, it's already gone
				blockDeviceMappings[i].Ebs.SnapshotId = nil
			}
			if mapping.Ebs.VolumeSize == nil {
				// By default, AWS instances run on a 8gb volumes (?)
				blockDeviceMappings[i].Ebs.VolumeSize = aws.Int32(8)
			}
		}
	}
	return lb.Complete(internalCreate(p, lb, iNickname, flavorId, imageId, blockDeviceMappings))
}

func (p *AwsDeployProvider) DeleteSnapshotImage(iNickname string) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName()+":"+iNickname, p.GetCtx().IsVerbose)

	// TODO: get EBS snapshot id for this image

	imageId := p.GetCtx().PrjPair.Live.Instances[iNickname].SnapshotImageId
	err := cldaws.DeregisterImage(p.GetCtx().Aws.Ec2Client, p.GetCtx().GoCtx, lb, imageId)
	if err != nil {
		return lb.Complete(err)
	}

	// TODO: delete snapshot

	p.GetCtx().PrjPair.SetInstanceSnapshotImageId(iNickname, "")
	return lb.Complete(nil)
}
