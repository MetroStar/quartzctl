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
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbTypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	eksTypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/aws/smithy-go"
	"sigs.k8s.io/aws-iam-authenticator/pkg/token"
)

// AwsSdkClientMock provides a mock implementation of the AWS SDK client factory.
// It is used for testing AWS-related functionality.
type AwsSdkClientMock struct {
	stsClient         StsClient             // *sts.Client
	iamClient         IamClient             // *iam.Client
	s3Client          S3Client              // *s3.Client
	dynamodbClient    DynamodbClient        // *dynamodb.Client
	eksClient         EksClient             // *eks.Client
	eksTokenGenerator EksTokenGeneratorMock // token.Generator
}

// StsClientMock provides a mock implementation of the STS client.
type StsClientMock struct {
	err     error
	account string
	userid  string
	arn     string
}

// IamClientMock provides a mock implementation of the IAM client.
type IamClientMock struct {
	err            error
	accountAliases []string
}

// S3ClientMock provides a mock implementation of the S3 client.
type S3ClientMock struct {
	t       *testing.T
	err     error
	region  string
	objects []string
	exists  bool
}

// DynamodbClientMock provides a mock implementation of the DynamoDB client.
type DynamodbClientMock struct {
	err error
}

// EksClientMock provides a mock implementation of the EKS client.
type EksClientMock struct {
	err error

	clusterArn             string
	clusterEndpoint        string
	clusterCertificateData string
}

// EksTokenGeneratorMock provides a mock implementation of the EKS token generator.
type EksTokenGeneratorMock struct {
	err        error
	token      string
	expiration time.Time
	json       string
}

// Sts returns the mock STS client.
func (c *AwsSdkClientMock) Sts() StsClient {
	return c.stsClient
}

// Iam returns the mock IAM client.
func (c *AwsSdkClientMock) Iam() IamClient {
	return c.iamClient
}

// Dynamodb returns the mock DynamoDB client.
func (c *AwsSdkClientMock) Dynamodb() DynamodbClient {
	return c.dynamodbClient
}

// S3 returns the mock S3 client.
func (c *AwsSdkClientMock) S3() S3Client {
	return c.s3Client
}

// S3Region returns a mock S3 client for the specified region.
func (c *AwsSdkClientMock) S3Region(region string) S3Client {
	return S3ClientMock{region: region}
}

// Eks returns the mock EKS client.
func (c *AwsSdkClientMock) Eks() EksClient {
	return c.eksClient
}

// EksTokenGenerator returns the mock EKS token generator.
func (c *AwsSdkClientMock) EksTokenGenerator() (token.Generator, error) {
	return c.eksTokenGenerator, nil
}

// GetCallerIdentity returns a mock response for the GetCallerIdentity API call.
func (c StsClientMock) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{
		Account: aws.String(c.account),
		UserId:  aws.String(c.userid),
		Arn:     aws.String(c.arn),
	}, c.err
}

// ListAccountAliases returns a mock response for the ListAccountAliases API call.
func (c IamClientMock) ListAccountAliases(ctx context.Context, params *iam.ListAccountAliasesInput, optFns ...func(*iam.Options)) (*iam.ListAccountAliasesOutput, error) {
	return &iam.ListAccountAliasesOutput{
		AccountAliases: c.accountAliases,
	}, c.err
}

// CreateServiceLinkedRole returns a mock response for the CreateServiceLinkedRole API call.
func (c IamClientMock) CreateServiceLinkedRole(ctx context.Context, params *iam.CreateServiceLinkedRoleInput, optFns ...func(*iam.Options)) (*iam.CreateServiceLinkedRoleOutput, error) {
	return &iam.CreateServiceLinkedRoleOutput{}, c.err
}

// HeadBucket returns a mock response for the HeadBucket API call.
func (c S3ClientMock) HeadBucket(ctx context.Context, params *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
	if !c.exists {
		return &s3.HeadBucketOutput{}, &smithy.GenericAPIError{Code: "NotFound"}
	}

	return &s3.HeadBucketOutput{}, nil
}

// CreateBucket returns a mock response for the CreateBucket API call.
func (c S3ClientMock) CreateBucket(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error) {
	if c.t != nil &&
		c.region != "us-east-1" {
		c.t.Errorf("unsupported region for s3 bucket creation, expected %v, found %v", "us-east-1", c.region)
		return nil, fmt.Errorf("unsupported region")
	}

	return &s3.CreateBucketOutput{}, c.err
}

// DeleteBucket returns a mock response for the DeleteBucket API call.
func (c S3ClientMock) DeleteBucket(ctx context.Context, params *s3.DeleteBucketInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketOutput, error) {
	return &s3.DeleteBucketOutput{}, c.err
}

// ListObjectVersions returns a mock response for the ListObjectVersions API call.
func (c S3ClientMock) ListObjectVersions(ctx context.Context, params *s3.ListObjectVersionsInput, optFns ...func(*s3.Options)) (*s3.ListObjectVersionsOutput, error) {
	versions := []s3Types.ObjectVersion{}
	for _, o := range c.objects {
		versions = append(versions, s3Types.ObjectVersion{
			Key:       aws.String(o),
			VersionId: aws.String(o),
		})
	}

	return &s3.ListObjectVersionsOutput{
		Versions: versions,
	}, c.err
}

// DeleteObjects returns a mock response for the DeleteObjects API call.
func (c S3ClientMock) DeleteObjects(ctx context.Context, params *s3.DeleteObjectsInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error) {
	return &s3.DeleteObjectsOutput{}, c.err
}

// CreateTable returns a mock response for the CreateTable API call.
func (c DynamodbClientMock) CreateTable(ctx context.Context, params *dynamodb.CreateTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.CreateTableOutput, error) {
	return &dynamodb.CreateTableOutput{}, c.err
}

// DeleteTable returns a mock response for the DeleteTable API call.
func (c DynamodbClientMock) DeleteTable(ctx context.Context, params *dynamodb.DeleteTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteTableOutput, error) {
	return &dynamodb.DeleteTableOutput{}, c.err
}

// DescribeTable returns a mock response for the DescribeTable API call.
func (c DynamodbClientMock) DescribeTable(context.Context, *dynamodb.DescribeTableInput, ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	return &dynamodb.DescribeTableOutput{
		Table: &dynamodbTypes.TableDescription{
			TableStatus: "ACTIVE",
		},
	}, c.err
}

// DescribeCluster returns a mock response for the DescribeCluster API call.
func (c EksClientMock) DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
	return &eks.DescribeClusterOutput{
		Cluster: &eksTypes.Cluster{
			Name:      aws.String(c.clusterArn),
			Version:   aws.String("0.1"),
			CreatedAt: aws.Time(time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)),
			Arn:       aws.String(c.clusterArn),
			Endpoint:  aws.String(c.clusterEndpoint),
			CertificateAuthority: &eksTypes.Certificate{
				Data: aws.String(c.clusterCertificateData),
			},
		},
	}, c.err
}

// GetWithOptions returns a mock token for the GetWithOptions API call.
func (c EksTokenGeneratorMock) GetWithOptions(options *token.GetTokenOptions) (token.Token, error) {
	return token.Token{
		Token:      c.token,
		Expiration: c.expiration,
	}, c.err
}

// FormatJSON returns a mock JSON representation of the token.
func (c EksTokenGeneratorMock) FormatJSON(token.Token) string {
	return c.json
}

func (c EksTokenGeneratorMock) Get(string) (token.Token, error) {
	panic("unused")
}

func (c EksTokenGeneratorMock) GetWithRole(clusterID, roleARN string) (token.Token, error) {
	panic("unused")
}

func (c EksTokenGeneratorMock) GetWithRoleForSession(clusterID string, roleARN string, sess *session.Session) (token.Token, error) {
	panic("unused")
}

func (c EksTokenGeneratorMock) GetWithSTS(clusterID string, stsAPI stsiface.STSAPI) (token.Token, error) {
	panic("unused")
}
