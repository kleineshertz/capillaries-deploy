package provider

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/capillariesio/capillaries-deploy/pkg/cld"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
	"github.com/capillariesio/capillaries-deploy/pkg/prj"
	"github.com/capillariesio/capillaries-deploy/pkg/rexec"
)

type deployProviderImpl interface {
	getDeployCtx() *DeployCtx
	listDeployments() (map[string]int, l.LogMsg, error)
	listDeploymentResources() ([]*cld.Resource, l.LogMsg, error)
	CreateFloatingIps() (l.LogMsg, error)
	DeleteFloatingIps() (l.LogMsg, error)
	CreateSecurityGroups() (l.LogMsg, error)
	DeleteSecurityGroups() (l.LogMsg, error)
	CreateNetworking() (l.LogMsg, error)
	DeleteNetworking() (l.LogMsg, error)
	HarvestInstanceTypesByFlavorNames(flavorMap map[string]string) (l.LogMsg, error)
	HarvestImageIds(imageMap map[string]bool) (l.LogMsg, error)
	VerifyKeypairs(keypairMap map[string]struct{}) (l.LogMsg, error)
	CreateInstanceAndWaitForCompletion(iNickname string, flavorId string, imageId string) (l.LogMsg, error)
	DeleteInstance(iNickname string, ignoreAttachedVolumes bool) (l.LogMsg, error)
	CreateSnapshotImage(iNickname string) (l.LogMsg, error)
	CreateInstanceFromSnapshotImageAndWaitForCompletion(iNickname string, flavorId string) (l.LogMsg, error)
	DeleteSnapshotImage(iNickname string) (l.LogMsg, error)
	CreateVolume(iNickname string, volNickname string) (l.LogMsg, error)
	AttachVolume(iNickname string, volNickname string) (l.LogMsg, error)
	DetachVolume(iNickname string, volNickname string) (l.LogMsg, error)
	DeleteVolume(iNickname string, volNickname string) (l.LogMsg, error)
	PopulateInstanceExternalAddressByName() (l.LogMsg, error)
	CheckCassStatus() (l.LogMsg, error)
}

func isAllNodesJoined(strOut string, instances map[string]*prj.InstanceDef) error {
	missingIps := make([]string, 0)
	for _, iDef := range instances {
		if iDef.Purpose == string(prj.InstancePurposeCassandra) {
			re := regexp.MustCompile(`UN  ` + iDef.IpAddress)
			matches := re.FindAllString(strOut, -1)
			if len(matches) == 0 {
				missingIps = append(missingIps, iDef.IpAddress)
			}
		}
	}
	if len(missingIps) > 0 {
		return fmt.Errorf("nodes did not join cassandra cluster: %s", strings.Join(missingIps, ","))
	}
	return nil
}

func (p *AwsDeployProvider) CheckCassStatus() (l.LogMsg, error) {
	for _, iDef := range p.DeployCtx.Project.Instances {
		if iDef.Purpose == string(prj.InstancePurposeCassandra) {
			logMsg, err := rexec.ExecCommandOnInstance(p.DeployCtx.Project.SshConfig, iDef.IpAddress, "nodetool describecluster;nodetool status", true)
			if err == nil {
				// All Cassandra nodes must have "UN  $cassNodeIp"
				err = isAllNodesJoined(string(logMsg), p.DeployCtx.Project.Instances)
			}
			if p.DeployCtx.IsVerbose {
				return logMsg, err
			} else {
				return "", err
			}
		}
	}

	return "", fmt.Errorf("cannot find even a single cassandra node")
}
