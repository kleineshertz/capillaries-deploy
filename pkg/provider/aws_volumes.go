package provider

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/capillariesio/capillaries-deploy/pkg/cld/cldaws"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
	"github.com/capillariesio/capillaries-deploy/pkg/prj"
	"github.com/capillariesio/capillaries-deploy/pkg/rexec"
)

func (p *AwsDeployProvider) CreateVolume(iNickname string, volNickname string) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.DeployCtx.IsVerbose)

	volDef := p.DeployCtx.Project.Instances[iNickname].Volumes[volNickname]
	foundVolIdByName, err := cldaws.GetVolumeIdByName(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, volDef.Name)
	if err != nil {
		return lb.Complete(err)
	}

	if foundVolIdByName != "" {
		lb.Add(fmt.Sprintf("volume %s(%s) already there", volDef.Name, foundVolIdByName))
		return lb.Complete(nil)
	}

	_, err = cldaws.CreateVolume(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, p.DeployCtx.Tags, lb, volDef.Name, volDef.AvailabilityZone, int32(volDef.Size), volDef.Type)
	if err != nil {
		return lb.Complete(err)
	}

	return lb.Complete(nil)
}

// AWS hell https://stackoverflow.com/questions/70205661/correctly-specifying-device-name-for-ebs-volume-while-attaching-to-an-ec2-instan
func volNicknameToAwsSuggestedDeviceName(volumes map[string]*prj.VolumeDef, volNickname string) string {
	// Sorted list of vol nicknames
	volNicknames := make([]string, len(volumes))
	volCount := 0
	for volNickname := range volumes {
		volNicknames[volCount] = volNickname
		volCount++
	}
	sort.Slice(volNicknames, func(i, j int) bool { return volNicknames[i] > volNicknames[j] })
	volDeviceSuffix := 'f'
	for i := 0; i < len(volNicknames); i++ {
		if volNicknames[i] == volNickname {
			return "/dev/sd" + string(volDeviceSuffix)
		}
		volDeviceSuffix++
	}
	return "invalid-device-for-vol-" + volNickname
}

// Not used anymore, hopefully
// func awsFinalDeviceNameOld(suggestedDeviceName string) string {
// 	return strings.ReplaceAll(suggestedDeviceName, "/dev/sd", "/dev/xvd")
// }

func awsFinalDeviceNameNitro(suggestedDeviceName string) string {
	// See what lsblk shows for your case.
	// This is very hacky, but I didn't spend time to do it the right way
	deviceNameReplacer := strings.NewReplacer(
		"/dev/sdf", "/dev/nvme1n1",
		"/dev/sdg", "/dev/nvme2n1",
		"/dev/sdh", "/dev/nvme3n1")
	return deviceNameReplacer.Replace(suggestedDeviceName)
}

func (p *AwsDeployProvider) AttachVolume(iNickname string, volNickname string) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.DeployCtx.IsVerbose)

	volDef := p.DeployCtx.Project.Instances[iNickname].Volumes[volNickname]

	if volDef.MountPoint == "" || volDef.Permissions == 0 || volDef.Owner == "" {
		return lb.Complete(fmt.Errorf("empty parameter not allowed: volDef.MountPoint (%s), volDef.Permissions (%d), volDef.Owner (%s)", volDef.MountPoint, volDef.Permissions, volDef.Owner))
	}

	foundVolIdByName, err := cldaws.GetVolumeIdByName(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, volDef.Name)
	if err != nil {
		return lb.Complete(err)
	}

	foundDevice, foundAttachmentState, err := cldaws.GetVolumeAttachedDeviceById(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, foundVolIdByName)
	if err != nil {
		return lb.Complete(err)
	}

	if foundDevice != "" && foundAttachmentState != types.VolumeAttachmentStateAttached {
		return lb.Complete(fmt.Errorf("cannot attach volume %s: it's already attached to device %s, but has invalid attachment state %s", volDef.Name, foundDevice, foundAttachmentState))
	}

	suggestedDevice := volNicknameToAwsSuggestedDeviceName(p.DeployCtx.Project.Instances[iNickname].Volumes, volNickname)

	if foundDevice == "" {
		// Attach
		foundInstanceIdByName, _, err := cldaws.GetInstanceIdAndStateByHostName(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, p.DeployCtx.Project.Instances[iNickname].InstName)
		if err != nil {
			return lb.Complete(err)
		}

		_, err = cldaws.AttachVolume(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, foundVolIdByName, foundInstanceIdByName, suggestedDevice, p.DeployCtx.Project.Timeouts.AttachVolume)
		if err != nil {
			return lb.Complete(err)
		}
	}

	// Mount

	deviceBlockId, er := rexec.ExecSshAndReturnLastLine(
		p.DeployCtx.Project.SshConfig,
		p.DeployCtx.Project.Instances[iNickname].BestIpAddress(),
		fmt.Sprintf("%s\ninit_volume_attachment %s %s %d '%s'",
			cldaws.InitVolumeAttachmentFunc,
			awsFinalDeviceNameNitro(suggestedDevice), // AWS final device here
			volDef.MountPoint,
			volDef.Permissions,
			volDef.Owner))
	lb.Add(er.ToString())
	if er.Error != nil {
		return lb.Complete(fmt.Errorf("cannot mount volume %s to instance %s: %s", volNickname, iNickname, er.Error.Error()))
	}

	if deviceBlockId == "" || strings.HasPrefix(deviceBlockId, "Error") {
		return lb.Complete(fmt.Errorf("cannot mount volume %s to instance %s, returned blockDeviceId is: %s", volNickname, iNickname, deviceBlockId))
	}

	return lb.Complete(nil)
}

func (p *AwsDeployProvider) DetachVolume(iNickname string, volNickname string) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.DeployCtx.IsVerbose)

	volDef := p.DeployCtx.Project.Instances[iNickname].Volumes[volNickname]

	foundVolIdByName, err := cldaws.GetVolumeIdByName(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, volDef.Name)
	if err != nil {
		return lb.Complete(err)
	}

	if foundVolIdByName == "" {
		lb.Add(fmt.Sprintf("volume %s not found, nothing to detach", volDef.Name))
		return lb.Complete(nil)
	}

	foundDevice, _, err := cldaws.GetVolumeAttachedDeviceById(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, foundVolIdByName)
	if err != nil {
		return lb.Complete(err)
	}

	if foundDevice == "" {
		lb.Add(fmt.Sprintf("volume %s not mounted, nothing to detach", volDef.Name))
		return lb.Complete(nil)
	}

	// Unmount

	er := rexec.ExecSsh(
		p.DeployCtx.Project.SshConfig,
		p.DeployCtx.Project.Instances[iNickname].BestIpAddress(),
		fmt.Sprintf("sudo umount -d %s", volDef.MountPoint), map[string]string{})
	lb.Add(er.ToString())
	if er.Error != nil {
		return lb.Complete(fmt.Errorf("cannot umount volume %s on instance %s: %s", volNickname, iNickname, er.Error.Error()))
	}

	foundInstanceIdByName, _, err := cldaws.GetInstanceIdAndStateByHostName(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, p.DeployCtx.Project.Instances[iNickname].InstName)
	if err != nil {
		return lb.Complete(err)
	}

	// Detach

	return lb.Complete(cldaws.DetachVolume(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, foundVolIdByName, foundInstanceIdByName, foundDevice, p.DeployCtx.Project.Timeouts.DetachVolume))
}

func (p *AwsDeployProvider) DeleteVolume(iNickname string, volNickname string) (l.LogMsg, error) {
	lb := l.NewLogBuilder(l.CurFuncName(), p.DeployCtx.IsVerbose)

	volDef := p.DeployCtx.Project.Instances[iNickname].Volumes[volNickname]
	foundVolIdByName, err := cldaws.GetVolumeIdByName(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, volDef.Name)
	if err != nil {
		return lb.Complete(err)
	}

	if foundVolIdByName == "" {
		lb.Add(fmt.Sprintf("volume %s not found, nothing to delete", volDef.Name))
		return lb.Complete(nil)
	}

	return lb.Complete(cldaws.DeleteVolume(p.DeployCtx.Aws.Ec2Client, p.DeployCtx.GoCtx, lb, foundVolIdByName))
}
