package versions

import (
	"sort"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/croman/delete-s3-versions/s3api"
)

type fakeBucket struct {
	VersioningStatus *string
	Objects          map[string][]*fakeVersion
}

type fakeVersion struct {
	VersionID      string
	LastModified   time.Time
	IsLatest       bool
	Size           int64
	IsDeleteMarker bool
}

type s3apiMock struct {
	buckets         map[string]*fakeBucket
	versionsPerPage int
}

func newS3ApiMock(buckets map[string]*fakeBucket, versionsPerPage int) s3api.S3API {
	return &s3apiMock{
		buckets:         buckets,
		versionsPerPage: versionsPerPage,
	}
}

// Mock methods used in the package functionality

func (c *s3apiMock) ListBuckets(input *s3.ListBucketsInput) (*s3.ListBucketsOutput, error) {
	buckets := []*s3.Bucket{}
	for bucketName := range c.buckets {
		buckets = append(buckets, &s3.Bucket{
			Name: aws.String(bucketName),
		})
	}

	return &s3.ListBucketsOutput{
		Buckets: buckets,
	}, nil
}

func (c *s3apiMock) HeadBucket(input *s3.HeadBucketInput) (*s3.HeadBucketOutput, error) {
	if _, ok := c.buckets[*input.Bucket]; ok {
		return &s3.HeadBucketOutput{}, nil
	}

	return nil, awserr.New("NotFound", "NotFound", nil)
}

func (c *s3apiMock) GetBucketVersioning(input *s3.GetBucketVersioningInput) (*s3.GetBucketVersioningOutput, error) {
	if *input.Bucket == "bucket-in-wrong-region" {
		return nil, awserr.New("BucketRegionError", "BucketRegionError", nil)
	}

	if bucket, ok := c.buckets[*input.Bucket]; ok {
		return &s3.GetBucketVersioningOutput{
			Status: bucket.VersioningStatus,
		}, nil
	}

	return nil, awserr.New("NotFound", "NotFound", nil)
}

func (c *s3apiMock) ListObjectVersions(input *s3.ListObjectVersionsInput) (*s3.ListObjectVersionsOutput, error) {
	bucket, ok := c.buckets[*input.Bucket]

	if !ok {
		return nil, awserr.New("NotFound", "NotFound", nil)
	}

	objectVersions := []*s3.ObjectVersion{}
	deleteMarkers := []*s3.DeleteMarkerEntry{}

	var page int
	var err error
	if input.KeyMarker != nil {
		page, err = strconv.Atoi(*input.KeyMarker)
		if err != nil {
			return nil, awserr.New("InvalidKeyMarker", "InvalidKeyMarker", err)
		}
	}

	startIndex := c.versionsPerPage * page
	endIndex := c.versionsPerPage * (page + 1)

	hasNextPage := false
	index := -1

	keys := []string{}
	for key := range bucket.Objects {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if hasNextPage {
			break
		}

		versions := bucket.Objects[key]
		for _, version := range versions {
			index++
			if index < startIndex {
				continue
			}

			if index >= endIndex {
				hasNextPage = true
				break
			}

			if version.IsDeleteMarker {
				deleteMarkers = append(deleteMarkers, &s3.DeleteMarkerEntry{
					Key:          aws.String(key),
					VersionId:    aws.String(version.VersionID),
					IsLatest:     aws.Bool(version.IsLatest),
					LastModified: aws.Time(version.LastModified),
				})
			} else {
				objectVersions = append(objectVersions, &s3.ObjectVersion{
					Key:          aws.String(key),
					VersionId:    aws.String(version.VersionID),
					IsLatest:     aws.Bool(version.IsLatest),
					LastModified: aws.Time(version.LastModified),
					Size:         aws.Int64(version.Size),
				})
			}
		}
	}

	var nextKeymarker *string
	if hasNextPage {
		nextKeymarker = aws.String(strconv.Itoa(page + 1))
	}

	return &s3.ListObjectVersionsOutput{
		DeleteMarkers: deleteMarkers,
		Versions:      objectVersions,
		NextKeyMarker: nextKeymarker,
	}, nil
}

func (c *s3apiMock) DeleteObjects(input *s3.DeleteObjectsInput) (*s3.DeleteObjectsOutput, error) {
	bucket, ok := c.buckets[*input.Bucket]
	if !ok {
		return nil, awserr.New("NotFound", "NotFound", nil)
	}

	deleted := []*s3.DeletedObject{}

	objects := input.Delete.Objects
	for _, object := range objects {
		versions, ok := bucket.Objects[*object.Key]
		if !ok {
			return nil, awserr.New("NotFound", "NotFound", nil)
		}

		for i, version := range versions {
			if version.VersionID == *object.VersionId {
				deleted = append(deleted, &s3.DeletedObject{
					VersionId: aws.String(version.VersionID),
				})

				bucket.Objects[*object.Key] = remove(versions, i)
				break
			}
		}
	}

	return &s3.DeleteObjectsOutput{
		Deleted: deleted,
	}, nil
}

func remove(slice []*fakeVersion, i int) []*fakeVersion {
	copy(slice[i:], slice[i+1:])
	return slice[:len(slice)-1]
}

// Methods not implemented

func (c *s3apiMock) AbortMultipartUploadRequest(input *s3.AbortMultipartUploadInput) (req *request.Request, output *s3.AbortMultipartUploadOutput) {
	return nil, nil
}
func (c *s3apiMock) AbortMultipartUpload(input *s3.AbortMultipartUploadInput) (*s3.AbortMultipartUploadOutput, error) {
	return nil, nil
}
func (c *s3apiMock) AbortMultipartUploadWithContext(ctx aws.Context, input *s3.AbortMultipartUploadInput, opts ...request.Option) (*s3.AbortMultipartUploadOutput, error) {
	return nil, nil
}
func (c *s3apiMock) CompleteMultipartUploadRequest(input *s3.CompleteMultipartUploadInput) (req *request.Request, output *s3.CompleteMultipartUploadOutput) {
	return nil, nil
}
func (c *s3apiMock) CompleteMultipartUpload(input *s3.CompleteMultipartUploadInput) (*s3.CompleteMultipartUploadOutput, error) {
	return nil, nil
}
func (c *s3apiMock) CompleteMultipartUploadWithContext(ctx aws.Context, input *s3.CompleteMultipartUploadInput, opts ...request.Option) (*s3.CompleteMultipartUploadOutput, error) {
	return nil, nil
}
func (c *s3apiMock) CopyObjectRequest(input *s3.CopyObjectInput) (req *request.Request, output *s3.CopyObjectOutput) {
	return nil, nil
}
func (c *s3apiMock) CopyObject(input *s3.CopyObjectInput) (*s3.CopyObjectOutput, error) {
	return nil, nil
}
func (c *s3apiMock) CopyObjectWithContext(ctx aws.Context, input *s3.CopyObjectInput, opts ...request.Option) (*s3.CopyObjectOutput, error) {
	return nil, nil
}
func (c *s3apiMock) CreateBucketRequest(input *s3.CreateBucketInput) (req *request.Request, output *s3.CreateBucketOutput) {
	return nil, nil
}
func (c *s3apiMock) CreateBucket(input *s3.CreateBucketInput) (*s3.CreateBucketOutput, error) {
	return nil, nil
}
func (c *s3apiMock) CreateBucketWithContext(ctx aws.Context, input *s3.CreateBucketInput, opts ...request.Option) (*s3.CreateBucketOutput, error) {
	return nil, nil
}
func (c *s3apiMock) CreateMultipartUploadRequest(input *s3.CreateMultipartUploadInput) (req *request.Request, output *s3.CreateMultipartUploadOutput) {
	return nil, nil
}
func (c *s3apiMock) CreateMultipartUpload(input *s3.CreateMultipartUploadInput) (*s3.CreateMultipartUploadOutput, error) {
	return nil, nil
}
func (c *s3apiMock) CreateMultipartUploadWithContext(ctx aws.Context, input *s3.CreateMultipartUploadInput, opts ...request.Option) (*s3.CreateMultipartUploadOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketRequest(input *s3.DeleteBucketInput) (req *request.Request, output *s3.DeleteBucketOutput) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucket(input *s3.DeleteBucketInput) (*s3.DeleteBucketOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketWithContext(ctx aws.Context, input *s3.DeleteBucketInput, opts ...request.Option) (*s3.DeleteBucketOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketAnalyticsConfigurationRequest(input *s3.DeleteBucketAnalyticsConfigurationInput) (req *request.Request, output *s3.DeleteBucketAnalyticsConfigurationOutput) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketAnalyticsConfiguration(input *s3.DeleteBucketAnalyticsConfigurationInput) (*s3.DeleteBucketAnalyticsConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketAnalyticsConfigurationWithContext(ctx aws.Context, input *s3.DeleteBucketAnalyticsConfigurationInput, opts ...request.Option) (*s3.DeleteBucketAnalyticsConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketCorsRequest(input *s3.DeleteBucketCorsInput) (req *request.Request, output *s3.DeleteBucketCorsOutput) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketCors(input *s3.DeleteBucketCorsInput) (*s3.DeleteBucketCorsOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketCorsWithContext(ctx aws.Context, input *s3.DeleteBucketCorsInput, opts ...request.Option) (*s3.DeleteBucketCorsOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketEncryptionRequest(input *s3.DeleteBucketEncryptionInput) (req *request.Request, output *s3.DeleteBucketEncryptionOutput) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketEncryption(input *s3.DeleteBucketEncryptionInput) (*s3.DeleteBucketEncryptionOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketEncryptionWithContext(ctx aws.Context, input *s3.DeleteBucketEncryptionInput, opts ...request.Option) (*s3.DeleteBucketEncryptionOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketInventoryConfigurationRequest(input *s3.DeleteBucketInventoryConfigurationInput) (req *request.Request, output *s3.DeleteBucketInventoryConfigurationOutput) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketInventoryConfiguration(input *s3.DeleteBucketInventoryConfigurationInput) (*s3.DeleteBucketInventoryConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketInventoryConfigurationWithContext(ctx aws.Context, input *s3.DeleteBucketInventoryConfigurationInput, opts ...request.Option) (*s3.DeleteBucketInventoryConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketLifecycleRequest(input *s3.DeleteBucketLifecycleInput) (req *request.Request, output *s3.DeleteBucketLifecycleOutput) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketLifecycle(input *s3.DeleteBucketLifecycleInput) (*s3.DeleteBucketLifecycleOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketLifecycleWithContext(ctx aws.Context, input *s3.DeleteBucketLifecycleInput, opts ...request.Option) (*s3.DeleteBucketLifecycleOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketMetricsConfigurationRequest(input *s3.DeleteBucketMetricsConfigurationInput) (req *request.Request, output *s3.DeleteBucketMetricsConfigurationOutput) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketMetricsConfiguration(input *s3.DeleteBucketMetricsConfigurationInput) (*s3.DeleteBucketMetricsConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketMetricsConfigurationWithContext(ctx aws.Context, input *s3.DeleteBucketMetricsConfigurationInput, opts ...request.Option) (*s3.DeleteBucketMetricsConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketPolicyRequest(input *s3.DeleteBucketPolicyInput) (req *request.Request, output *s3.DeleteBucketPolicyOutput) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketPolicy(input *s3.DeleteBucketPolicyInput) (*s3.DeleteBucketPolicyOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketPolicyWithContext(ctx aws.Context, input *s3.DeleteBucketPolicyInput, opts ...request.Option) (*s3.DeleteBucketPolicyOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketReplicationRequest(input *s3.DeleteBucketReplicationInput) (req *request.Request, output *s3.DeleteBucketReplicationOutput) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketReplication(input *s3.DeleteBucketReplicationInput) (*s3.DeleteBucketReplicationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketReplicationWithContext(ctx aws.Context, input *s3.DeleteBucketReplicationInput, opts ...request.Option) (*s3.DeleteBucketReplicationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketTaggingRequest(input *s3.DeleteBucketTaggingInput) (req *request.Request, output *s3.DeleteBucketTaggingOutput) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketTagging(input *s3.DeleteBucketTaggingInput) (*s3.DeleteBucketTaggingOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketTaggingWithContext(ctx aws.Context, input *s3.DeleteBucketTaggingInput, opts ...request.Option) (*s3.DeleteBucketTaggingOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketWebsiteRequest(input *s3.DeleteBucketWebsiteInput) (req *request.Request, output *s3.DeleteBucketWebsiteOutput) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketWebsite(input *s3.DeleteBucketWebsiteInput) (*s3.DeleteBucketWebsiteOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeleteBucketWebsiteWithContext(ctx aws.Context, input *s3.DeleteBucketWebsiteInput, opts ...request.Option) (*s3.DeleteBucketWebsiteOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeleteObjectRequest(input *s3.DeleteObjectInput) (req *request.Request, output *s3.DeleteObjectOutput) {
	return nil, nil
}
func (c *s3apiMock) DeleteObject(input *s3.DeleteObjectInput) (*s3.DeleteObjectOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeleteObjectWithContext(ctx aws.Context, input *s3.DeleteObjectInput, opts ...request.Option) (*s3.DeleteObjectOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeleteObjectTaggingRequest(input *s3.DeleteObjectTaggingInput) (req *request.Request, output *s3.DeleteObjectTaggingOutput) {
	return nil, nil
}
func (c *s3apiMock) DeleteObjectTagging(input *s3.DeleteObjectTaggingInput) (*s3.DeleteObjectTaggingOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeleteObjectTaggingWithContext(ctx aws.Context, input *s3.DeleteObjectTaggingInput, opts ...request.Option) (*s3.DeleteObjectTaggingOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeleteObjectsRequest(input *s3.DeleteObjectsInput) (req *request.Request, output *s3.DeleteObjectsOutput) {
	return nil, nil
}
func (c *s3apiMock) DeleteObjectsWithContext(ctx aws.Context, input *s3.DeleteObjectsInput, opts ...request.Option) (*s3.DeleteObjectsOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeletePublicAccessBlockRequest(input *s3.DeletePublicAccessBlockInput) (req *request.Request, output *s3.DeletePublicAccessBlockOutput) {
	return nil, nil
}
func (c *s3apiMock) DeletePublicAccessBlock(input *s3.DeletePublicAccessBlockInput) (*s3.DeletePublicAccessBlockOutput, error) {
	return nil, nil
}
func (c *s3apiMock) DeletePublicAccessBlockWithContext(ctx aws.Context, input *s3.DeletePublicAccessBlockInput, opts ...request.Option) (*s3.DeletePublicAccessBlockOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketAccelerateConfigurationRequest(input *s3.GetBucketAccelerateConfigurationInput) (req *request.Request, output *s3.GetBucketAccelerateConfigurationOutput) {
	return nil, nil
}
func (c *s3apiMock) GetBucketAccelerateConfiguration(input *s3.GetBucketAccelerateConfigurationInput) (*s3.GetBucketAccelerateConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketAccelerateConfigurationWithContext(ctx aws.Context, input *s3.GetBucketAccelerateConfigurationInput, opts ...request.Option) (*s3.GetBucketAccelerateConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketAclRequest(input *s3.GetBucketAclInput) (req *request.Request, output *s3.GetBucketAclOutput) {
	return nil, nil
}
func (c *s3apiMock) GetBucketAcl(input *s3.GetBucketAclInput) (*s3.GetBucketAclOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketAclWithContext(ctx aws.Context, input *s3.GetBucketAclInput, opts ...request.Option) (*s3.GetBucketAclOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketAnalyticsConfigurationRequest(input *s3.GetBucketAnalyticsConfigurationInput) (req *request.Request, output *s3.GetBucketAnalyticsConfigurationOutput) {
	return nil, nil
}
func (c *s3apiMock) GetBucketAnalyticsConfiguration(input *s3.GetBucketAnalyticsConfigurationInput) (*s3.GetBucketAnalyticsConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketAnalyticsConfigurationWithContext(ctx aws.Context, input *s3.GetBucketAnalyticsConfigurationInput, opts ...request.Option) (*s3.GetBucketAnalyticsConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketCorsRequest(input *s3.GetBucketCorsInput) (req *request.Request, output *s3.GetBucketCorsOutput) {
	return nil, nil
}
func (c *s3apiMock) GetBucketCors(input *s3.GetBucketCorsInput) (*s3.GetBucketCorsOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketCorsWithContext(ctx aws.Context, input *s3.GetBucketCorsInput, opts ...request.Option) (*s3.GetBucketCorsOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketEncryptionRequest(input *s3.GetBucketEncryptionInput) (req *request.Request, output *s3.GetBucketEncryptionOutput) {
	return nil, nil
}
func (c *s3apiMock) GetBucketEncryption(input *s3.GetBucketEncryptionInput) (*s3.GetBucketEncryptionOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketEncryptionWithContext(ctx aws.Context, input *s3.GetBucketEncryptionInput, opts ...request.Option) (*s3.GetBucketEncryptionOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketInventoryConfigurationRequest(input *s3.GetBucketInventoryConfigurationInput) (req *request.Request, output *s3.GetBucketInventoryConfigurationOutput) {
	return nil, nil
}
func (c *s3apiMock) GetBucketInventoryConfiguration(input *s3.GetBucketInventoryConfigurationInput) (*s3.GetBucketInventoryConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketInventoryConfigurationWithContext(ctx aws.Context, input *s3.GetBucketInventoryConfigurationInput, opts ...request.Option) (*s3.GetBucketInventoryConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketLifecycleRequest(input *s3.GetBucketLifecycleInput) (req *request.Request, output *s3.GetBucketLifecycleOutput) {
	return nil, nil
}
func (c *s3apiMock) GetBucketLifecycle(input *s3.GetBucketLifecycleInput) (*s3.GetBucketLifecycleOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketLifecycleWithContext(ctx aws.Context, input *s3.GetBucketLifecycleInput, opts ...request.Option) (*s3.GetBucketLifecycleOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketLifecycleConfigurationRequest(input *s3.GetBucketLifecycleConfigurationInput) (req *request.Request, output *s3.GetBucketLifecycleConfigurationOutput) {
	return nil, nil
}
func (c *s3apiMock) GetBucketLifecycleConfiguration(input *s3.GetBucketLifecycleConfigurationInput) (*s3.GetBucketLifecycleConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketLifecycleConfigurationWithContext(ctx aws.Context, input *s3.GetBucketLifecycleConfigurationInput, opts ...request.Option) (*s3.GetBucketLifecycleConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketLocationRequest(input *s3.GetBucketLocationInput) (req *request.Request, output *s3.GetBucketLocationOutput) {
	return nil, nil
}
func (c *s3apiMock) GetBucketLocation(input *s3.GetBucketLocationInput) (*s3.GetBucketLocationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketLocationWithContext(ctx aws.Context, input *s3.GetBucketLocationInput, opts ...request.Option) (*s3.GetBucketLocationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketLoggingRequest(input *s3.GetBucketLoggingInput) (req *request.Request, output *s3.GetBucketLoggingOutput) {
	return nil, nil
}
func (c *s3apiMock) GetBucketLogging(input *s3.GetBucketLoggingInput) (*s3.GetBucketLoggingOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketLoggingWithContext(ctx aws.Context, input *s3.GetBucketLoggingInput, opts ...request.Option) (*s3.GetBucketLoggingOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketMetricsConfigurationRequest(input *s3.GetBucketMetricsConfigurationInput) (req *request.Request, output *s3.GetBucketMetricsConfigurationOutput) {
	return nil, nil
}
func (c *s3apiMock) GetBucketMetricsConfiguration(input *s3.GetBucketMetricsConfigurationInput) (*s3.GetBucketMetricsConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketMetricsConfigurationWithContext(ctx aws.Context, input *s3.GetBucketMetricsConfigurationInput, opts ...request.Option) (*s3.GetBucketMetricsConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketNotificationRequest(input *s3.GetBucketNotificationConfigurationRequest) (req *request.Request, output *s3.NotificationConfigurationDeprecated) {
	return nil, nil
}
func (c *s3apiMock) GetBucketNotification(input *s3.GetBucketNotificationConfigurationRequest) (*s3.NotificationConfigurationDeprecated, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketNotificationWithContext(ctx aws.Context, input *s3.GetBucketNotificationConfigurationRequest, opts ...request.Option) (*s3.NotificationConfigurationDeprecated, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketNotificationConfigurationRequest(input *s3.GetBucketNotificationConfigurationRequest) (req *request.Request, output *s3.NotificationConfiguration) {
	return nil, nil
}
func (c *s3apiMock) GetBucketNotificationConfiguration(input *s3.GetBucketNotificationConfigurationRequest) (*s3.NotificationConfiguration, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketNotificationConfigurationWithContext(ctx aws.Context, input *s3.GetBucketNotificationConfigurationRequest, opts ...request.Option) (*s3.NotificationConfiguration, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketPolicyRequest(input *s3.GetBucketPolicyInput) (req *request.Request, output *s3.GetBucketPolicyOutput) {
	return nil, nil
}
func (c *s3apiMock) GetBucketPolicy(input *s3.GetBucketPolicyInput) (*s3.GetBucketPolicyOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketPolicyWithContext(ctx aws.Context, input *s3.GetBucketPolicyInput, opts ...request.Option) (*s3.GetBucketPolicyOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketPolicyStatusRequest(input *s3.GetBucketPolicyStatusInput) (req *request.Request, output *s3.GetBucketPolicyStatusOutput) {
	return nil, nil
}
func (c *s3apiMock) GetBucketPolicyStatus(input *s3.GetBucketPolicyStatusInput) (*s3.GetBucketPolicyStatusOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketPolicyStatusWithContext(ctx aws.Context, input *s3.GetBucketPolicyStatusInput, opts ...request.Option) (*s3.GetBucketPolicyStatusOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketReplicationRequest(input *s3.GetBucketReplicationInput) (req *request.Request, output *s3.GetBucketReplicationOutput) {
	return nil, nil
}
func (c *s3apiMock) GetBucketReplication(input *s3.GetBucketReplicationInput) (*s3.GetBucketReplicationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketReplicationWithContext(ctx aws.Context, input *s3.GetBucketReplicationInput, opts ...request.Option) (*s3.GetBucketReplicationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketRequestPaymentRequest(input *s3.GetBucketRequestPaymentInput) (req *request.Request, output *s3.GetBucketRequestPaymentOutput) {
	return nil, nil
}
func (c *s3apiMock) GetBucketRequestPayment(input *s3.GetBucketRequestPaymentInput) (*s3.GetBucketRequestPaymentOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketRequestPaymentWithContext(ctx aws.Context, input *s3.GetBucketRequestPaymentInput, opts ...request.Option) (*s3.GetBucketRequestPaymentOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketTaggingRequest(input *s3.GetBucketTaggingInput) (req *request.Request, output *s3.GetBucketTaggingOutput) {
	return nil, nil
}
func (c *s3apiMock) GetBucketTagging(input *s3.GetBucketTaggingInput) (*s3.GetBucketTaggingOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketTaggingWithContext(ctx aws.Context, input *s3.GetBucketTaggingInput, opts ...request.Option) (*s3.GetBucketTaggingOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketVersioningRequest(input *s3.GetBucketVersioningInput) (req *request.Request, output *s3.GetBucketVersioningOutput) {
	return nil, nil
}
func (c *s3apiMock) GetBucketVersioningWithContext(ctx aws.Context, input *s3.GetBucketVersioningInput, opts ...request.Option) (*s3.GetBucketVersioningOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketWebsiteRequest(input *s3.GetBucketWebsiteInput) (req *request.Request, output *s3.GetBucketWebsiteOutput) {
	return nil, nil
}
func (c *s3apiMock) GetBucketWebsite(input *s3.GetBucketWebsiteInput) (*s3.GetBucketWebsiteOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetBucketWebsiteWithContext(ctx aws.Context, input *s3.GetBucketWebsiteInput, opts ...request.Option) (*s3.GetBucketWebsiteOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetObjectRequest(input *s3.GetObjectInput) (req *request.Request, output *s3.GetObjectOutput) {
	return nil, nil
}
func (c *s3apiMock) GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetObjectWithContext(ctx aws.Context, input *s3.GetObjectInput, opts ...request.Option) (*s3.GetObjectOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetObjectAclRequest(input *s3.GetObjectAclInput) (req *request.Request, output *s3.GetObjectAclOutput) {
	return nil, nil
}
func (c *s3apiMock) GetObjectAcl(input *s3.GetObjectAclInput) (*s3.GetObjectAclOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetObjectAclWithContext(ctx aws.Context, input *s3.GetObjectAclInput, opts ...request.Option) (*s3.GetObjectAclOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetObjectLegalHoldRequest(input *s3.GetObjectLegalHoldInput) (req *request.Request, output *s3.GetObjectLegalHoldOutput) {
	return nil, nil
}
func (c *s3apiMock) GetObjectLegalHold(input *s3.GetObjectLegalHoldInput) (*s3.GetObjectLegalHoldOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetObjectLegalHoldWithContext(ctx aws.Context, input *s3.GetObjectLegalHoldInput, opts ...request.Option) (*s3.GetObjectLegalHoldOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetObjectLockConfigurationRequest(input *s3.GetObjectLockConfigurationInput) (req *request.Request, output *s3.GetObjectLockConfigurationOutput) {
	return nil, nil
}
func (c *s3apiMock) GetObjectLockConfiguration(input *s3.GetObjectLockConfigurationInput) (*s3.GetObjectLockConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetObjectLockConfigurationWithContext(ctx aws.Context, input *s3.GetObjectLockConfigurationInput, opts ...request.Option) (*s3.GetObjectLockConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetObjectRetentionRequest(input *s3.GetObjectRetentionInput) (req *request.Request, output *s3.GetObjectRetentionOutput) {
	return nil, nil
}
func (c *s3apiMock) GetObjectRetention(input *s3.GetObjectRetentionInput) (*s3.GetObjectRetentionOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetObjectRetentionWithContext(ctx aws.Context, input *s3.GetObjectRetentionInput, opts ...request.Option) (*s3.GetObjectRetentionOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetObjectTaggingRequest(input *s3.GetObjectTaggingInput) (req *request.Request, output *s3.GetObjectTaggingOutput) {
	return nil, nil
}
func (c *s3apiMock) GetObjectTagging(input *s3.GetObjectTaggingInput) (*s3.GetObjectTaggingOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetObjectTaggingWithContext(ctx aws.Context, input *s3.GetObjectTaggingInput, opts ...request.Option) (*s3.GetObjectTaggingOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetObjectTorrentRequest(input *s3.GetObjectTorrentInput) (req *request.Request, output *s3.GetObjectTorrentOutput) {
	return nil, nil
}
func (c *s3apiMock) GetObjectTorrent(input *s3.GetObjectTorrentInput) (*s3.GetObjectTorrentOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetObjectTorrentWithContext(ctx aws.Context, input *s3.GetObjectTorrentInput, opts ...request.Option) (*s3.GetObjectTorrentOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetPublicAccessBlockRequest(input *s3.GetPublicAccessBlockInput) (req *request.Request, output *s3.GetPublicAccessBlockOutput) {
	return nil, nil
}
func (c *s3apiMock) GetPublicAccessBlock(input *s3.GetPublicAccessBlockInput) (*s3.GetPublicAccessBlockOutput, error) {
	return nil, nil
}
func (c *s3apiMock) GetPublicAccessBlockWithContext(ctx aws.Context, input *s3.GetPublicAccessBlockInput, opts ...request.Option) (*s3.GetPublicAccessBlockOutput, error) {
	return nil, nil
}
func (c *s3apiMock) HeadBucketRequest(input *s3.HeadBucketInput) (req *request.Request, output *s3.HeadBucketOutput) {
	return nil, nil
}
func (c *s3apiMock) HeadBucketWithContext(ctx aws.Context, input *s3.HeadBucketInput, opts ...request.Option) (*s3.HeadBucketOutput, error) {
	return nil, nil
}
func (c *s3apiMock) HeadObjectRequest(input *s3.HeadObjectInput) (req *request.Request, output *s3.HeadObjectOutput) {
	return nil, nil
}
func (c *s3apiMock) HeadObject(input *s3.HeadObjectInput) (*s3.HeadObjectOutput, error) {
	return nil, nil
}
func (c *s3apiMock) HeadObjectWithContext(ctx aws.Context, input *s3.HeadObjectInput, opts ...request.Option) (*s3.HeadObjectOutput, error) {
	return nil, nil
}
func (c *s3apiMock) ListBucketAnalyticsConfigurationsRequest(input *s3.ListBucketAnalyticsConfigurationsInput) (req *request.Request, output *s3.ListBucketAnalyticsConfigurationsOutput) {
	return nil, nil
}
func (c *s3apiMock) ListBucketAnalyticsConfigurations(input *s3.ListBucketAnalyticsConfigurationsInput) (*s3.ListBucketAnalyticsConfigurationsOutput, error) {
	return nil, nil
}
func (c *s3apiMock) ListBucketAnalyticsConfigurationsWithContext(ctx aws.Context, input *s3.ListBucketAnalyticsConfigurationsInput, opts ...request.Option) (*s3.ListBucketAnalyticsConfigurationsOutput, error) {
	return nil, nil
}
func (c *s3apiMock) ListBucketInventoryConfigurationsRequest(input *s3.ListBucketInventoryConfigurationsInput) (req *request.Request, output *s3.ListBucketInventoryConfigurationsOutput) {
	return nil, nil
}
func (c *s3apiMock) ListBucketInventoryConfigurations(input *s3.ListBucketInventoryConfigurationsInput) (*s3.ListBucketInventoryConfigurationsOutput, error) {
	return nil, nil
}
func (c *s3apiMock) ListBucketInventoryConfigurationsWithContext(ctx aws.Context, input *s3.ListBucketInventoryConfigurationsInput, opts ...request.Option) (*s3.ListBucketInventoryConfigurationsOutput, error) {
	return nil, nil
}
func (c *s3apiMock) ListBucketMetricsConfigurationsRequest(input *s3.ListBucketMetricsConfigurationsInput) (req *request.Request, output *s3.ListBucketMetricsConfigurationsOutput) {
	return nil, nil
}
func (c *s3apiMock) ListBucketMetricsConfigurations(input *s3.ListBucketMetricsConfigurationsInput) (*s3.ListBucketMetricsConfigurationsOutput, error) {
	return nil, nil
}
func (c *s3apiMock) ListBucketMetricsConfigurationsWithContext(ctx aws.Context, input *s3.ListBucketMetricsConfigurationsInput, opts ...request.Option) (*s3.ListBucketMetricsConfigurationsOutput, error) {
	return nil, nil
}
func (c *s3apiMock) ListBucketsRequest(input *s3.ListBucketsInput) (req *request.Request, output *s3.ListBucketsOutput) {
	return nil, nil
}
func (c *s3apiMock) ListBucketsWithContext(ctx aws.Context, input *s3.ListBucketsInput, opts ...request.Option) (*s3.ListBucketsOutput, error) {
	return nil, nil
}
func (c *s3apiMock) ListMultipartUploadsRequest(input *s3.ListMultipartUploadsInput) (req *request.Request, output *s3.ListMultipartUploadsOutput) {
	return nil, nil
}
func (c *s3apiMock) ListMultipartUploads(input *s3.ListMultipartUploadsInput) (*s3.ListMultipartUploadsOutput, error) {
	return nil, nil
}
func (c *s3apiMock) ListMultipartUploadsWithContext(ctx aws.Context, input *s3.ListMultipartUploadsInput, opts ...request.Option) (*s3.ListMultipartUploadsOutput, error) {
	return nil, nil
}
func (c *s3apiMock) ListMultipartUploadsPages(input *s3.ListMultipartUploadsInput, fn func(*s3.ListMultipartUploadsOutput, bool) bool) error {
	return nil
}
func (c *s3apiMock) ListMultipartUploadsPagesWithContext(ctx aws.Context, input *s3.ListMultipartUploadsInput, fn func(*s3.ListMultipartUploadsOutput, bool) bool, opts ...request.Option) error {
	return nil
}
func (c *s3apiMock) ListObjectVersionsRequest(input *s3.ListObjectVersionsInput) (req *request.Request, output *s3.ListObjectVersionsOutput) {
	return nil, nil
}
func (c *s3apiMock) ListObjectVersionsWithContext(ctx aws.Context, input *s3.ListObjectVersionsInput, opts ...request.Option) (*s3.ListObjectVersionsOutput, error) {
	return nil, nil
}
func (c *s3apiMock) ListObjectVersionsPages(input *s3.ListObjectVersionsInput, fn func(*s3.ListObjectVersionsOutput, bool) bool) error {
	return nil
}
func (c *s3apiMock) ListObjectVersionsPagesWithContext(ctx aws.Context, input *s3.ListObjectVersionsInput, fn func(*s3.ListObjectVersionsOutput, bool) bool, opts ...request.Option) error {
	return nil
}
func (c *s3apiMock) ListObjectsRequest(input *s3.ListObjectsInput) (req *request.Request, output *s3.ListObjectsOutput) {
	return nil, nil
}
func (c *s3apiMock) ListObjects(input *s3.ListObjectsInput) (*s3.ListObjectsOutput, error) {
	return nil, nil
}
func (c *s3apiMock) ListObjectsWithContext(ctx aws.Context, input *s3.ListObjectsInput, opts ...request.Option) (*s3.ListObjectsOutput, error) {
	return nil, nil
}
func (c *s3apiMock) ListObjectsPages(input *s3.ListObjectsInput, fn func(*s3.ListObjectsOutput, bool) bool) error {
	return nil
}
func (c *s3apiMock) ListObjectsPagesWithContext(ctx aws.Context, input *s3.ListObjectsInput, fn func(*s3.ListObjectsOutput, bool) bool, opts ...request.Option) error {
	return nil
}
func (c *s3apiMock) ListObjectsV2Request(input *s3.ListObjectsV2Input) (req *request.Request, output *s3.ListObjectsV2Output) {
	return nil, nil
}
func (c *s3apiMock) ListObjectsV2(input *s3.ListObjectsV2Input) (*s3.ListObjectsV2Output, error) {
	return nil, nil
}
func (c *s3apiMock) ListObjectsV2WithContext(ctx aws.Context, input *s3.ListObjectsV2Input, opts ...request.Option) (*s3.ListObjectsV2Output, error) {
	return nil, nil
}
func (c *s3apiMock) ListObjectsV2Pages(input *s3.ListObjectsV2Input, fn func(*s3.ListObjectsV2Output, bool) bool) error {
	return nil
}
func (c *s3apiMock) ListObjectsV2PagesWithContext(ctx aws.Context, input *s3.ListObjectsV2Input, fn func(*s3.ListObjectsV2Output, bool) bool, opts ...request.Option) error {
	return nil
}
func (c *s3apiMock) ListPartsRequest(input *s3.ListPartsInput) (req *request.Request, output *s3.ListPartsOutput) {
	return nil, nil
}
func (c *s3apiMock) ListParts(input *s3.ListPartsInput) (*s3.ListPartsOutput, error) {
	return nil, nil
}
func (c *s3apiMock) ListPartsWithContext(ctx aws.Context, input *s3.ListPartsInput, opts ...request.Option) (*s3.ListPartsOutput, error) {
	return nil, nil
}
func (c *s3apiMock) ListPartsPages(input *s3.ListPartsInput, fn func(*s3.ListPartsOutput, bool) bool) error {
	return nil
}
func (c *s3apiMock) ListPartsPagesWithContext(ctx aws.Context, input *s3.ListPartsInput, fn func(*s3.ListPartsOutput, bool) bool, opts ...request.Option) error {
	return nil
}
func (c *s3apiMock) PutBucketAccelerateConfigurationRequest(input *s3.PutBucketAccelerateConfigurationInput) (req *request.Request, output *s3.PutBucketAccelerateConfigurationOutput) {
	return nil, nil
}
func (c *s3apiMock) PutBucketAccelerateConfiguration(input *s3.PutBucketAccelerateConfigurationInput) (*s3.PutBucketAccelerateConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketAccelerateConfigurationWithContext(ctx aws.Context, input *s3.PutBucketAccelerateConfigurationInput, opts ...request.Option) (*s3.PutBucketAccelerateConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketAclRequest(input *s3.PutBucketAclInput) (req *request.Request, output *s3.PutBucketAclOutput) {
	return nil, nil
}
func (c *s3apiMock) PutBucketAcl(input *s3.PutBucketAclInput) (*s3.PutBucketAclOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketAclWithContext(ctx aws.Context, input *s3.PutBucketAclInput, opts ...request.Option) (*s3.PutBucketAclOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketAnalyticsConfigurationRequest(input *s3.PutBucketAnalyticsConfigurationInput) (req *request.Request, output *s3.PutBucketAnalyticsConfigurationOutput) {
	return nil, nil
}
func (c *s3apiMock) PutBucketAnalyticsConfiguration(input *s3.PutBucketAnalyticsConfigurationInput) (*s3.PutBucketAnalyticsConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketAnalyticsConfigurationWithContext(ctx aws.Context, input *s3.PutBucketAnalyticsConfigurationInput, opts ...request.Option) (*s3.PutBucketAnalyticsConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketCorsRequest(input *s3.PutBucketCorsInput) (req *request.Request, output *s3.PutBucketCorsOutput) {
	return nil, nil
}
func (c *s3apiMock) PutBucketCors(input *s3.PutBucketCorsInput) (*s3.PutBucketCorsOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketCorsWithContext(ctx aws.Context, input *s3.PutBucketCorsInput, opts ...request.Option) (*s3.PutBucketCorsOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketEncryptionRequest(input *s3.PutBucketEncryptionInput) (req *request.Request, output *s3.PutBucketEncryptionOutput) {
	return nil, nil
}
func (c *s3apiMock) PutBucketEncryption(input *s3.PutBucketEncryptionInput) (*s3.PutBucketEncryptionOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketEncryptionWithContext(ctx aws.Context, input *s3.PutBucketEncryptionInput, opts ...request.Option) (*s3.PutBucketEncryptionOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketInventoryConfigurationRequest(input *s3.PutBucketInventoryConfigurationInput) (req *request.Request, output *s3.PutBucketInventoryConfigurationOutput) {
	return nil, nil
}
func (c *s3apiMock) PutBucketInventoryConfiguration(input *s3.PutBucketInventoryConfigurationInput) (*s3.PutBucketInventoryConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketInventoryConfigurationWithContext(ctx aws.Context, input *s3.PutBucketInventoryConfigurationInput, opts ...request.Option) (*s3.PutBucketInventoryConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketLifecycleRequest(input *s3.PutBucketLifecycleInput) (req *request.Request, output *s3.PutBucketLifecycleOutput) {
	return nil, nil
}
func (c *s3apiMock) PutBucketLifecycle(input *s3.PutBucketLifecycleInput) (*s3.PutBucketLifecycleOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketLifecycleWithContext(ctx aws.Context, input *s3.PutBucketLifecycleInput, opts ...request.Option) (*s3.PutBucketLifecycleOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketLifecycleConfigurationRequest(input *s3.PutBucketLifecycleConfigurationInput) (req *request.Request, output *s3.PutBucketLifecycleConfigurationOutput) {
	return nil, nil
}
func (c *s3apiMock) PutBucketLifecycleConfiguration(input *s3.PutBucketLifecycleConfigurationInput) (*s3.PutBucketLifecycleConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketLifecycleConfigurationWithContext(ctx aws.Context, input *s3.PutBucketLifecycleConfigurationInput, opts ...request.Option) (*s3.PutBucketLifecycleConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketLoggingRequest(input *s3.PutBucketLoggingInput) (req *request.Request, output *s3.PutBucketLoggingOutput) {
	return nil, nil
}
func (c *s3apiMock) PutBucketLogging(input *s3.PutBucketLoggingInput) (*s3.PutBucketLoggingOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketLoggingWithContext(ctx aws.Context, input *s3.PutBucketLoggingInput, opts ...request.Option) (*s3.PutBucketLoggingOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketMetricsConfigurationRequest(input *s3.PutBucketMetricsConfigurationInput) (req *request.Request, output *s3.PutBucketMetricsConfigurationOutput) {
	return nil, nil
}
func (c *s3apiMock) PutBucketMetricsConfiguration(input *s3.PutBucketMetricsConfigurationInput) (*s3.PutBucketMetricsConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketMetricsConfigurationWithContext(ctx aws.Context, input *s3.PutBucketMetricsConfigurationInput, opts ...request.Option) (*s3.PutBucketMetricsConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketNotificationRequest(input *s3.PutBucketNotificationInput) (req *request.Request, output *s3.PutBucketNotificationOutput) {
	return nil, nil
}
func (c *s3apiMock) PutBucketNotification(input *s3.PutBucketNotificationInput) (*s3.PutBucketNotificationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketNotificationWithContext(ctx aws.Context, input *s3.PutBucketNotificationInput, opts ...request.Option) (*s3.PutBucketNotificationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketNotificationConfigurationRequest(input *s3.PutBucketNotificationConfigurationInput) (req *request.Request, output *s3.PutBucketNotificationConfigurationOutput) {
	return nil, nil
}
func (c *s3apiMock) PutBucketNotificationConfiguration(input *s3.PutBucketNotificationConfigurationInput) (*s3.PutBucketNotificationConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketNotificationConfigurationWithContext(ctx aws.Context, input *s3.PutBucketNotificationConfigurationInput, opts ...request.Option) (*s3.PutBucketNotificationConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketPolicyRequest(input *s3.PutBucketPolicyInput) (req *request.Request, output *s3.PutBucketPolicyOutput) {
	return nil, nil
}
func (c *s3apiMock) PutBucketPolicy(input *s3.PutBucketPolicyInput) (*s3.PutBucketPolicyOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketPolicyWithContext(ctx aws.Context, input *s3.PutBucketPolicyInput, opts ...request.Option) (*s3.PutBucketPolicyOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketReplicationRequest(input *s3.PutBucketReplicationInput) (req *request.Request, output *s3.PutBucketReplicationOutput) {
	return nil, nil
}
func (c *s3apiMock) PutBucketReplication(input *s3.PutBucketReplicationInput) (*s3.PutBucketReplicationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketReplicationWithContext(ctx aws.Context, input *s3.PutBucketReplicationInput, opts ...request.Option) (*s3.PutBucketReplicationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketRequestPaymentRequest(input *s3.PutBucketRequestPaymentInput) (req *request.Request, output *s3.PutBucketRequestPaymentOutput) {
	return nil, nil
}
func (c *s3apiMock) PutBucketRequestPayment(input *s3.PutBucketRequestPaymentInput) (*s3.PutBucketRequestPaymentOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketRequestPaymentWithContext(ctx aws.Context, input *s3.PutBucketRequestPaymentInput, opts ...request.Option) (*s3.PutBucketRequestPaymentOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketTaggingRequest(input *s3.PutBucketTaggingInput) (req *request.Request, output *s3.PutBucketTaggingOutput) {
	return nil, nil
}
func (c *s3apiMock) PutBucketTagging(input *s3.PutBucketTaggingInput) (*s3.PutBucketTaggingOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketTaggingWithContext(ctx aws.Context, input *s3.PutBucketTaggingInput, opts ...request.Option) (*s3.PutBucketTaggingOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketVersioningRequest(input *s3.PutBucketVersioningInput) (req *request.Request, output *s3.PutBucketVersioningOutput) {
	return nil, nil
}
func (c *s3apiMock) PutBucketVersioning(input *s3.PutBucketVersioningInput) (*s3.PutBucketVersioningOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketVersioningWithContext(ctx aws.Context, input *s3.PutBucketVersioningInput, opts ...request.Option) (*s3.PutBucketVersioningOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketWebsiteRequest(input *s3.PutBucketWebsiteInput) (req *request.Request, output *s3.PutBucketWebsiteOutput) {
	return nil, nil
}
func (c *s3apiMock) PutBucketWebsite(input *s3.PutBucketWebsiteInput) (*s3.PutBucketWebsiteOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutBucketWebsiteWithContext(ctx aws.Context, input *s3.PutBucketWebsiteInput, opts ...request.Option) (*s3.PutBucketWebsiteOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutObjectRequest(input *s3.PutObjectInput) (req *request.Request, output *s3.PutObjectOutput) {
	return nil, nil
}
func (c *s3apiMock) PutObject(input *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutObjectWithContext(ctx aws.Context, input *s3.PutObjectInput, opts ...request.Option) (*s3.PutObjectOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutObjectAclRequest(input *s3.PutObjectAclInput) (req *request.Request, output *s3.PutObjectAclOutput) {
	return nil, nil
}
func (c *s3apiMock) PutObjectAcl(input *s3.PutObjectAclInput) (*s3.PutObjectAclOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutObjectAclWithContext(ctx aws.Context, input *s3.PutObjectAclInput, opts ...request.Option) (*s3.PutObjectAclOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutObjectLegalHoldRequest(input *s3.PutObjectLegalHoldInput) (req *request.Request, output *s3.PutObjectLegalHoldOutput) {
	return nil, nil
}
func (c *s3apiMock) PutObjectLegalHold(input *s3.PutObjectLegalHoldInput) (*s3.PutObjectLegalHoldOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutObjectLegalHoldWithContext(ctx aws.Context, input *s3.PutObjectLegalHoldInput, opts ...request.Option) (*s3.PutObjectLegalHoldOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutObjectLockConfigurationRequest(input *s3.PutObjectLockConfigurationInput) (req *request.Request, output *s3.PutObjectLockConfigurationOutput) {
	return nil, nil
}
func (c *s3apiMock) PutObjectLockConfiguration(input *s3.PutObjectLockConfigurationInput) (*s3.PutObjectLockConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutObjectLockConfigurationWithContext(ctx aws.Context, input *s3.PutObjectLockConfigurationInput, opts ...request.Option) (*s3.PutObjectLockConfigurationOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutObjectRetentionRequest(input *s3.PutObjectRetentionInput) (req *request.Request, output *s3.PutObjectRetentionOutput) {
	return nil, nil
}
func (c *s3apiMock) PutObjectRetention(input *s3.PutObjectRetentionInput) (*s3.PutObjectRetentionOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutObjectRetentionWithContext(ctx aws.Context, input *s3.PutObjectRetentionInput, opts ...request.Option) (*s3.PutObjectRetentionOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutObjectTaggingRequest(input *s3.PutObjectTaggingInput) (req *request.Request, output *s3.PutObjectTaggingOutput) {
	return nil, nil
}
func (c *s3apiMock) PutObjectTagging(input *s3.PutObjectTaggingInput) (*s3.PutObjectTaggingOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutObjectTaggingWithContext(ctx aws.Context, input *s3.PutObjectTaggingInput, opts ...request.Option) (*s3.PutObjectTaggingOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutPublicAccessBlockRequest(input *s3.PutPublicAccessBlockInput) (req *request.Request, output *s3.PutPublicAccessBlockOutput) {
	return nil, nil
}
func (c *s3apiMock) PutPublicAccessBlock(input *s3.PutPublicAccessBlockInput) (*s3.PutPublicAccessBlockOutput, error) {
	return nil, nil
}
func (c *s3apiMock) PutPublicAccessBlockWithContext(ctx aws.Context, input *s3.PutPublicAccessBlockInput, opts ...request.Option) (*s3.PutPublicAccessBlockOutput, error) {
	return nil, nil
}
func (c *s3apiMock) RestoreObjectRequest(input *s3.RestoreObjectInput) (req *request.Request, output *s3.RestoreObjectOutput) {
	return nil, nil
}
func (c *s3apiMock) RestoreObject(input *s3.RestoreObjectInput) (*s3.RestoreObjectOutput, error) {
	return nil, nil
}
func (c *s3apiMock) RestoreObjectWithContext(ctx aws.Context, input *s3.RestoreObjectInput, opts ...request.Option) (*s3.RestoreObjectOutput, error) {
	return nil, nil
}
func (c *s3apiMock) SelectObjectContentRequest(input *s3.SelectObjectContentInput) (req *request.Request, output *s3.SelectObjectContentOutput) {
	return nil, nil
}
func (c *s3apiMock) SelectObjectContent(input *s3.SelectObjectContentInput) (*s3.SelectObjectContentOutput, error) {
	return nil, nil
}
func (c *s3apiMock) SelectObjectContentWithContext(ctx aws.Context, input *s3.SelectObjectContentInput, opts ...request.Option) (*s3.SelectObjectContentOutput, error) {
	return nil, nil
}
func (c *s3apiMock) UploadPartRequest(input *s3.UploadPartInput) (req *request.Request, output *s3.UploadPartOutput) {
	return nil, nil
}
func (c *s3apiMock) UploadPart(input *s3.UploadPartInput) (*s3.UploadPartOutput, error) {
	return nil, nil
}
func (c *s3apiMock) UploadPartWithContext(ctx aws.Context, input *s3.UploadPartInput, opts ...request.Option) (*s3.UploadPartOutput, error) {
	return nil, nil
}
func (c *s3apiMock) UploadPartCopyRequest(input *s3.UploadPartCopyInput) (req *request.Request, output *s3.UploadPartCopyOutput) {
	return nil, nil
}
func (c *s3apiMock) UploadPartCopy(input *s3.UploadPartCopyInput) (*s3.UploadPartCopyOutput, error) {
	return nil, nil
}
func (c *s3apiMock) UploadPartCopyWithContext(ctx aws.Context, input *s3.UploadPartCopyInput, opts ...request.Option) (*s3.UploadPartCopyOutput, error) {
	return nil, nil
}
