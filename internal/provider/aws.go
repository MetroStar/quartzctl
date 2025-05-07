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

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/MetroStar/quartzctl/internal/util"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

const (
	AWS_PROVIDER = "AWS"
)

// AwsClient represents an AWS client with configuration and SDK factory.
// It provides methods to interact with AWS services.
type AwsClient struct {
	cfg aws.Config
	sdk AwsSdkClientFactory

	id     string
	region string
}

type AwsProviderCheckResult struct {
	Identity CloudProviderIdentity
	Error    error
}

func NewLazyAwsClient(ctx context.Context, id string, region string) (AwsClient, error) {
	c, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return AwsClient{}, err
	}

	return NewAwsClient(id, region, c, &LazyAwsSdkClient{
		cfg:    c,
		region: region,
	}), nil
}

func NewAwsClient(id string, region string, cfg aws.Config, sdk AwsSdkClientFactory) AwsClient {
	return AwsClient{
		cfg:    cfg,
		id:     id,
		region: region,
		sdk:    sdk,
	}
}

func (c AwsClient) stateBackendBucketName() string {
	return c.id + "-state-" + c.region
}

func (c AwsClient) stateBackendTableName() string {
	return c.stateBackendBucketName() + "-lock"
}

// ------------- implement interface ICloudProviderClient -------------

// ProviderName returns the name of the cloud provider ("AWS").
func (c AwsClient) ProviderName() string {
	return AWS_PROVIDER
}

func (c AwsClient) CheckConfig() error {
	if c.cfg.Region == "" {
		return fmt.Errorf("aws.region required")
	}

	return nil
}

func (c AwsClient) CheckAccess(ctx context.Context) ProviderCheckResult {
	id, err := c.CurrentIdentity(ctx)

	res := AwsProviderCheckResult{
		Identity: id,
		Error:    err,
	}

	return res
}

func (c AwsClient) CurrentIdentity(ctx context.Context) (CloudProviderIdentity, error) {
	callerId, err := c.sdk.Sts().GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return CloudProviderIdentity{}, err
	}

	aliases, _ := c.sdk.Iam().ListAccountAliases(ctx, &iam.ListAccountAliasesInput{})

	return CloudProviderIdentity{
		AccountId:   *callerId.Account,
		AccountName: strings.Join(aliases.AccountAliases, ", "),
		UserId:      *callerId.UserId,
		UserName:    strings.Split(*callerId.Arn, "/")[1],
	}, nil
}

func (c AwsClient) StateBackendInfo(stage string) CloudProviderStateBackend {
	name := c.stateBackendBucketName()
	bc := []string{
		fmt.Sprintf("bucket=%s", name),
		fmt.Sprintf("dynamodb_table=%s", c.stateBackendTableName()),
		fmt.Sprintf("key=%s.tfstate", stage),
		fmt.Sprintf("region=%s", c.region),
		"encrypt=true",
	}
	return CloudProviderStateBackend{
		Name:              name,
		InitBackendConfig: bc,
	}
}

func (c AwsClient) CreateStateBackend(ctx context.Context) error {
	err := c.CreateBucket(ctx, c.stateBackendBucketName(), false)
	if err != nil {
		return err
	}

	err = c.CreateDynamodbTable(ctx, c.stateBackendTableName(), false)
	if err != nil {
		return err
	}

	return nil
}

func (c AwsClient) DestroyStateBackend(ctx context.Context) error {
	err := c.DestroyDynamodbTable(ctx, c.stateBackendTableName())
	if err != nil {
		return err
	}

	err = c.DestroyBucket(ctx, c.stateBackendBucketName())
	if err != nil {
		return err
	}

	return nil
}

func (c AwsClient) KubeconfigInfo(ctx context.Context) (KubeconfigInfo, error) {
	kc, _, err := c.EksKubeconfigInfo(ctx)
	return kc, err
}

func (c AwsClient) PrintConfig() {
	headers := []string{"Cluster", "Region"}
	rows := [][]string{
		{c.id, c.region},
	}

	util.PrintTable(headers, rows)
}

func (c AwsClient) PrintClusterInfo(ctx context.Context) error {
	o, err := c.DescribeEksCluster(ctx)
	if err != nil {
		return err
	}

	if o == nil {
		return fmt.Errorf("no response from describe cluster")
	}

	name := *o.Cluster.Name
	version := *o.Cluster.Version
	created := *o.Cluster.CreatedAt
	headers := []string{"Name", "Version", "Created"}
	rows := [][]string{
		{name, version, created.String()},
	}
	util.PrintTable(headers, rows)
	return nil
}

func (c AwsClient) PrepareAccount(ctx context.Context) error {
	services := []string{
		"autoscaling.amazonaws.com",
		"spot.amazonaws.com",
	}

	for _, svc := range services {
		_, err := c.sdk.Iam().CreateServiceLinkedRole(ctx, &iam.CreateServiceLinkedRoleInput{
			AWSServiceName: aws.String(svc),
		})
		if err != nil {
			// expecting a 400 response when the roles exist already which they pretty much
			// always will except the first install in a new account
			log.Debug("Error creating service linked role", "service", svc, "err", err)
		}
	}

	return nil
}

// ------------- end ICloudProviderClient -------------

func (r AwsProviderCheckResult) ToTable() ([]string, []ProviderCheckResultRow) {
	headers := []string{"Provider", "Account ID", "Account Name", "User Name"}
	rows := []ProviderCheckResultRow{
		{
			Status: r.Error == nil,
			Error:  r.Error,
			Data:   []string{AWS_PROVIDER, r.Identity.AccountId, r.Identity.AccountName, r.Identity.UserName},
		},
	}

	return headers, rows
}
