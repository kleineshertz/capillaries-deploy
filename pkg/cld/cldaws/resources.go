package cldaws

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	tagging "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	taggingTypes "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"
	"github.com/capillariesio/capillaries-deploy/pkg/cld"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
)

func arnToResource(arn string) cld.Resource {
	r := cld.Resource{
		Svc:         "unknown",
		Type:        "unknown",
		Id:          "unknown",
		State:       "unknown",
		BilledState: cld.ResourceBilledStateUnknown,
	}
	s := strings.Split(arn, "/")
	if len(s) >= 2 {
		r.Id = s[1]
	}
	s = strings.Split(s[0], ":")
	if len(s) >= 3 {
		r.Svc = s[2]
	}
	if len(s) >= 6 {
		r.Type = s[5]
	}
	return r
}

func getInstanceBilledState(state types.InstanceStateName) cld.ResourceBilledState {
	if state == types.InstanceStateNamePending || state == types.InstanceStateNameRunning {
		return cld.ResourceBilledStateActive
	} else {
		return cld.ResourceBilledStateTerminated
	}
}

func getVolumeBilledState(state types.VolumeState) cld.ResourceBilledState {
	if state == types.VolumeStateAvailable || state == types.VolumeStateCreating || state == types.VolumeStateInUse {
		return cld.ResourceBilledStateActive
	} else {
		return cld.ResourceBilledStateTerminated
	}
}

func getNatGatewayBilledState(state types.NatGatewayState) cld.ResourceBilledState {
	if state == types.NatGatewayStatePending || state == types.NatGatewayStateAvailable {
		return cld.ResourceBilledStateActive
	} else {
		return cld.ResourceBilledStateTerminated
	}
}

func getVpcBilledState(state types.VpcState) cld.ResourceBilledState {
	if state == types.VpcStatePending || state == types.VpcStateAvailable {
		return cld.ResourceBilledStateActive
	} else {
		return cld.ResourceBilledStateTerminated
	}
}

func getImageBilledState(state types.ImageState) cld.ResourceBilledState {
	if state == types.ImageStateAvailable || state == types.ImageStateDisabled || state == types.ImageStateError || state == types.ImageStatePending || state == types.ImageStateTransient {
		return cld.ResourceBilledStateActive
	} else {
		return cld.ResourceBilledStateTerminated
	}
}
func getSnapshotBilledState(_ types.SnapshotState) cld.ResourceBilledState {
	return cld.ResourceBilledStateActive
}

func getResourceState(ec2Client *ec2.Client, goCtx context.Context, r *cld.Resource) (string, cld.ResourceBilledState, error) {
	switch r.Svc {
	case "ec2":
		switch r.Type {
		case "elastic-ip":
			out, err := ec2Client.DescribeAddresses(goCtx, &ec2.DescribeAddressesInput{AllocationIds: []string{r.Id}})
			if err != nil {
				return "", "", err
			}
			return *out.Addresses[0].PublicIp, cld.ResourceBilledStateActive, nil
		case "vpc":
			out, err := ec2Client.DescribeVpcs(goCtx, &ec2.DescribeVpcsInput{VpcIds: []string{r.Id}})
			if err != nil {
				return "", "", err
			}
			return string(out.Vpcs[0].State), getVpcBilledState(out.Vpcs[0].State), nil
		case "subnet":
			out, err := ec2Client.DescribeSubnets(goCtx, &ec2.DescribeSubnetsInput{SubnetIds: []string{r.Id}})
			if err != nil {
				return "", "", err
			}
			return string(out.Subnets[0].State), cld.ResourceBilledStateActive, nil
		case "security-group":
			_, err := ec2Client.DescribeSecurityGroups(goCtx, &ec2.DescribeSecurityGroupsInput{GroupIds: []string{r.Id}})
			if err != nil {
				return "", "", err
			}
			return "present", cld.ResourceBilledStateActive, nil
		case "route-table":
			out, err := ec2Client.DescribeRouteTables(goCtx, &ec2.DescribeRouteTablesInput{RouteTableIds: []string{r.Id}})
			if err != nil {
				if strings.Contains(err.Error(), "does not exist") {
					return "doesnotexist", cld.ResourceBilledStateTerminated, nil
				}
				return "", "", err
			}
			return fmt.Sprintf("%droutes", len(out.RouteTables[0].Routes)), cld.ResourceBilledStateActive, nil
		case "instance":
			out, err := ec2Client.DescribeInstances(goCtx, &ec2.DescribeInstancesInput{InstanceIds: []string{r.Id}})
			if err != nil {
				return "", "", err
			}
			if len(out.Reservations) == 0 || len(out.Reservations[0].Instances) == 0 {
				return "notfound", cld.ResourceBilledStateTerminated, nil
			}
			return string(out.Reservations[0].Instances[0].State.Name), getInstanceBilledState(out.Reservations[0].Instances[0].State.Name), nil
		case "volume":
			out, err := ec2Client.DescribeVolumes(goCtx, &ec2.DescribeVolumesInput{VolumeIds: []string{r.Id}})
			if err != nil {
				if strings.Contains(err.Error(), "does not exist") {
					return "doesnotexist", cld.ResourceBilledStateTerminated, nil
				}
				return "", "", err
			}
			return string(out.Volumes[0].State), getVolumeBilledState(out.Volumes[0].State), nil
		case "natgateway":
			out, err := ec2Client.DescribeNatGateways(goCtx, &ec2.DescribeNatGatewaysInput{NatGatewayIds: []string{r.Id}})
			if err != nil {
				if strings.Contains(err.Error(), "was not found") {
					return "notfound", cld.ResourceBilledStateTerminated, nil
				}
				return "", "", err
			}
			return string(out.NatGateways[0].State), getNatGatewayBilledState(out.NatGateways[0].State), nil
		case "internet-gateway":
			out, err := ec2Client.DescribeInternetGateways(goCtx, &ec2.DescribeInternetGatewaysInput{InternetGatewayIds: []string{r.Id}})
			if err != nil {
				if strings.Contains(err.Error(), "does not exist") {
					return "doesnotexist", cld.ResourceBilledStateTerminated, nil
				}
				return "", "", err
			}
			return fmt.Sprintf("%dattachments", len(out.InternetGateways[0].Attachments)), cld.ResourceBilledStateActive, nil
		case "image":
			out, err := ec2Client.DescribeImages(goCtx, &ec2.DescribeImagesInput{ImageIds: []string{r.Id}})
			if err != nil {
				if strings.Contains(err.Error(), "does not exist") {
					return "doesnotexist", cld.ResourceBilledStateTerminated, nil
				}
				return "", "", err
			}
			return string(out.Images[0].State), getImageBilledState(out.Images[0].State), nil

		case "snapshot":
			out, err := ec2Client.DescribeSnapshots(goCtx, &ec2.DescribeSnapshotsInput{SnapshotIds: []string{r.Id}})
			if err != nil {
				if strings.Contains(err.Error(), "does not exist") {
					return "doesnotexist", cld.ResourceBilledStateTerminated, nil
				}
				return "", "", err
			}
			return string(out.Snapshots[0].State), getSnapshotBilledState(out.Snapshots[0].State), nil
		default:
			return "", "", fmt.Errorf("unsupported ec2 type %s", r.Type)
		}
	default:
		return "", "", fmt.Errorf("unsupported svc %s", r.Svc)
	}
}

func getResourceDeploymentNameAndNameTags(ec2Client *ec2.Client, goCtx context.Context, resourceId string) (string, string, error) {
	out, err := ec2Client.DescribeTags(goCtx, &ec2.DescribeTagsInput{Filters: []types.Filter{{
		Name: aws.String("resource-id"), Values: []string{resourceId}}}})
	if err != nil {
		return "", "", err
	}
	deploymentNameTagValue := ""
	resourceNameTagValue := ""
	for _, tagDesc := range out.Tags {
		if *tagDesc.Key == "Name" {
			resourceNameTagValue = *tagDesc.Value
		} else if *tagDesc.Key == cld.DeploymentNameTagName {
			deploymentNameTagValue = *tagDesc.Value
		}
	}
	return deploymentNameTagValue, resourceNameTagValue, nil
}

func GetResourcesByTag(tClient *tagging.Client, ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, region string, tagFilters []taggingTypes.TagFilter, readState bool) ([]*cld.Resource, error) {
	resources := make([]*cld.Resource, 0)
	paginationToken := ""
	for {
		out, err := tClient.GetResources(goCtx, &tagging.GetResourcesInput{
			ResourcesPerPage: aws.Int32(100),
			PaginationToken:  &paginationToken,
			TagFilters:       tagFilters})
		if err != nil {
			return []*cld.Resource{}, err
		}

		for _, rtMapping := range out.ResourceTagMappingList {
			res := arnToResource(*rtMapping.ResourceARN)
			if readState {
				state, billedState, err := getResourceState(ec2Client, goCtx, &res)
				if err != nil {
					lb.Add(err.Error())
				} else {
					res.State = state
					res.BilledState = billedState
				}
			}
			deploymentName, resourceName, err := getResourceDeploymentNameAndNameTags(ec2Client, goCtx, res.Id)
			if err != nil {
				lb.Add(err.Error())
			} else {
				res.DeploymentName = deploymentName
				res.Name = resourceName
			}
			resources = append(resources, &res)
		}
		paginationToken = *out.PaginationToken
		if *out.PaginationToken == "" {
			break
		}
	}

	sort.Slice(resources, func(i, j int) bool {
		if resources[i].DeploymentName < resources[j].DeploymentName {
			return true
		} else if resources[i].DeploymentName > resources[j].DeploymentName {
			return false
		} else if resources[i].Svc < resources[j].Svc {
			return true
		} else if resources[i].Svc > resources[j].Svc {
			return false
		} else if resources[i].Type < resources[j].Type {
			return true
		} else if resources[i].Type > resources[j].Type {
			return false
		} else if resources[i].Name < resources[j].Name {
			return true
		} else if resources[i].Name > resources[j].Name {
			return false
		} else if resources[i].Id < resources[j].Id {
			return true
		} else if resources[i].Id > resources[j].Id {
			return false
		} else {
			return true
		}
	})

	return resources, nil
}
