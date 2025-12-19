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
	"os/exec"
	"strings"
	"time"

	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/MetroStar/quartzctl/internal/util"
)

// ForceAWSCleanup runs AWS CLI commands to forcibly clean up resources that may block Terraform destroy.
// This includes detaching/deleting ENIs and removing security groups that may have lingering dependencies.
//
// Parameters:
//   - ctx: The context for the operation.
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - error: An error if the cleanup fails, otherwise nil.
func ForceAWSCleanup(ctx context.Context, p *CommandParams) error {
	log.Debug("Entering", "command", "force-aws-cleanup")
	defer log.Debug("Completed", "command", "force-aws-cleanup")

	cfg := p.Settings().Config
	clusterName := cfg.Name
	region := cfg.Aws.Region

	if clusterName == "" || region == "" {
		return fmt.Errorf("cluster name and region are required for AWS cleanup")
	}

	util.Msgf("Starting force AWS cleanup for cluster: %s in region: %s", clusterName, region)

	startTime := time.Now()

	// Phase 1: Delete LoadBalancers
	util.Msg("Phase 1: Cleaning up LoadBalancers...")
	if err := cleanupLoadBalancers(ctx, clusterName, region); err != nil {
		log.Warn("Error cleaning up LoadBalancers", "error", err)
	}
	util.Msgf("  LoadBalancer cleanup completed in %v", time.Since(startTime))

	// Phase 2: Detach and delete ENIs
	util.Msg("Phase 2: Cleaning up ENIs...")
	eniStart := time.Now()
	if err := cleanupENIs(ctx, clusterName, region); err != nil {
		log.Warn("Error cleaning up ENIs", "error", err)
	}
	util.Msgf("  ENI cleanup completed in %v", time.Since(eniStart))

	// Phase 3: Delete Security Groups
	util.Msg("Phase 3: Cleaning up Security Groups...")
	sgStart := time.Now()
	if err := cleanupSecurityGroups(ctx, clusterName, region); err != nil {
		log.Warn("Error cleaning up Security Groups", "error", err)
	}
	util.Msgf("  Security Group cleanup completed in %v", time.Since(sgStart))

	util.Msgf("Force AWS cleanup completed in %v", time.Since(startTime))
	return nil
}

// cleanupLoadBalancers deletes all ELBv2 load balancers tagged with the cluster name.
func cleanupLoadBalancers(ctx context.Context, clusterName, region string) error {
	// Get all LB ARNs
	cmd := exec.CommandContext(ctx, "aws", "elbv2", "describe-load-balancers",
		"--region", region,
		"--query", "LoadBalancers[].LoadBalancerArn",
		"--output", "text")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list load balancers: %w", err)
	}

	lbArns := strings.Fields(string(output))
	if len(lbArns) == 0 {
		util.Msg("  No LoadBalancers found")
		return nil
	}

	// Check each LB for cluster tag
	for _, arn := range lbArns {
		if arn == "" || arn == "None" {
			continue
		}

		// Get tags for this LB
		tagCmd := exec.CommandContext(ctx, "aws", "elbv2", "describe-tags",
			"--region", region,
			"--resource-arns", arn,
			"--query", "TagDescriptions[0].Tags[?Key=='elbv2.k8s.aws/cluster'].Value",
			"--output", "text")
		tagOutput, err := tagCmd.Output()
		if err != nil {
			log.Warn("Failed to get tags for LB", "arn", arn, "error", err)
			continue
		}

		if strings.TrimSpace(string(tagOutput)) == clusterName {
			util.Msgf("  Deleting LoadBalancer: %s", arn)
			deleteCmd := exec.CommandContext(ctx, "aws", "elbv2", "delete-load-balancer",
				"--region", region,
				"--load-balancer-arn", arn)
			if err := deleteCmd.Run(); err != nil {
				log.Warn("Failed to delete LB", "arn", arn, "error", err)
			}
		}
	}

	// Wait for LBs to be deleted
	util.Msg("  Waiting for LoadBalancers to be deleted...")
	for i := 0; i < 12; i++ { // 2 minutes max
		time.Sleep(10 * time.Second)

		remaining := 0
		for _, arn := range lbArns {
			tagCmd := exec.CommandContext(ctx, "aws", "elbv2", "describe-tags",
				"--region", region,
				"--resource-arns", arn,
				"--query", "TagDescriptions[0].Tags[?Key=='elbv2.k8s.aws/cluster'].Value",
				"--output", "text")
			tagOutput, _ := tagCmd.Output()
			if strings.TrimSpace(string(tagOutput)) == clusterName {
				remaining++
			}
		}

		if remaining == 0 {
			util.Msg("  ✅ All LoadBalancers deleted")
			return nil
		}
	}

	return nil
}

// cleanupENIs detaches and deletes ENIs associated with the cluster.
func cleanupENIs(ctx context.Context, clusterName, region string) error {
	// Find ENIs with cluster tag
	cmd := exec.CommandContext(ctx, "aws", "ec2", "describe-network-interfaces",
		"--region", region,
		"--filters", fmt.Sprintf("Name=tag:kubernetes.io/cluster/%s,Values=owned,shared", clusterName),
		"--query", "NetworkInterfaces[?Status=='in-use'].[NetworkInterfaceId,Attachment.AttachmentId,RequesterManaged]",
		"--output", "text")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list ENIs: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		util.Msg("  No in-use ENIs found")
		return nil
	}

	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		eniId := parts[0]
		attachmentId := parts[1]
		requesterManaged := parts[2]

		if eniId == "" || eniId == "None" {
			continue
		}

		if requesterManaged == "true" || requesterManaged == "True" {
			util.Msgf("  Skipping requester-managed ENI: %s", eniId)
			continue
		}

		if attachmentId != "" && attachmentId != "None" {
			util.Msgf("  Force detaching ENI: %s", eniId)
			detachCmd := exec.CommandContext(ctx, "aws", "ec2", "detach-network-interface",
				"--region", region,
				"--force",
				"--attachment-id", attachmentId)
			if err := detachCmd.Run(); err != nil {
				log.Warn("Failed to detach ENI", "eni", eniId, "error", err)
			}
		}
	}

	// Wait for ENIs to become available
	time.Sleep(15 * time.Second)

	// Delete available ENIs
	availableCmd := exec.CommandContext(ctx, "aws", "ec2", "describe-network-interfaces",
		"--region", region,
		"--filters",
		fmt.Sprintf("Name=tag:kubernetes.io/cluster/%s,Values=owned,shared", clusterName),
		"Name=status,Values=available",
		"--query", "NetworkInterfaces[].NetworkInterfaceId",
		"--output", "text")
	availableOutput, err := availableCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list available ENIs: %w", err)
	}

	eniIds := strings.Fields(string(availableOutput))
	deletedCount := 0
	for _, eniId := range eniIds {
		if eniId == "" || eniId == "None" {
			continue
		}

		deleteCmd := exec.CommandContext(ctx, "aws", "ec2", "delete-network-interface",
			"--region", region,
			"--network-interface-id", eniId)
		if err := deleteCmd.Run(); err != nil {
			log.Warn("Failed to delete ENI", "eni", eniId, "error", err)
		} else {
			deletedCount++
		}
	}

	util.Msgf("  ✅ Deleted %d ENI(s)", deletedCount)
	return nil
}

// cleanupSecurityGroups removes rules and deletes security groups associated with the cluster.
func cleanupSecurityGroups(ctx context.Context, clusterName, region string) error {
	// Find security groups with cluster tags
	cmd := exec.CommandContext(ctx, "aws", "ec2", "describe-security-groups",
		"--region", region,
		"--filters",
		fmt.Sprintf("Name=tag:kubernetes.io/cluster/%s,Values=owned", clusterName),
		"--query", "SecurityGroups[].GroupId",
		"--output", "text")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list security groups: %w", err)
	}

	k8sSgs := strings.Fields(string(output))

	// Also find LB-specific security groups
	lbCmd := exec.CommandContext(ctx, "aws", "ec2", "describe-security-groups",
		"--region", region,
		"--filters",
		fmt.Sprintf("Name=tag:elbv2.k8s.aws/cluster,Values=%s", clusterName),
		"--query", "SecurityGroups[].GroupId",
		"--output", "text")
	lbOutput, _ := lbCmd.Output()
	lbSgs := strings.Fields(string(lbOutput))

	// Also find backend SG by name
	backendCmd := exec.CommandContext(ctx, "aws", "ec2", "describe-security-groups",
		"--region", region,
		"--filters",
		fmt.Sprintf("Name=group-name,Values=%s-elb-backend", clusterName),
		"--query", "SecurityGroups[].GroupId",
		"--output", "text")
	backendOutput, _ := backendCmd.Output()
	backendSgs := strings.Fields(string(backendOutput))

	// Combine and dedupe
	sgSet := make(map[string]bool)
	for _, sg := range k8sSgs {
		if sg != "" && sg != "None" {
			sgSet[sg] = true
		}
	}
	for _, sg := range lbSgs {
		if sg != "" && sg != "None" {
			sgSet[sg] = true
		}
	}
	for _, sg := range backendSgs {
		if sg != "" && sg != "None" {
			sgSet[sg] = true
		}
	}

	if len(sgSet) == 0 {
		util.Msg("  No security groups found")
		return nil
	}

	allSgs := make([]string, 0, len(sgSet))
	for sg := range sgSet {
		allSgs = append(allSgs, sg)
	}

	util.Msgf("  Found %d security group(s) to clean up", len(allSgs))

	// Revoke ingress and egress rules referencing these SGs
	for _, sg := range allSgs {
		// Revoke all ingress rules
		revokeIngressCmd := exec.CommandContext(ctx, "aws", "ec2", "describe-security-groups",
			"--region", region,
			"--group-ids", sg,
			"--query", "SecurityGroups[0].IpPermissions",
			"--output", "json")
		ingressRules, _ := revokeIngressCmd.Output()
		if len(ingressRules) > 2 && string(ingressRules) != "[]" && string(ingressRules) != "null" {
			revokeCmd := exec.CommandContext(ctx, "aws", "ec2", "revoke-security-group-ingress",
				"--region", region,
				"--group-id", sg,
				"--ip-permissions", string(ingressRules))
			revokeCmd.Run() // Ignore errors
		}

		// Revoke all egress rules (except default)
		revokeEgressCmd := exec.CommandContext(ctx, "aws", "ec2", "describe-security-groups",
			"--region", region,
			"--group-ids", sg,
			"--query", "SecurityGroups[0].IpPermissionsEgress",
			"--output", "json")
		egressRules, _ := revokeEgressCmd.Output()
		if len(egressRules) > 2 && string(egressRules) != "[]" && string(egressRules) != "null" {
			revokeCmd := exec.CommandContext(ctx, "aws", "ec2", "revoke-security-group-egress",
				"--region", region,
				"--group-id", sg,
				"--ip-permissions", string(egressRules))
			revokeCmd.Run() // Ignore errors
		}
	}

	time.Sleep(5 * time.Second)

	// Delete security groups
	deletedCount := 0
	for _, sg := range allSgs {
		deleteCmd := exec.CommandContext(ctx, "aws", "ec2", "delete-security-group",
			"--region", region,
			"--group-id", sg)
		if err := deleteCmd.Run(); err != nil {
			log.Warn("Failed to delete security group", "sg", sg, "error", err)
		} else {
			deletedCount++
		}
	}

	util.Msgf("  ✅ Deleted %d security group(s)", deletedCount)
	return nil
}
