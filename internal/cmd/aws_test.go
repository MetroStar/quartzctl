// Copyright 2025 Metrostar Systems, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
