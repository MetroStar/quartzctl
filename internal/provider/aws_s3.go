package provider

import (
	"context"
	"errors"

	"github.com/MetroStar/quartzctl/internal/log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// CreateBucket creates an S3 bucket with the specified name.
// If the bucket already exists and `force` is false, the operation is skipped.
func (c AwsClient) CreateBucket(ctx context.Context, name string, force bool) error {
	// try to create it regardless of existence
	if !force {
		e, err := c.BucketExists(ctx, name)
		if err != nil {
			log.Info("Failed to lookup bucket", "name", name, "err", err)
			return err
		}

		if e {
			// bucket already exists, skip
			log.Info("Bucket exists already, skipping", "name", name)
			return nil
		}
	}

	var cbCfg *types.CreateBucketConfiguration

	// AWS throws an error if you supply the default region explicitly
	if c.region != "us-east-1" {
		cbCfg = &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(c.region),
		}
	}

	// https://docs.aws.amazon.com/AmazonS3/latest/API/API_CreateBucket.html
	// NOTE: this is deliberately hard coded to us-east-1 per the note indicating
	// s3 api always authenticates against us-east-1 regardless of the requested
	// region when creating buckets :/
	// TODO: assuming this needs work for govcloud
	s3CreateClient := c.sdk.S3Region("us-east-1")
	_, err := s3CreateClient.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket:                    aws.String(name),
		CreateBucketConfiguration: cbCfg,
	})

	if err != nil {
		log.Info("Failed to create bucket", "name", name, "err", err)
		return err
	}

	return nil
}

// DestroyBucket deletes an S3 bucket with the specified name.
// The bucket must be empty before it can be deleted.
func (c *AwsClient) DestroyBucket(ctx context.Context, name string) error {
	resp, err := c.sdk.S3().ListObjectVersions(ctx, &s3.ListObjectVersionsInput{
		Bucket: aws.String(name),
	})

	if err != nil {
		return err
	}

	// bucket has to be empty first
	// list should be short to tf state buckets so not bothering to paginate for now
	if len(resp.Versions) > 0 {
		oi := make([]types.ObjectIdentifier, len(resp.Versions))
		for i, o := range resp.Versions {
			oi[i] = types.ObjectIdentifier{
				Key:       o.Key,
				VersionId: o.VersionId,
			}
		}

		_, err = c.sdk.S3().DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(name),
			Delete: &types.Delete{
				Objects: oi,
			},
		})
		if err != nil {
			return err
		}
	}

	_, err = c.sdk.S3().DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(name),
	})
	return err
}

// BucketExists checks if an S3 bucket with the specified name exists.
// It returns true if the bucket exists, false otherwise, and an error if the operation fails.
func (c *AwsClient) BucketExists(ctx context.Context, name string) (bool, error) {
	_, err := c.sdk.S3().HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(name),
	})

	if err != nil {
		var apiError smithy.APIError
		if errors.As(err, &apiError) {
			switch apiError.ErrorCode() {
			case "BadRequest", "Forbidden", "NotFound":
				return false, nil
			}
		}

		log.Error("Unexpected error checking bucket existence", "name", name, "err", err)
		return false, err
	}

	log.Warn("Bucket already exists", "name", name)
	return true, nil
}
