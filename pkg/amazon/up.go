package amazon

import (
	"context"
	"fmt"

	cf "github.com/aws/aws-sdk-go/service/cloudformation"

	"github.com/awslabs/goformation/v4/cloudformation"
	"github.com/docker/ecs-plugin/pkg/compose"
)

func (c *client) ComposeUp(ctx context.Context, project *compose.Project) error {
	ok, err := c.api.ClusterExists(ctx, c.Cluster)
	if err != nil {
		return err
	}
	if !ok {
		c.api.CreateCluster(ctx, c.Cluster)
	}
	update, err := c.api.StackExists(ctx, project.Name)
	if err != nil {
		return err
	}
	if update {
		return fmt.Errorf("we do not (yet) support updating an existing CloudFormation stack")
	}

	template, err := c.Convert(ctx, project)
	if err != nil {
		return err
	}

	err = c.api.CreateStack(ctx, project.Name, template)
	if err != nil {
		return err
	}

	known := map[string]struct{}{}
	err = c.api.WaitStackComplete(ctx, project.Name, func() error {
		events, err := c.api.DescribeStackEvents(ctx, project.Name)
		if err != nil {
			return err
		}
		for _, event := range events {
			if _, ok := known[*event.EventId]; ok {
				continue
			}
			known[*event.EventId] = struct{}{}

			description := "-"
			if event.ResourceStatusReason != nil {
				description = *event.ResourceStatusReason
			}
			fmt.Printf("%s %q %s %s\n", *event.ResourceType, *event.LogicalResourceId, *event.ResourceStatus, description)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// TODO monitor progress
	return nil
}

type upAPI interface {
	ClusterExists(ctx context.Context, name string) (bool, error)
	CreateCluster(ctx context.Context, name string) (string, error)
	StackExists(ctx context.Context, name string) (bool, error)
	CreateStack(ctx context.Context, name string, template *cloudformation.Template) error
	WaitStackComplete(ctx context.Context, name string, fn func() error) error
	DescribeStackEvents(ctx context.Context, stack string) ([]*cf.StackEvent, error)
}
