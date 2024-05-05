package cldaws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	cc "github.com/aws/aws-sdk-go-v2/service/cloudcontrol"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	tagging "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	taggingTypes "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
)

type BilledState string

const (
	BilledStateUnknown  BilledState = "unknown"
	BilledStateBilled   BilledState = "billed"
	BilledStateUnbilled BilledState = "unbilled"
)

type Resource struct {
	Svc    string
	Type   string
	Id     string
	State  string
	Billed BilledState
}

func (r *Resource) String() string {
	return fmt.Sprintf("%s,%s,%s,%s,%s", r.Svc, r.Type, r.Id, r.State, r.Billed)
}

func arnToResource(arn string) Resource {
	r := Resource{
		Svc:    "unknown",
		Type:   "unknown",
		Id:     "unknown",
		State:  "unknown",
		Billed: BilledStateUnknown,
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

func getInstanceBilledState(state types.InstanceStateName) BilledState {
	if state == types.InstanceStateNamePending || state == types.InstanceStateNameRunning {
		return BilledStateBilled
	} else {
		return BilledStateUnbilled
	}
}

func getVolumeBilledState(state types.VolumeState) BilledState {
	if state == types.VolumeStateAvailable || state == types.VolumeStateCreating || state == types.VolumeStateInUse {
		return BilledStateBilled
	} else {
		return BilledStateUnbilled
	}
}

func getNatGatewayBilledState(state types.NatGatewayState) BilledState {
	if state == types.NatGatewayStatePending || state == types.NatGatewayStateAvailable {
		return BilledStateBilled
	} else {
		return BilledStateUnbilled
	}
}

func getVpcBilledState(state types.VpcState) BilledState {
	if state == types.VpcStatePending || state == types.VpcStateAvailable {
		return BilledStateBilled
	} else {
		return BilledStateUnbilled
	}
}

func getResourceState(ec2Client *ec2.Client, goCtx context.Context, r *Resource) (string, BilledState, error) {
	switch r.Svc {
	case "ec2":
		switch r.Type {
		case "elastic-ip":
			out, err := ec2Client.DescribeAddresses(goCtx, &ec2.DescribeAddressesInput{AllocationIds: []string{r.Id}})
			if err != nil {
				return "", "", err
			}
			return *out.Addresses[0].PublicIp, BilledStateBilled, nil
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
			return string(out.Subnets[0].State), BilledStateBilled, nil
		case "security-group":
			out, err := ec2Client.DescribeSecurityGroups(goCtx, &ec2.DescribeSecurityGroupsInput{GroupIds: []string{r.Id}})
			if err != nil {
				return "", "", err
			}
			return string(*out.SecurityGroups[0].GroupName), BilledStateBilled, nil
		case "route-table":
			out, err := ec2Client.DescribeRouteTables(goCtx, &ec2.DescribeRouteTablesInput{RouteTableIds: []string{r.Id}})
			if err != nil {
				if strings.Contains(err.Error(), "does not exist") {
					return "doesnotexist", BilledStateUnbilled, nil
				}
				return "", "", err
			}
			return fmt.Sprintf("%d", len(out.RouteTables[0].Routes)), BilledStateBilled, nil
		case "instance":
			out, err := ec2Client.DescribeInstances(goCtx, &ec2.DescribeInstancesInput{InstanceIds: []string{r.Id}})
			if err != nil {
				return "", "", err
			}
			return string(out.Reservations[0].Instances[0].State.Name), getInstanceBilledState(out.Reservations[0].Instances[0].State.Name), nil
		case "volume":
			out, err := ec2Client.DescribeVolumes(goCtx, &ec2.DescribeVolumesInput{VolumeIds: []string{r.Id}})
			if err != nil {
				if strings.Contains(err.Error(), "does not exist") {
					return "doesnotexist", BilledStateUnbilled, nil
				}
				return "", "", err
			}
			return string(out.Volumes[0].State), getVolumeBilledState(out.Volumes[0].State), nil
		case "natgateway":
			out, err := ec2Client.DescribeNatGateways(goCtx, &ec2.DescribeNatGatewaysInput{NatGatewayIds: []string{r.Id}})
			if err != nil {
				if strings.Contains(err.Error(), "was not found") {
					return "notfound", BilledStateUnbilled, nil
				}
				return "", "", err
			}
			return string(out.NatGateways[0].State), getNatGatewayBilledState(out.NatGateways[0].State), nil
		case "internet-gateway":
			out, err := ec2Client.DescribeInternetGateways(goCtx, &ec2.DescribeInternetGatewaysInput{InternetGatewayIds: []string{r.Id}})
			if err != nil {
				return "", "", err
			}
			return string(len(out.InternetGateways[0].Attachments)), BilledStateBilled, nil
		default:
			return "", "", fmt.Errorf("unsupported ec2 type %s", r.Type)
		}
	default:
		return "", "", fmt.Errorf("unsupported svc %s", r.Svc)
	}
}

func GetResourcesByTag(tClient *tagging.Client, ccClient *cc.Client, ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, region string, tagName string, tagVal string) ([]string, error) {
	resources := make([]string, 0)
	paginationToken := ""
	for {
		out, err := tClient.GetResources(goCtx, &tagging.GetResourcesInput{
			ResourcesPerPage: aws.Int32(100),
			PaginationToken:  &paginationToken,
			TagFilters:       []taggingTypes.TagFilter{{Key: aws.String(tagName), Values: []string{tagVal}}}})
		if err != nil {
			return []string{}, err
		}

		for _, rtMapping := range out.ResourceTagMappingList {
			res := arnToResource(*rtMapping.ResourceARN)
			state, billedState, err := getResourceState(ec2Client, goCtx, &res)
			if err != nil {
				lb.Add(err.Error())
			} else {
				res.State = state
				res.Billed = billedState
			}
			resources = append(resources, res.String())
		}
		paginationToken = *out.PaginationToken
		if *out.PaginationToken == "" {
			break
		}
	}
	return resources, nil
}
