package amazon

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/docker/ecs-plugin/pkg/compose"
)

const (
	ProjectTag = "com.docker.compose.project"
	ServiceTag = "com.docker.compose.service"
	NetworkTag = "com.docker.compose.network"
)

func NewClient(profile string, cluster string, region string) (compose.API, error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		Profile: profile,
		Config: aws.Config{
			Region: aws.String(region),
		},
	})
	if err != nil {
		return nil, err
	}
	return &client{
		Cluster: cluster,
		Region:  region,
		api:     NewAPI(sess),
	}, nil
}

type client struct {
	Cluster string
	Region  string
	api     API
}

var _ compose.API = &client{}
