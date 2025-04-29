package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
)

func TestProviderNewLazyAwsClient(t *testing.T) {
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "foo")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "bar")

	c, err := NewLazyAwsClient(context.Background(), "test-cluster", "test-region")
	if err != nil {
		t.Errorf("unexpected error from aws lazu client ctor, %v", err)
	}

	name := c.ProviderName()
	if name != AWS_PROVIDER {
		t.Errorf("unexpected aws client provider name, %v", name)
	}

	sts := c.sdk.Sts()
	t.Logf("sts - %v", sts)

	iam := c.sdk.Iam()
	t.Logf("iam - %v", iam)

	s3 := c.sdk.S3()
	t.Logf("s3 - %v", s3)

	s3Region := c.sdk.S3Region("test-region")
	t.Logf("s3Region - %v", s3Region)

	dynamodb := c.sdk.Dynamodb()
	t.Logf("dynamodb - %v", dynamodb)

	eks := c.sdk.Eks()
	t.Logf("eks - %v", eks)

	eksTokenGen, err := c.sdk.EksTokenGenerator()
	if err != nil {
		t.Errorf("unexpected error from aws client eks token gen ctor, %v", err)
		return
	}
	t.Logf("eksTokenGen - %v", eksTokenGen)
}

// TestProviderAwsClientProviderName verifies that the AWS client returns the correct provider name.
func TestProviderAwsClientProviderName(t *testing.T) {
	c := AwsClient{}
	name := c.ProviderName()
	if name != AWS_PROVIDER {
		t.Errorf("unexpected aws client provider name, %v", name)
	}
}

func TestProviderAwsClientCurrentIdentity(t *testing.T) {
	c := NewAwsClient("", "", aws.Config{},
		&AwsSdkClientMock{
			stsClient: StsClientMock{account: "123456789", userid: "testuserid", arn: "arn:aws:iam::123456789:user/testusername"},
			iamClient: IamClientMock{accountAliases: []string{"testaccount"}},
		})

	id, err := c.CurrentIdentity(context.Background())
	if err != nil {
		t.Errorf("unexpected error from aws client identity lookup, %v", err)
	}

	if id.AccountId != "123456789" ||
		id.UserId != "testuserid" ||
		id.UserName != "testusername" ||
		id.AccountName != "testaccount" {
		t.Errorf("unexpected aws client identity, %v", id)
	}
}

func TestProviderAwsClientCheckAccess(t *testing.T) {
	c := NewAwsClient("", "", aws.Config{},
		&AwsSdkClientMock{
			stsClient: StsClientMock{account: "123456789", userid: "testuserid", arn: "arn:aws:iam::123456789:user/testusername"},
			iamClient: IamClientMock{accountAliases: []string{"testaccount"}},
		})

	res := c.CheckAccess(context.Background())
	switch r := res.(type) {
	case AwsProviderCheckResult:
		if r.Error != nil {
			t.Errorf("unexpected error from aws client check access, %v", r.Error)
			return
		}

		headers, rows := r.ToTable()
		if len(headers) != 4 ||
			len(rows) != 1 {
			t.Errorf("unexpected response from aws client access result table, %v, %v", headers, rows)
		}
	default:
		t.Error("unexpected response type")
	}
}

func TestProviderAwsClientStateBackendInfo(t *testing.T) {
	c := NewAwsClient("testcluster", "us-test-1", aws.Config{}, nil)

	s := c.StateBackendInfo("teststage")

	if s.Name != "testcluster-state-us-test-1" ||
		len(s.InitBackendConfig) != 5 {
		t.Errorf("unexpected aws state backend, %v", s)
	}
}

func TestProviderAwsClientCreateStateBackend(t *testing.T) {
	c := NewAwsClient("testcluster", "us-west-1", aws.Config{
		Region: "us-west-1",
	}, &AwsSdkClientMock{
		s3Client:       S3ClientMock{exists: false},
		dynamodbClient: DynamodbClientMock{},
	})

	err := c.CreateStateBackend(context.Background())
	if err != nil {
		t.Errorf("unexpected error from aws client create backend, %v", err)
	}
}

func TestProviderAwsClientDestroyStateBackend(t *testing.T) {
	c := NewAwsClient("testcluster", "us-east-1", aws.Config{
		Region: "us-east-1",
	}, &AwsSdkClientMock{
		s3Client:       S3ClientMock{objects: []string{"file1", "file2"}},
		dynamodbClient: DynamodbClientMock{},
	})

	err := c.DestroyStateBackend(context.Background())
	if err != nil {
		t.Errorf("unexpected error from aws client destroy backend, %v", err)
	}
}

func TestProviderAwsClientKubeconfigInfo(t *testing.T) {
	c := NewAwsClient("testcluster", "us-east-1", aws.Config{
		Region: "us-east-1",
	}, &AwsSdkClientMock{
		eksClient: EksClientMock{
			clusterArn: "arn:aws:iam::123456789:cluster/testcluster",
		},
		eksTokenGenerator: EksTokenGeneratorMock{
			token: "mysecureapitoken",
		},
	})

	kc, err := c.KubeconfigInfo(context.Background())
	if err != nil {
		t.Errorf("unexpected error from aws client kubeconfig generator, %v", err)
	}

	if kc.Cluster != "arn:aws:iam::123456789:cluster/testcluster" &&
		kc.Token != "mysecureapitoken" {
		t.Errorf("unexpected response from aws client kubeconfig generator, %v", kc)
	}
}

func TestProviderAwsClientPrepareAccount(t *testing.T) {
	c1 := NewAwsClient("testcluster", "us-test-1", aws.Config{}, &AwsSdkClientMock{
		iamClient: IamClientMock{},
	})

	err := c1.PrepareAccount(context.Background())
	if err != nil {
		t.Errorf("unexpected error from aws client prepare account, %v", err)
	}

	c2 := NewAwsClient("testcluster", "us-test-1", aws.Config{}, &AwsSdkClientMock{
		iamClient: IamClientMock{err: fmt.Errorf("simulating error in CreateServiceLinkedRole")},
	})

	err = c2.PrepareAccount(context.Background())
	if err != nil {
		t.Errorf("unexpected error from aws client prepare account, should have been suppressed, %v", err)
	}
}

func TestProviderAwsClientPrint(t *testing.T) {
	c := NewAwsClient("testcluster", "us-test-1", aws.Config{}, &AwsSdkClientMock{
		eksClient: EksClientMock{
			clusterArn: "arn:aws:iam::123456789:cluster/testcluster",
		},
		eksTokenGenerator: EksTokenGeneratorMock{
			token: "mysecureapitoken",
		},
	})

	c.PrintConfig()
	err := c.PrintClusterInfo(context.Background())
	if err != nil {
		t.Errorf("unexpected error from aws client print, %v", err)
	}
}

func TestProviderAwsClientCreateBucketExists(t *testing.T) {
	c := NewAwsClient("testcluster", "us-west-1", aws.Config{
		Region: "us-west-1",
	}, &AwsSdkClientMock{
		s3Client: S3ClientMock{exists: true},
	})

	err := c.CreateBucket(context.Background(), "testbucket", false)
	if err != nil {
		t.Errorf("unexpected error from aws client create bucket, %v", err)
	}
}

func TestProviderAwsClientCreateDynamodbTable(t *testing.T) {
	c := NewAwsClient("testcluster", "us-west-1", aws.Config{
		Region: "us-west-1",
	}, &AwsSdkClientMock{
		dynamodbClient: DynamodbClientMock{},
	})

	err := c.CreateDynamodbTable(context.Background(), "testtable", true)
	if err != nil {
		t.Errorf("unexpected error from aws client create dynamodb table, %v", err)
	}
}

func TestAwsClient_CheckConfig(t *testing.T) {
	client := AwsClient{
		cfg:    aws.Config{Region: "us-west-2"},
		region: "us-west-2",
	}

	err := client.CheckConfig()
	assert.NoError(t, err, "CheckConfig should not return an error for valid configuration")

	client.cfg.Region = ""
	err = client.CheckConfig()
	assert.Error(t, err, "CheckConfig should return an error if aws.Config.Region is not set")
}

func TestAwsClient_StateBackendInfo(t *testing.T) {
	client := AwsClient{
		id:     "test-cluster",
		region: "us-west-2",
	}

	info := client.StateBackendInfo("test-stage")
	assert.Equal(t, "test-cluster-state-us-west-2", info.Name, "StateBackendInfo should return the correct bucket name")
	assert.Contains(t, info.InitBackendConfig, "bucket=test-cluster-state-us-west-2", "StateBackendInfo should include the correct bucket config")
	assert.Contains(t, info.InitBackendConfig, "region=us-west-2", "StateBackendInfo should include the correct region config")
}
