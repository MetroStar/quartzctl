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
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/MetroStar/quartzctl/internal/util"
)

// awsNoneValue is the string AWS CLI returns for empty/null query results
const awsNoneValue = "None"

// HasBlockingAWSResources performs a quick check to detect resources that would block
// Terraform destroy (orphaned EC2 instances, in-use ENIs). This is a fast check
// (~2-3 seconds) that allows us to proactively run cleanup instead of waiting
// 15+ minutes for Terraform to timeout.
//
// Parameters:
//   - ctx: The context for the operation.
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - bool: true if blocking resources were found, false otherwise.
//   - error: An error if the check fails.
func HasBlockingAWSResources(ctx context.Context, p *CommandParams) (bool, error) {
	cfg := p.Settings().Config
	clusterName := cfg.Name
	region := cfg.Aws.Region

	if clusterName == "" || region == "" {
		return false, fmt.Errorf("cluster name and region are required")
	}

	// Check for running EC2 instances tagged with this cluster
	cmd := exec.CommandContext(ctx, "aws", "ec2", "describe-instances",
		"--region", region,
		"--filters",
		fmt.Sprintf("Name=tag:kubernetes.io/cluster/%s,Values=owned,shared", clusterName),
		"Name=instance-state-name,Values=running,pending,stopping,stopped",
		"--query", "length(Reservations[].Instances[])",
		"--output", "text")
	output, err := cmd.Output()
	if err == nil {
		count := strings.TrimSpace(string(output))
		if count != "" && count != "0" && count != awsNoneValue {
			log.Info("Found running EC2 instances", "count", count)
			return true, nil
		}
	}

	// Check for in-use ENIs tagged with this cluster
	cmd = exec.CommandContext(ctx, "aws", "ec2", "describe-network-interfaces",
		"--region", region,
		"--filters",
		fmt.Sprintf("Name=tag:kubernetes.io/cluster/%s,Values=owned,shared", clusterName),
		"Name=status,Values=in-use",
		"--query", "length(NetworkInterfaces)",
		"--output", "text")
	output, err = cmd.Output()
	if err == nil {
		count := strings.TrimSpace(string(output))
		if count != "" && count != "0" && count != awsNoneValue {
			log.Info("Found in-use ENIs", "count", count)
			return true, nil
		}
	}

	return false, nil
}

// ForceAWSCleanup runs AWS CLI commands to forcibly clean up resources that may block Terraform destroy.
// This includes detaching/deleting ENIs and removing security groups that may have lingering dependencies.
//
// IMPORTANT: This function first cleans up Kubernetes resources (webhooks, API services) that would
// block Helm uninstall operations BEFORE terminating EC2 instances. This prevents the scenario where
// nodes are killed but webhooks still exist with no backing pods, causing "no endpoints available" errors.
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

	// Phase 0: Clean up Kubernetes blocking resources BEFORE terminating nodes
	// This ensures webhooks and API services are removed while the cluster is still healthy,
	// preventing "no endpoints available" errors during subsequent Helm uninstall operations.
	util.Msg("Phase 0: Cleaning up Kubernetes blocking resources...")
	k8sStart := time.Now()
	cleanupKubernetesBlockers(ctx)
	util.Msgf("  Kubernetes cleanup completed in %v", time.Since(k8sStart))

	// Phase 1: Delete LoadBalancers
	util.Msg("Phase 1: Cleaning up LoadBalancers...")
	if err := cleanupLoadBalancers(ctx, clusterName, region); err != nil {
		log.Warn("Error cleaning up LoadBalancers", "error", err)
	}
	util.Msgf("  LoadBalancer cleanup completed in %v", time.Since(startTime))

	// Phase 2: Terminate EC2 instances (especially Karpenter nodes)
	util.Msg("Phase 2: Terminating cluster EC2 instances...")
	ec2Start := time.Now()
	if err := cleanupEC2Instances(ctx, clusterName, region); err != nil {
		log.Warn("Error terminating EC2 instances", "error", err)
	}
	util.Msgf("  EC2 instance cleanup completed in %v", time.Since(ec2Start))

	// Phase 3: Detach and delete ENIs
	util.Msg("Phase 3: Cleaning up ENIs...")
	eniStart := time.Now()
	if err := cleanupENIs(ctx, clusterName, region); err != nil {
		log.Warn("Error cleaning up ENIs", "error", err)
	}
	util.Msgf("  ENI cleanup completed in %v", time.Since(eniStart))

	// Phase 4: Delete Security Groups
	util.Msg("Phase 4: Cleaning up Security Groups...")
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
		if arn == "" || arn == awsNoneValue {
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

// cleanupEC2Instances terminates all EC2 instances tagged with the cluster name.
// This is especially important for Karpenter-provisioned nodes that may not be cleaned up
// when the EKS cluster is destroyed.
func cleanupEC2Instances(ctx context.Context, clusterName, region string) error {
	// Find running instances with cluster tag
	// #nosec G204 -- clusterName and region are validated configuration values, not user input
	cmd := exec.CommandContext(ctx, "aws", "ec2", "describe-instances",
		"--region", region,
		"--filters",
		fmt.Sprintf("Name=tag:kubernetes.io/cluster/%s,Values=owned,shared", clusterName),
		"Name=instance-state-name,Values=running,pending,stopping,stopped",
		"--query", "Reservations[].Instances[].InstanceId",
		"--output", "text")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list EC2 instances: %w", err)
	}

	instanceIds := strings.Fields(strings.TrimSpace(string(output)))
	if len(instanceIds) == 0 || (len(instanceIds) == 1 && instanceIds[0] == "") {
		util.Msg("  No cluster EC2 instances found")
		return nil
	}

	util.Msgf("  Found %d EC2 instances to terminate", len(instanceIds))

	// Terminate instances
	args := []string{"ec2", "terminate-instances", "--region", region, "--instance-ids"}
	args = append(args, instanceIds...)
	cmd = exec.CommandContext(ctx, "aws", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to terminate instances: %w, output: %s", err, string(out))
	}

	util.Msgf("  Initiated termination of %d instances", len(instanceIds))

	// Wait for instances to terminate (up to 5 minutes)
	util.Msg("  Waiting for instances to terminate...")
	for i := 0; i < 30; i++ {
		time.Sleep(10 * time.Second)

		// #nosec G204 -- region is validated configuration, instanceIds are from previous AWS API call
		cmd = exec.CommandContext(ctx, "aws", "ec2", "describe-instances",
			"--region", region,
			"--instance-ids", strings.Join(instanceIds, " "),
			"--query", "Reservations[].Instances[?State.Name!='terminated'].InstanceId",
			"--output", "text")
		output, err := cmd.Output()
		if err != nil {
			// Instances may no longer exist
			break
		}

		remaining := strings.TrimSpace(string(output))
		if remaining == "" || remaining == awsNoneValue {
			util.Msg("  ✅ All instances terminated")
			return nil
		}

		if i%6 == 0 { // Every minute
			util.Msgf("  Still waiting for instances to terminate... (%ds)", (i+1)*10)
		}
	}

	return nil
}

// cleanupENIs detaches and deletes ENIs associated with the cluster.
func cleanupENIs(ctx context.Context, clusterName, region string) error {
	// Find ENIs with cluster tag
	// #nosec G204 -- clusterName and region are validated configuration values, not user input
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

		if eniId == "" || eniId == awsNoneValue {
			continue
		}

		if requesterManaged == "true" || requesterManaged == "True" {
			util.Msgf("  Skipping requester-managed ENI: %s", eniId)
			continue
		}

		if attachmentId != "" && attachmentId != awsNoneValue {
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
	// #nosec G204 -- clusterName and region are validated configuration values, not user input
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
		if eniId == "" || eniId == awsNoneValue {
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
	// #nosec G204 -- clusterName and region are validated configuration values, not user input
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
	// #nosec G204 -- clusterName and region are validated configuration values, not user input
	lbCmd := exec.CommandContext(ctx, "aws", "ec2", "describe-security-groups",
		"--region", region,
		"--filters",
		fmt.Sprintf("Name=tag:elbv2.k8s.aws/cluster,Values=%s", clusterName),
		"--query", "SecurityGroups[].GroupId",
		"--output", "text")
	lbOutput, _ := lbCmd.Output()
	lbSgs := strings.Fields(string(lbOutput))

	// Also find backend SG by name
	// #nosec G204 -- clusterName and region are validated configuration values, not user input
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
		if sg != "" && sg != awsNoneValue {
			sgSet[sg] = true
		}
	}
	for _, sg := range lbSgs {
		if sg != "" && sg != awsNoneValue {
			sgSet[sg] = true
		}
	}
	for _, sg := range backendSgs {
		if sg != "" && sg != awsNoneValue {
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
			_ = revokeCmd.Run() // Ignore errors - best effort cleanup
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
			_ = revokeCmd.Run() // Ignore errors - best effort cleanup
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

// cleanupKubernetesBlockers removes Kubernetes resources that would block Helm uninstall operations.
// This includes:
// - Validating and mutating webhooks (especially Kyverno, Istio, cert-manager)
// - Stale API services (metrics-server, custom-metrics) that block namespace finalization
// - Finalizers on stuck namespaces
//
// This function should be called BEFORE terminating EC2 instances to ensure the cluster
// is still healthy enough to process these deletions.
func cleanupKubernetesBlockers(ctx context.Context) {
	// Use KUBECONFIG from environment if set, otherwise use default path
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = "./out/kubeconfig"
	}

	// Check if kubeconfig file exists
	if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
		log.Warn("Kubeconfig not found, skipping Kubernetes cleanup", "path", kubeconfig)
		return
	}

	// Check if kubectl is available
	if _, err := exec.LookPath("kubectl"); err != nil {
		log.Warn("kubectl not found, skipping Kubernetes cleanup")
		return
	}

	// Test cluster connectivity with a short timeout
	testCmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfig,
		"cluster-info", "--request-timeout=5s")
	if err := testCmd.Run(); err != nil {
		log.Warn("Cluster not reachable, skipping Kubernetes cleanup", "error", err)
		return
	}

	util.Msg("  Removing ALL validating webhooks...")
	// Delete ALL validating webhooks during cleanup to prevent any blocking
	// This is aggressive but safe during teardown - the cluster is being destroyed anyway
	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfig,
		"delete", "validatingwebhookconfiguration", "--all",
		"--ignore-not-found=true", "--timeout=30s")
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Warn("Failed to delete all validating webhooks", "error", err, "output", string(out))
	}

	util.Msg("  Removing ALL mutating webhooks...")
	// Delete ALL mutating webhooks during cleanup
	cmd = exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfig,
		"delete", "mutatingwebhookconfiguration", "--all",
		"--ignore-not-found=true", "--timeout=30s")
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Warn("Failed to delete all mutating webhooks", "error", err, "output", string(out))
	}

	util.Msg("  Removing stale API services...")
	// These API services often block namespace finalization when their backing pods are gone
	staleAPIServices := []string{
		"v1beta1.metrics.k8s.io",
		"v1beta1.external.metrics.k8s.io",
		"v1beta1.custom.metrics.k8s.io",
		"v1.external.metrics.k8s.io",
	}
	for _, apiSvc := range staleAPIServices {
		cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfig,
			"delete", "apiservice", apiSvc,
			"--ignore-not-found=true", "--timeout=10s")
		_ = cmd.Run() // Ignore errors - best effort cleanup
	}

	util.Msg("  Patching stuck namespaces to remove finalizers...")
	// Remove finalizers from namespaces that commonly get stuck during cleanup
	stuckNamespaces := []string{
		"flux-system",
		"kyverno",
		"monitoring",
		"istio-system",
		"cert-manager",
	}
	for _, ns := range stuckNamespaces {
		// Check if namespace exists and is terminating
		cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfig,
			"get", "namespace", ns, "-o", "jsonpath={.status.phase}",
			"--ignore-not-found=true", "--request-timeout=5s")
		out, err := cmd.Output()
		if err != nil || string(out) != "Terminating" {
			continue
		}

		// Remove finalizers from terminating namespace
		patchCmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfig,
			"patch", "namespace", ns, "--type=merge",
			"-p", `{"spec":{"finalizers":null}}`,
			"--timeout=10s")
		if err := patchCmd.Run(); err != nil {
			log.Warn("Failed to patch namespace finalizers", "namespace", ns, "error", err)
		} else {
			util.Msgf("    Removed finalizers from namespace: %s", ns)
		}
	}

	util.Msg("  ✅ Kubernetes blocking resources removed")
}
