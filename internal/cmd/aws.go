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
	"context"
	"fmt"
	"slices"

	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/MetroStar/quartzctl/internal/provider"
	"github.com/urfave/cli/v3"
)

// NewRootAwsCommand creates the root AWS CLI command.
// It organizes and returns all AWS-related subcommands.
//
// Parameters:
//   - cmds: AwsCommandParams containing the list of AWS subcommands.
//
// Returns:
//   - RootCommandResult containing the root AWS CLI command.
func NewRootAwsCommand(cmds AwsCommandParams) RootCommandResult {
	slices.SortFunc(cmds.Commands, ByCommandName)
	return RootCommandResult{
		Command: &cli.Command{
			Name:     "aws",
			Usage:    "AWS subcommands",
			Hidden:   true,
			Commands: cmds.Commands,
		},
	}
}

// NewGetEksTokenCommand creates a CLI command for retrieving an EKS authentication token.
//
// Returns:
//   - AwsCommandResult containing the CLI command for retrieving the token.
func NewGetEksTokenCommand() AwsCommandResult {
	return AwsCommandResult{
		Command: &cli.Command{
			Name:  "get-eks-token",
			Usage: "Retrieve an authentication token for an EKS cluster",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "cluster", Usage: "EKS cluster name", Required: true},
				&cli.StringFlag{Name: "region", Usage: "AWS region", Required: true},
			},
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				cluster := ccmd.String("cluster")
				region := ccmd.String("region")
				return AwsGetEksToken(ctx, cluster, region)
			},
		},
	}
}

// AwsGetEksToken retrieves an authentication token for an EKS cluster.
// This function is used by kubeconfig exec to request the token, similar to `aws eks get-token`.
//
// Parameters:
//   - ctx: The context for the operation.
//   - name: The name of the EKS cluster.
//   - region: The AWS region where the EKS cluster is located.
//
// Returns:
//   - error: An error if the token retrieval fails, otherwise nil.
func AwsGetEksToken(ctx context.Context, name string, region string) error {
	log.Debug("Entering", "command", "aws:get-eks-token")
	defer log.Debug("Completed", "command", "aws:get-eks-token")

	aws, err := provider.NewLazyAwsClient(ctx, name, region)
	if err != nil {
		return err
	}

	_, token, err := aws.EksKubeconfigInfo(ctx)
	if err != nil {
		return err
	}

	fmt.Println(token.JsonString)
	return nil
}
