package versions

import (
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/croman/delete-s3-versions/config"
)

func setupBucketsAndObjects() map[string]*fakeBucket {
	fakeBuckets := map[string]*fakeBucket{}

	fakeBuckets["b1"] = &fakeBucket{
		VersioningStatus: aws.String(s3.BucketVersioningStatusEnabled),
		Objects: map[string][]*fakeVersion{
			"key1": []*fakeVersion{
				&fakeVersion{
					VersionID:    "b1-key1-v1",
					LastModified: time.Now().Add(-10 * time.Hour),
				},
				&fakeVersion{
					VersionID:    "b1-key1-v2",
					LastModified: time.Now().Add(-9 * time.Hour),
				},
				&fakeVersion{
					VersionID:      "b1-key1-v2-deleted",
					LastModified:   time.Now().Add(-8 * time.Hour),
					IsDeleteMarker: true,
				},
				&fakeVersion{
					VersionID:    "b1-key1-v3",
					LastModified: time.Now().Add(-7 * time.Hour),
				},
			},
			"key2": []*fakeVersion{
				&fakeVersion{
					VersionID:      "b1-key2-deleted",
					LastModified:   time.Now().Add(-11 * time.Hour),
					IsDeleteMarker: true,
				},
				&fakeVersion{
					VersionID:    "b1-key2-v1",
					LastModified: time.Now().Add(-10 * time.Hour),
				},
				&fakeVersion{
					VersionID:    "b1-key2-v2",
					LastModified: time.Now().Add(-9 * time.Hour),
				},
			},
		},
	}

	fakeBuckets["b2"] = &fakeBucket{
		Objects: map[string][]*fakeVersion{
			"key1": []*fakeVersion{
				&fakeVersion{
					VersionID:    "b2-key1-v1",
					LastModified: time.Now().Add(-10 * time.Hour),
				},
				&fakeVersion{
					VersionID:    "b2-key1-v2",
					LastModified: time.Now().Add(-9 * time.Hour),
				},
			},
			"key2": []*fakeVersion{
				&fakeVersion{
					VersionID:    "b2-key2-v1",
					LastModified: time.Now().Add(-10 * time.Hour),
				},
			},
		},
	}

	fakeBuckets["bucket-in-wrong-region"] = &fakeBucket{
		Objects: map[string][]*fakeVersion{
			"key1": []*fakeVersion{
				&fakeVersion{
					VersionID:    "b3-key1-v1",
					LastModified: time.Now().Add(-10 * time.Hour),
				},
			},
		},
	}

	return fakeBuckets
}

func getBasicTestService(fakeBuckets map[string]*fakeBucket) *s3Versions {
	return &s3Versions{
		config: &config.Config{
			S3Region:     "eu-west-1",
			S3DisableSSL: os.Getenv("S3_DISABLE_SSL"),
			S3Endpoint:   os.Getenv("S3_ENDPOINT"),

			BucketName:    "*",
			BucketPrefix:  "",
			VersionsCount: 1,
			Confirm:       true,
		},
		s3: newS3ApiMock(fakeBuckets, 3),
	}
}

func TestGetBuckets(t *testing.T) {
	fakeBuckets := setupBucketsAndObjects()
	s := getBasicTestService(fakeBuckets)

	buckets, err := s.getBuckets()
	require.Nil(t, err)
	sort.Strings(buckets)
	assert.Equal(t, []string{"b1", "b2", "bucket-in-wrong-region"}, buckets)
}

func TestGetBuckets_OneBucketConfig(t *testing.T) {
	fakeBuckets := setupBucketsAndObjects()
	s := getBasicTestService(fakeBuckets)
	s.config.BucketName = "b1"

	buckets, err := s.getBuckets()
	require.Nil(t, err)
	assert.Equal(t, []string{"b1"}, buckets)
}
func TestGetBuckets_MissingBucketConfig(t *testing.T) {
	fakeBuckets := setupBucketsAndObjects()
	s := getBasicTestService(fakeBuckets)
	s.config.BucketName = "missing-bucket"

	_, err := s.getBuckets()
	assert.True(t, strings.Index(err.Error(), "Bucket doesn't exist") > -1)
}

func TestVersioningEnabled(t *testing.T) {
	fakeBuckets := setupBucketsAndObjects()
	s := getBasicTestService(fakeBuckets)

	buckets, err := s.getBuckets()
	require.Nil(t, err)

	buckets, err = s.filterBucketsByVersioningEnabled(buckets)
	assert.Equal(t, []string{"b1"}, buckets)
}

func TestFindAndDelete(t *testing.T) {
	fakeBuckets := setupBucketsAndObjects()
	s := getBasicTestService(fakeBuckets)

	err := s.Delete()
	require.Nil(t, err)

	assert.Equal(t, 1, len(fakeBuckets["b1"].Objects["key1"]))
	assert.Equal(t, 1, len(fakeBuckets["b1"].Objects["key2"]))
	assert.Equal(t, 2, len(fakeBuckets["b2"].Objects["key1"]))
}

func TestSkipDelete(t *testing.T) {
	fakeBuckets := setupBucketsAndObjects()
	s := getBasicTestService(fakeBuckets)
	s.config.Confirm = false

	err := s.Delete()
	require.Nil(t, err)

	assert.Equal(t, 4, len(fakeBuckets["b1"].Objects["key1"]))
	assert.Equal(t, 3, len(fakeBuckets["b1"].Objects["key2"]))
	assert.Equal(t, 2, len(fakeBuckets["b2"].Objects["key1"]))
}

func TestS3Config(t *testing.T) {
	c := &config.Config{
		S3Region:     "eu-west-2",
		S3DisableSSL: "true",
		S3Endpoint:   "http://localhost:1234",
	}

	assert.Equal(t, getS3Config(c), &aws.Config{
		DisableSSL:       aws.Bool(true),
		Endpoint:         aws.String("http://localhost:1234"),
		Region:           aws.String("eu-west-2"),
		S3ForcePathStyle: aws.Bool(true),
	})

	c = &config.Config{
		S3DisableSSL: "true",
		S3Endpoint:   "http://localhost:1234",
	}

	assert.Equal(t, getS3Config(c), &aws.Config{
		DisableSSL:       aws.Bool(true),
		Endpoint:         aws.String("http://localhost:1234"),
		Region:           aws.String("eu-west-1"),
		S3ForcePathStyle: aws.Bool(true),
	})
}
