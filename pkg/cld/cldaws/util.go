package cldaws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
)

func TagResource(ec2Client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, resourceId string, tagName string, tagMap map[string]string) error {
	out, err := ec2Client.CreateTags(goCtx, &ec2.CreateTagsInput{
		Resources: []string{resourceId},
		Tags:      mapToTags(tagName, tagMap)})
	lb.AddObject(fmt.Sprintf("CreateTags(resources=%s,tag:Name=%s)", resourceId, tagName), out)
	if err != nil {
		return fmt.Errorf("cannot tag resource %s: %s", resourceId, err.Error())
	}
	return nil
}

func mapToTags(tagName string, tagMap map[string]string) []types.Tag {
	result := make([]types.Tag, len(tagMap))
	if tagMap != nil {
		tagIdx := 0
		for tagName, tagVal := range tagMap {
			result[tagIdx] = types.Tag{Key: aws.String(tagName), Value: aws.String(tagVal)}
			tagIdx++
		}
	}
	if tagName != "" {
		result = append(result, types.Tag{Key: aws.String("Name"), Value: aws.String(tagName)})
	}
	return result
}
