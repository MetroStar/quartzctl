package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/MetroStar/quartzctl/internal/log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// CreateDynamodbTable creates a DynamoDB table with the specified name.
// If the table already exists and `force` is false, the operation is skipped.
func (c *AwsClient) CreateDynamodbTable(ctx context.Context, name string, force bool) error {
	if !force {
		// TODO: figure out timings for these waits
		e := c.DynamodbTableExists(ctx, name, 5*time.Second)
		if e {
			// table already exists, skip
			log.Info("Table exists already, skipping", "name", name)
			return nil
		}
	}

	_, err := c.sdk.Dynamodb().CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName:   aws.String(name),
		BillingMode: types.BillingModeProvisioned,
		AttributeDefinitions: []types.AttributeDefinition{{
			AttributeName: aws.String("LockID"),
			AttributeType: types.ScalarAttributeTypeS,
		}},
		KeySchema: []types.KeySchemaElement{{
			AttributeName: aws.String("LockID"),
			KeyType:       types.KeyTypeHash,
		}},
		ProvisionedThroughput: &types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(5),
			WriteCapacityUnits: aws.Int64(5),
		},
	})

	if err != nil {
		log.Info("Failed to create table", "name", name, "err", err)
		return err
	}

	e := c.DynamodbTableExists(ctx, name, 5*time.Minute)
	if !e {
		return fmt.Errorf("dynamodb table %s not found", name)
	}

	return nil
}

// DestroyDynamodbTable deletes a DynamoDB table with the specified name.
func (c *AwsClient) DestroyDynamodbTable(ctx context.Context, name string) error {
	_, err := c.sdk.Dynamodb().DeleteTable(ctx, &dynamodb.DeleteTableInput{
		TableName: aws.String(name),
	})
	return err
}

// DynamodbTableExists checks if a DynamoDB table with the specified name exists within the given duration.
func (c *AwsClient) DynamodbTableExists(ctx context.Context, name string, d time.Duration) bool {
	w := dynamodb.NewTableExistsWaiter(c.sdk.Dynamodb())
	err := w.Wait(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(name),
	}, d)

	return err == nil
}
