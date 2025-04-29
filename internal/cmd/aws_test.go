package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
)

func TestNewRootAwsCommand(t *testing.T) {
	cmds := AwsCommandParams{
		Commands: []*cli.Command{
			{Name: "s3"},
			{Name: "ec2"},
		},
	}
	cmd := NewRootAwsCommand(cmds).Command

	assert.Equal(t, "aws", cmd.Name)
	assert.Equal(t, "AWS subcommands", cmd.Usage)
	assert.True(t, cmd.Hidden)
	assert.Len(t, cmd.Commands, 2)
	assert.Equal(t, "ec2", cmd.Commands[0].Name)
	assert.Equal(t, "s3", cmd.Commands[1].Name)
}
