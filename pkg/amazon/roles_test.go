package amazon

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/compose-spec/compose-go/types"
	"github.com/docker/ecs-plugin/pkg/amazon/mock"

	"github.com/golang/mock/gomock"
	"gotest.tools/v3/assert"
	"testing"
)

// role set by x-aws-TaskExecutionRole should always be preferred
func Test_GetEcsTaskExecutionRole_extension(t *testing.T) {
	c := client{}
	got, err := c.GetEcsTaskExecutionRole(types.ServiceConfig{
		Extras: map[string]interface{}{
			"x-aws-TaskExecutionRole": "123",
		},
	})
	assert.NilError(t, err)
	assert.Equal(t, *got, "123")
}

func Test_GetEcsTaskExecutionRole_iampolicy(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	iammock := mock.NewMockIAMAPI(ctrl)
	expect := iammock.EXPECT()
	expect.ListEntitiesForPolicy(gomock.Any()).Return(&iam.ListEntitiesForPolicyOutput{
		PolicyRoles: []*iam.PolicyRole{
			{
				RoleName: aws.String("123"),
			},
		},
	}, nil).Times(1)
	expect.GetRole(gomock.Eq(&iam.GetRoleInput{
		RoleName: aws.String("123"),
	})).Return(&iam.GetRoleOutput{
		Role: &iam.Role{
			Arn: aws.String("Arn:123"),
		},
	}, nil).Times(1)

	c := client{
		IAM: iammock,
	}
	got, err := c.GetEcsTaskExecutionRole(types.ServiceConfig{})
	assert.NilError(t, err)
	assert.Equal(t, *got, "Arn:123")
}