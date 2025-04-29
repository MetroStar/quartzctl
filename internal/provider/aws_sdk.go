package provider

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"sigs.k8s.io/aws-iam-authenticator/pkg/token"
)

// AwsSdkClientFactory defines the interface for creating AWS SDK clients.
type AwsSdkClientFactory interface {
	Sts() StsClient
	Iam() IamClient
	Dynamodb() DynamodbClient
	S3() S3Client
	S3Region(region string) S3Client
	Eks() EksClient
	EksTokenGenerator() (token.Generator, error)
}

// LazyAwsSdkClient is a lazy-loading implementation of AWS SDK clients.
// It initializes clients only when they are accessed.
type LazyAwsSdkClient struct {
	cfg    aws.Config
	region string

	stsClient      StsClient      // *sts.Client
	iamClient      IamClient      // *iam.Client
	s3Client       S3Client       // *s3.Client
	dynamodbClient DynamodbClient // *dynamodb.Client
	eksClient      EksClient      // *eks.Client
}

// StsClient defines the interface for interacting with AWS STS.
type StsClient interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

// IamClient defines the interface for interacting with AWS IAM.
type IamClient interface {
	ListAccountAliases(ctx context.Context, params *iam.ListAccountAliasesInput, optFns ...func(*iam.Options)) (*iam.ListAccountAliasesOutput, error)
	CreateServiceLinkedRole(ctx context.Context, params *iam.CreateServiceLinkedRoleInput, optFns ...func(*iam.Options)) (*iam.CreateServiceLinkedRoleOutput, error)
}

// S3Client defines the interface for interacting with AWS S3.
type S3Client interface {
	HeadBucket(ctx context.Context, params *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error)
	CreateBucket(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error)
	DeleteBucket(ctx context.Context, params *s3.DeleteBucketInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketOutput, error)
	ListObjectVersions(ctx context.Context, params *s3.ListObjectVersionsInput, optFns ...func(*s3.Options)) (*s3.ListObjectVersionsOutput, error)
	DeleteObjects(ctx context.Context, params *s3.DeleteObjectsInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error)
}

// DynamodbClient defines the interface for interacting with AWS DynamoDB.
type DynamodbClient interface {
	dynamodb.DescribeTableAPIClient
	CreateTable(ctx context.Context, params *dynamodb.CreateTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.CreateTableOutput, error)
	DeleteTable(ctx context.Context, params *dynamodb.DeleteTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteTableOutput, error)
}

// EksClient defines the interface for interacting with AWS EKS.
type EksClient interface {
	DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error)
}

// Sts returns a lazily initialized STS client.
func (c *LazyAwsSdkClient) Sts() StsClient {
	if c.stsClient == nil {
		c.stsClient = sts.NewFromConfig(c.cfg, func(o *sts.Options) {
			o.Region = c.region
		})
	}

	return c.stsClient
}

// Iam returns a lazily initialized IAM client.
func (c *LazyAwsSdkClient) Iam() IamClient {
	if c.iamClient == nil {
		c.iamClient = iam.NewFromConfig(c.cfg, func(o *iam.Options) {
			o.Region = c.region
		})
	}

	return c.iamClient
}

// S3 returns a lazily initialized S3 client.
func (c *LazyAwsSdkClient) S3() S3Client {
	if c.s3Client == nil {
		c.s3Client = s3.NewFromConfig(c.cfg, func(o *s3.Options) {
			o.Region = c.region
		})
	}

	return c.s3Client
}

// S3Region returns an S3 client configured for the specified region.
func (c *LazyAwsSdkClient) S3Region(region string) S3Client {
	return s3.NewFromConfig(c.cfg, func(o *s3.Options) {
		o.Region = region
	})
}

// Dynamodb returns a lazily initialized DynamoDB client.
func (c *LazyAwsSdkClient) Dynamodb() DynamodbClient {
	if c.dynamodbClient == nil {
		c.dynamodbClient = dynamodb.NewFromConfig(c.cfg, func(o *dynamodb.Options) {
			o.Region = c.region
		})
	}

	return c.dynamodbClient
}

// Eks returns a lazily initialized EKS client.
func (c *LazyAwsSdkClient) Eks() EksClient {
	if c.eksClient == nil {
		c.eksClient = eks.NewFromConfig(c.cfg, func(o *eks.Options) {
			o.Region = c.region
		})
	}

	return c.eksClient
}

// EksTokenGenerator returns a new EKS token generator.
func (c *LazyAwsSdkClient) EksTokenGenerator() (token.Generator, error) {
	return token.NewGenerator(false, false)
}
