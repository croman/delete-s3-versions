package versions

import (
	"fmt"
	"log"
	"math"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/dustin/go-humanize"

	"github.com/croman/delete-s3-versions/config"
	"github.com/croman/delete-s3-versions/s3api"
)

const defaultS3Region = "eu-west-1"
const defaultMaxKeys = 10000

type fileVersion struct {
	Key            string
	VersionID      string
	IsLatest       bool
	LastModified   time.Time
	Size           int64
	IsDeleteMarker bool
}

// S3Versions exposes functionality for dealing with S3 files versions
type S3Versions interface {
	Delete() error
}

type s3Versions struct {
	config *config.Config
	s3     s3api.S3API
}

// New create a new S3Versions instance
func New(c *config.Config) S3Versions {
	s3Config := getS3Config(c)
	svc := s3.New(session.New(), s3Config)

	return &s3Versions{
		config: c,
		s3:     svc,
	}
}

// DeleteOldFileVersions delete older versions of S3 files
func (v *s3Versions) Delete() error {
	buckets, err := v.getBuckets()
	if err != nil {
		return err
	}
	log.Println("Found these buckets", buckets)

	buckets, err = v.filterBucketsByVersioningEnabled(buckets)
	if err != nil {
		return err
	}
	log.Println("Found these buckets with versioning enabled", buckets)

	for _, bucket := range buckets {
		err = v.findAndRemoveVersions(bucket)
		if err != nil {
			return err
		}
	}

	return nil
}

func (v *s3Versions) getBuckets() ([]string, error) {
	if v.config.BucketName == "*" {
		return v.getAllBuckets()
	}

	exists, err := v.existsBucket(v.config.BucketName)
	if err != nil {
		return nil, err
	} else if !exists {
		return nil, fmt.Errorf("Bucket doesn't exist: %s", v.config.BucketName)
	}

	return []string{v.config.BucketName}, nil
}

func (v *s3Versions) getAllBuckets() ([]string, error) {
	log.Println("List all buckets ...")

	input := &s3.ListBucketsInput{}
	response, err := v.s3.ListBuckets(input)
	if err != nil {
		return nil, err
	}

	bucketNames := []string{}
	for _, bucket := range response.Buckets {
		bucketNames = append(bucketNames, *bucket.Name)
	}

	return bucketNames, nil
}

func (v *s3Versions) existsBucket(name string) (bool, error) {
	input := &s3.HeadBucketInput{
		Bucket: aws.String(name),
	}

	_, err := v.s3.HeadBucket(input)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == "NotFound" {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (v *s3Versions) filterBucketsByVersioningEnabled(buckets []string) ([]string, error) {
	bucketsWithVersioning := []string{}

	for _, bucket := range buckets {
		versioningEnabled, err := v.isVersioningEnabled(bucket)
		if err != nil {
			return nil, err
		}

		if versioningEnabled {
			bucketsWithVersioning = append(bucketsWithVersioning, bucket)
		}
	}

	return bucketsWithVersioning, nil
}

func (v *s3Versions) isVersioningEnabled(bucket string) (bool, error) {
	input := &s3.GetBucketVersioningInput{
		Bucket: aws.String(bucket),
	}

	response, err := v.s3.GetBucketVersioning(input)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok && awsErr.Code() == "BucketRegionError" {
			return false, nil
		}

		return false, err
	}

	isEnabled := response.Status != nil && *response.Status == s3.BucketVersioningStatusEnabled

	return isEnabled, nil
}

func (v *s3Versions) findAndRemoveVersions(bucket string) error {
	fileVersions, err := v.getFileVersions(bucket)
	if err != nil {
		return err
	}

	ignoreFilesWithFewerVersions(fileVersions, v.config.VersionsCount)

	versionsToDelete := v.computeAndPrintVersionsInfo(bucket, fileVersions)
	if v.config.Confirm {
		return v.deleteS3Versions(bucket, versionsToDelete)
	}

	return nil
}

func (v *s3Versions) getFileVersions(bucket string) (map[string][]*fileVersion, error) {
	log.Printf("Get file versions for %s/%s", bucket, v.config.BucketPrefix)
	fileVersions := map[string][]*fileVersion{}

	var keyMarker *string
	pageNumber := 1
	versionCount := 0

	for {
		input := &s3.ListObjectVersionsInput{
			Bucket:    aws.String(bucket),
			Prefix:    aws.String(v.config.BucketPrefix),
			KeyMarker: keyMarker,
			MaxKeys:   aws.Int64(defaultMaxKeys),
		}

		response, err := v.s3.ListObjectVersions(input)
		if err != nil {
			return nil, err
		}

		pageVersionCount := len(response.Versions) + len(response.DeleteMarkers)

		log.Printf("\tGot %d versions for page %d", pageVersionCount, pageNumber)

		appendFileVersions(fileVersions, response.Versions)
		appendDeleteMarkers(fileVersions, response.DeleteMarkers)

		versionCount += pageVersionCount

		keyMarker = response.NextKeyMarker
		if keyMarker == nil || len(*keyMarker) == 0 {
			break
		}

		pageNumber++
	}

	log.Printf("Summary: %d file versions for %d files", versionCount, len(fileVersions))

	return fileVersions, nil
}

func appendFileVersions(fileVersions map[string][]*fileVersion, additionalVersions []*s3.ObjectVersion) {
	for _, version := range additionalVersions {
		fv := &fileVersion{
			Key:            *version.Key,
			VersionID:      *version.VersionId,
			IsLatest:       *version.IsLatest,
			LastModified:   *version.LastModified,
			Size:           *version.Size,
			IsDeleteMarker: false,
		}

		if _, ok := fileVersions[fv.Key]; ok {
			fileVersions[fv.Key] = append(fileVersions[fv.Key], fv)
		} else {
			fileVersions[fv.Key] = []*fileVersion{fv}
		}
	}
}

func appendDeleteMarkers(fileVersions map[string][]*fileVersion, deleteMarkers []*s3.DeleteMarkerEntry) {
	for _, marker := range deleteMarkers {
		fv := &fileVersion{
			Key:            *marker.Key,
			VersionID:      *marker.VersionId,
			IsLatest:       *marker.IsLatest,
			LastModified:   *marker.LastModified,
			Size:           0,
			IsDeleteMarker: true,
		}

		if _, ok := fileVersions[fv.Key]; ok {
			fileVersions[fv.Key] = append(fileVersions[fv.Key], fv)
		} else {
			fileVersions[fv.Key] = []*fileVersion{fv}
		}
	}
}

func ignoreFilesWithFewerVersions(fileVersions map[string][]*fileVersion, versionsCount int) {
	for key := range fileVersions {
		if len(fileVersions[key]) <= versionsCount {
			delete(fileVersions, key)
		}
	}
}

func (v *s3Versions) computeAndPrintVersionsInfo(bucket string, fileVersions map[string][]*fileVersion) []*s3.ObjectIdentifier {
	var spaceRecovered int64

	versionsToDelete := []*s3.ObjectIdentifier{}

	for key, versions := range fileVersions {
		sort.Slice(versions, func(i, j int) bool {
			return versions[i].LastModified.After(versions[j].LastModified)
		})

		log.Printf("Versions to delete for %s (count = %d):", key, len(versions)-v.config.VersionsCount)
		versionCount := 0
		for _, version := range versions {
			if versionCount >= v.config.VersionsCount {
				log.Printf("\t %s (%s)", version.VersionID, humanize.Bytes(uint64(version.Size)))
				spaceRecovered += version.Size

				versionsToDelete = append(versionsToDelete, &s3.ObjectIdentifier{
					Key:       aws.String(version.Key),
					VersionId: aws.String(version.VersionID),
				})
			}

			if !version.IsDeleteMarker {
				versionCount++
			}
		}
	}

	log.Printf("Total space recovered for %s: %s", bucket, humanize.Bytes(uint64(spaceRecovered)))
	log.Printf("Total versions to delete for %s: %d", bucket, len(versionsToDelete))

	return versionsToDelete
}

func (v *s3Versions) deleteS3Versions(bucket string, versionsToDelete []*s3.ObjectIdentifier) error {
	log.Printf("Deleting %d file versions for %s/%s ...", len(versionsToDelete), bucket, v.config.BucketPrefix)
	beginIndex := 0

	for beginIndex < len(versionsToDelete) {
		endIndex := int(math.Min(float64(beginIndex+1000), float64(len(versionsToDelete))))
		input := &s3.DeleteObjectsInput{
			Bucket: aws.String(bucket),
			Delete: &s3.Delete{
				Objects: versionsToDelete[beginIndex:endIndex],
			},
		}

		response, err := v.s3.DeleteObjects(input)
		if err != nil {
			return err
		}

		beginIndex = endIndex

		log.Printf("\tDeleted %d versions", len(response.Deleted))
	}

	return nil
}

func getS3Config(c *config.Config) *aws.Config {
	region := c.S3Region
	if len(region) == 0 {
		region = defaultS3Region
	}

	return &aws.Config{
		DisableSSL:       aws.Bool(c.S3DisableSSL == "true"),
		Endpoint:         aws.String(c.S3Endpoint),
		Region:           aws.String(region),
		S3ForcePathStyle: aws.Bool(true),
	}
}
