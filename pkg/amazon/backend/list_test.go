package backend

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestParseTargetGroup(t *testing.T) {
	pb, err := parseTargetGroup("FrontTCP80TargetGroup")
	assert.NilError(t, err)
	assert.Equal(t, "front", pb.ServiceName)
	assert.Equal(t, "80", pb.Port)
	assert.Equal(t, "tcp", pb.Protocol)
}

func TestParseInvalidTargetGroup(t *testing.T) {
	_, err := parseTargetGroup("Invalid")
	assert.Error(t, err, "malformed target group ID \"Invalid\"")
}
