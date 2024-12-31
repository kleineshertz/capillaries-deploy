package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/capillariesio/capillaries-deploy/pkg/cld/cldaws"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
	"github.com/capillariesio/capillaries-deploy/pkg/prj"
)

func (p *AwsDeployProvider) HarvestInstanceTypesByFlavorNames(flavorMap map[string]string) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.DeployCtx.IsVerbose)

	for flavorName := range flavorMap {
		instanceType, err := cldaws.GetInstanceType(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, flavorName)
		if err != nil {
			return lb.Complete(err)
		}
		flavorMap[flavorName] = instanceType
	}
	return lb.Complete(nil)
}

func (p *AwsDeployProvider) HarvestImageIds(imageMap map[string]bool) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.DeployCtx.IsVerbose)

	for imageId := range imageMap {
		_, _, err := cldaws.GetImageInfoById(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, imageId)
		if err != nil {
			return lb.Complete(err)
		}
		imageMap[imageId] = true
	}
	return lb.Complete(nil)
}

func (p *AwsDeployProvider) VerifyKeypairs(keypairMap map[string]struct{}) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.DeployCtx.IsVerbose)

	for keypairName := range keypairMap {
		err := cldaws.VerifyKeypair(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, keypairName)
		if err != nil {
			return lb.Complete(err)
		}
	}
	return lb.Complete(nil)
}

func getInstanceSubnetId(p *AwsDeployProvider, lb *l.LogBuilder, iNickname string) (string, error) {
	subnetName := p.DeployCtx.Project.Instances[iNickname].SubnetName

	subnetId, err := cldaws.GetSubnetIdByName(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, subnetName)
	if err != nil {
		return "", err
	}

	if subnetId == "" {
		return "", fmt.Errorf("requested instance %s should be created in subnet %s, but this subnet does not exist yet, did you run create_networking?", iNickname, subnetName)
	}

	return subnetId, nil
}

func getInstanceSecurityGroupId(p *AwsDeployProvider, lb *l.LogBuilder, iNickname string) (string, error) {
	sgName := p.DeployCtx.Project.Instances[iNickname].SecurityGroupName

	sgId, err := cldaws.GetSecurityGroupIdByName(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, sgName)
	if err != nil {
		return "", err
	}

	if sgId == "" {
		return "", fmt.Errorf("requested instance %s should be created in security group %s, but this it does not exist yet, did you run create_security_groups?", iNickname, sgName)
	}

	return sgId, nil
}

func internalCreate(p *AwsDeployProvider, lb *l.LogBuilder, iNickname string, instanceTypeString string, imageId string, blockDeviceMappings []types.BlockDeviceMapping, subnetId string, securityGroupId string) error {
	instName := p.DeployCtx.Project.Instances[iNickname].InstName

	// Check if the instance already exists

	instanceId, foundInstanceStateByName, err := cldaws.GetInstanceIdAndStateByHostName(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, instName)
	if err != nil {
		return err
	}

	// If floating ip is being requested (it's a bastion instance), but it's already assigned, fail

	externalIpAddressName := p.DeployCtx.Project.Instances[iNickname].ExternalIpAddressName
	var externalIpAddress string
	if externalIpAddressName != "" {
		foundExternalIpAddress, _, associatedInstanceId, err := cldaws.GetPublicIpAddressAllocationAssociatedInstanceByName(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, externalIpAddressName)
		if err != nil {
			return err
		}
		if associatedInstanceId != "" && associatedInstanceId != instanceId {
			return fmt.Errorf("cannot create instance %s, floating ip %s is already assigned, see instance %s", instName, externalIpAddressName, associatedInstanceId)
		}
		externalIpAddress = foundExternalIpAddress
	}

	if instanceId != "" {
		if foundInstanceStateByName == types.InstanceStateNameRunning || foundInstanceStateByName == types.InstanceStateNamePending {
			// Assuming it's the right instance, return ok
			return nil
		} else if foundInstanceStateByName != types.InstanceStateNameTerminated {
			return fmt.Errorf("instance %s(%s) already there and has invalid state %s", instName, instanceId, foundInstanceStateByName)
		}
	}

	instanceId, err = cldaws.CreateInstance(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, p.DeployCtx.Tags, lb,
		instanceTypeString,
		imageId,
		instName,
		p.DeployCtx.Project.Instances[iNickname].IpAddress,
		securityGroupId,
		p.DeployCtx.Project.Instances[iNickname].RootKeyName,
		subnetId,
		blockDeviceMappings,
		p.DeployCtx.Project.Timeouts.CreateInstance)
	if err != nil {
		return err
	}

	if externalIpAddress != "" {
		_, err = cldaws.AssignAwsFloatingIp(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, instanceId, externalIpAddress)
		if err != nil {
			return err
		}
	}

	if p.DeployCtx.Project.Instances[iNickname].AssociatedInstanceProfile != "" {
		// Associate "RoleAccessCapillariesTestbucket" instance profile
		// (see readme, this instance profile wraps the actual role RoleAccessCapillariesTestbucket)
		// with this instance so the instance can access S3 bucket
		err = cldaws.AssociateInstanceProfile(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, instanceId, p.DeployCtx.Project.Instances[iNickname].AssociatedInstanceProfile)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *AwsDeployProvider) CreateInstanceAndWaitForCompletion(iNickname string, flavorId string, imageId string) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName()+":"+iNickname, p.DeployCtx.IsVerbose)

	subnetId, err := getInstanceSubnetId(p, lb, iNickname)
	if err != nil {
		return lb.Complete(err)
	}

	sgId, err := getInstanceSecurityGroupId(p, lb, iNickname)
	if err != nil {
		return lb.Complete(err)
	}

	return lb.Complete(internalCreate(p, lb, iNickname, flavorId, imageId, nil, subnetId, sgId))
}

func getAttachedVolumeDeviceByName(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, volName string) (string, error) {
	foundVolIdByName, err := cldaws.GetVolumeIdByName(ec2Client, goCtx, lb, volName)
	if err != nil {
		return "", err
	}

	if foundVolIdByName == "" {
		return "", fmt.Errorf("volume %s not found, cannot check if it has device name for it; have you removed the volume before detaching it?", volName)
	}

	foundDevice, _, err := cldaws.GetVolumeAttachedDeviceById(ec2Client, goCtx, lb, foundVolIdByName)
	if err != nil {
		return "", err
	}

	return foundDevice, nil
}

func getAttachedVolumes(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, volumeDefMap map[string]*prj.VolumeDef) ([]string, error) {
	attachedVols := make([]string, 0)
	for volNickname, volDef := range volumeDefMap {
		volDevice, err := getAttachedVolumeDeviceByName(ec2Client, goCtx, lb, volDef.Name)
		if err != nil {
			return []string{}, err
		}
		if volDevice != "" {
			attachedVols = append(attachedVols, fmt.Sprintf("%s(%s)", volNickname, volDevice))
		}
	}
	return attachedVols, nil
}

func (p *AwsDeployProvider) DeleteInstance(iNickname string, ignoreAttachedVolumes bool) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName()+":"+iNickname, p.DeployCtx.IsVerbose)

	if !ignoreAttachedVolumes {
		attachedVols, err := getAttachedVolumes(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, p.DeployCtx.Project.Instances[iNickname].Volumes)
		if err != nil {
			return lb.Complete(err)
		}

		if len(attachedVols) > 0 {
			return lb.Complete(fmt.Errorf("cannot delete instance %s, detach volumes first: %s", iNickname, strings.Join(attachedVols, ",")))
		}
	}

	instName := p.DeployCtx.Project.Instances[iNickname].InstName

	foundId, foundState, err := cldaws.GetInstanceIdAndStateByHostName(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, instName)
	if err != nil {
		return lb.Complete(err)
	}

	if foundId != "" && foundState == types.InstanceStateNameTerminated {
		lb.Add(fmt.Sprintf("will not delete instance %s, already terminated", iNickname))
		return lb.Complete(nil)
	} else if foundId == "" {
		lb.Add(fmt.Sprintf("will not delete instance %s, instance not found", iNickname))
		return lb.Complete(nil)
	}

	return lb.Complete(cldaws.DeleteInstance(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, foundId, p.DeployCtx.Project.Timeouts.DeleteInstance))
}

func (p *AwsDeployProvider) CreateSnapshotImage(iNickname string) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName()+":"+iNickname, p.DeployCtx.IsVerbose)

	imageName := p.DeployCtx.Project.Instances[iNickname].InstName

	foundImageId, foundImageState, _, err := cldaws.GetImageInfoByName(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, imageName)
	if err != nil {
		return lb.Complete(err)
	}

	if foundImageId != "" || (foundImageState != "" && foundImageState != types.ImageStateDeregistered) {
		return lb.Complete(fmt.Errorf("cannot create snaphost image %s, delete/deregister existing image %s first", imageName, foundImageId))
	}

	attachedVols, err := getAttachedVolumes(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, p.DeployCtx.Project.Instances[iNickname].Volumes)
	if err != nil {
		return lb.Complete(err)
	}

	if len(attachedVols) > 0 {
		return lb.Complete(fmt.Errorf("cannot create snapshot image from instance %s, detach volumes first: %s", iNickname, strings.Join(attachedVols, ",")))
	}

	foundInstanceId, foundInstanceState, err := cldaws.GetInstanceIdAndStateByHostName(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, p.DeployCtx.Project.Instances[iNickname].InstName)
	if err != nil {
		return lb.Complete(err)
	}

	if foundInstanceId == "" {
		return lb.Complete(fmt.Errorf("cannot create snapshot image from instance %s, instance not found", iNickname))
	}

	if foundInstanceState != types.InstanceStateNameRunning &&
		foundInstanceState != types.InstanceStateNameStopped {
		return lb.Complete(fmt.Errorf("cannot create snapshot image from instance %s, instance state is %s, expected running", iNickname, foundInstanceState))
	}

	if foundInstanceState != types.InstanceStateNameStopped {
		err = cldaws.StopInstance(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, foundInstanceId, p.DeployCtx.Project.Timeouts.StopInstance)
		if err != nil {
			return lb.Complete(err)
		}
	}

	imageId, err := cldaws.CreateImageFromInstance(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, p.DeployCtx.Tags, lb,
		p.DeployCtx.Project.Instances[iNickname].InstName,
		foundInstanceId,
		p.DeployCtx.Project.Timeouts.CreateImage)
	if err != nil {
		return lb.Complete(err)
	}

	_, blockDeviceMappings, err := cldaws.GetImageInfoById(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, imageId)
	if err != nil {
		return lb.Complete(err)
	}

	// Tag each ebs mapping so the volume appears in the list of billed items
	for _, mapping := range blockDeviceMappings {
		if mapping.Ebs != nil {
			if mapping.Ebs.SnapshotId != nil && *mapping.Ebs.SnapshotId != "" {
				cldaws.TagResource(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, *mapping.Ebs.SnapshotId, p.DeployCtx.Project.Instances[iNickname].InstName, p.DeployCtx.Tags)
				if err != nil {
					return lb.Complete(err)
				}
			}
		}
	}

	return lb.Complete(nil)
}

// aws ec2 run-instances --region "us-east-1" --image-id ami-0bfdcfac85eb09d46 --count 1 --instance-type c7g.large --key-name $CAPIDEPLOY_AWS_SSH_ROOT_KEYPAIR_NAME --subnet-id subnet-09e2ba71bb1a5df94 --security-group-id sg-090b9d1ef7a1d1914 --private-ip-address 10.5.1.10
// aws ec2 associate-address --instance-id i-0c4b32d20a1671b1e --public-ip 54.86.220.208
func (p *AwsDeployProvider) CreateInstanceFromSnapshotImageAndWaitForCompletion(iNickname string, flavorId string) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName()+":"+iNickname, p.DeployCtx.IsVerbose)

	subnetId, err := getInstanceSubnetId(p, lb, iNickname)
	if err != nil {
		return lb.Complete(err)
	}

	sgId, err := getInstanceSecurityGroupId(p, lb, iNickname)
	if err != nil {
		return lb.Complete(err)
	}

	imageName := p.DeployCtx.Project.Instances[iNickname].InstName
	foundImageId, foundImageState, blockDeviceMappings, err := cldaws.GetImageInfoByName(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, imageName)
	if err != nil {
		return lb.Complete(err)
	}

	if foundImageId == "" {
		return lb.Complete(fmt.Errorf("cannot create instance for %s from snapshot image %s that is not found", iNickname, imageName))
	}

	if foundImageState != types.ImageStateAvailable {
		return lb.Complete(fmt.Errorf("cannot create instance for %s from snapshot image %s of invalid state %s", iNickname, imageName, foundImageState))
	}

	isSnapshotIdFound := false
	for _, mapping := range blockDeviceMappings {
		if mapping.Ebs != nil {
			if mapping.Ebs.SnapshotId != nil && *mapping.Ebs.SnapshotId != "" {
				isSnapshotIdFound = true
			}
		}
	}

	if !isSnapshotIdFound {
		return lb.Complete(fmt.Errorf("cannot create instance from image %s/%s, image snapshot not found", iNickname, flavorId))
	}

	return lb.Complete(internalCreate(p, lb, iNickname, flavorId, foundImageId, blockDeviceMappings, subnetId, sgId))
}

func (p *AwsDeployProvider) DeleteSnapshotImage(iNickname string) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName()+":"+iNickname, p.DeployCtx.IsVerbose)

	imageName := p.DeployCtx.Project.Instances[iNickname].InstName
	foundImageId, foundImageState, blockDeviceMappings, err := cldaws.GetImageInfoByName(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, imageName)
	if err != nil {
		return lb.Complete(err)
	}

	if foundImageId == "" {
		return lb.Complete(nil)
	}

	if foundImageState == types.ImageStateDeregistered {
		lb.Add(fmt.Sprintf("will not delete image for %s, already deregistred", iNickname))
		return lb.Complete(nil)
	}

	snapshotId := ""
	for _, mapping := range blockDeviceMappings {
		if mapping.Ebs != nil {
			if mapping.Ebs.SnapshotId != nil && *mapping.Ebs.SnapshotId != "" {
				snapshotId = *mapping.Ebs.SnapshotId
			}
		}
	}

	err = cldaws.DeregisterImage(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, foundImageId)
	if err != nil {
		return lb.Complete(err)
	}

	// Now we can delete the snapshot
	if snapshotId != "" {
		err := cldaws.DeleteSnapshot(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, snapshotId)
		if err != nil {
			return lb.Complete(err)
		}
	}

	return lb.Complete(nil)
}
