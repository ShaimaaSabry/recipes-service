// Package photostorage keeps recipe photos in the S3 bucket CloudFront serves them from. The app
// uploads straight to S3 with a pre-signed link, so images never pass through the service.
package photostorage

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/apperrors"
)

// UploadLinkTTL is how long an upload link works.
const UploadLinkTTL = 15 * time.Minute

// S3 is the photos bucket.
type S3 struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
	now     func() time.Time
}

// NewS3 uses the AWS credentials of the environment: the ECS task role in prod.
func NewS3(ctx context.Context, bucket string) (*S3, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("photo storage: %w", err)
	}
	return NewS3WithClient(s3.NewFromConfig(cfg, withoutOptionalChecksums), bucket), nil
}

// withoutOptionalChecksums keeps checksums the SDK would add by default out of upload links: the
// app's plain PUT couldn't send them, and S3 would refuse the upload.
func withoutOptionalChecksums(o *s3.Options) {
	o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
}

func NewS3WithClient(client *s3.Client, bucket string) *S3 {
	return &S3{client: client, presign: s3.NewPresignClient(client), bucket: bucket, now: time.Now}
}

// PresignUpload makes a link the app PUTs the image to, with the Content-Type it gave.
func (s *S3) PresignUpload(ctx context.Context, key, contentType string) (model.PhotoUpload, error) {
	request, err := s.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(UploadLinkTTL))
	if err != nil {
		return model.PhotoUpload{}, fmt.Errorf("presign photo upload: %w", err)
	}
	return model.PhotoUpload{
		Key:       key,
		URL:       request.URL,
		Headers:   map[string]string{"Content-Type": contentType},
		ExpiresAt: s.now().Add(UploadLinkTTL),
	}, nil
}

func (s *S3) Stat(ctx context.Context, key string) (model.StoredPhoto, bool, error) {
	head, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if isNotFound(err) {
		return model.StoredPhoto{}, false, nil
	}
	if err != nil {
		return model.StoredPhoto{}, false, fmt.Errorf("check photo: %w", err)
	}
	return model.StoredPhoto{Size: aws.ToInt64(head.ContentLength), ContentType: aws.ToString(head.ContentType)}, true, nil
}

// isNotFound recognises S3 saying there's nothing under a key. HEAD answers carry no body, so a
// 404 may come back without an error code.
func isNotFound(err error) bool {
	var notFound *types.NotFound
	var noSuchKey *types.NoSuchKey
	var response *awshttp.ResponseError
	return errors.As(err, &notFound) || errors.As(err, &noSuchKey) ||
		(errors.As(err, &response) && response.HTTPStatusCode() == http.StatusNotFound)
}

// Delete deletes the photos stored under keys. A key with nothing under it is not an error.
func (s *S3) Delete(ctx context.Context, keys ...string) error {
	for start := 0; start < len(keys); start += 1000 {
		batch := keys[start:min(start+1000, len(keys))]
		objects := make([]types.ObjectIdentifier, 0, len(batch))
		for _, key := range batch {
			objects = append(objects, types.ObjectIdentifier{Key: aws.String(key)})
		}
		out, err := s.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(s.bucket),
			Delete: &types.Delete{Objects: objects, Quiet: aws.Bool(true)},
		})
		if err != nil {
			return fmt.Errorf("delete photos: %w", err)
		}
		if len(out.Errors) > 0 {
			return fmt.Errorf("delete photos: %s: %s", aws.ToString(out.Errors[0].Key), aws.ToString(out.Errors[0].Message))
		}
	}
	return nil
}

// DeleteAll deletes every photo whose key starts with prefix, for account deletion.
func (s *S3) DeleteAll(ctx context.Context, prefix string) error {
	pages := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{Bucket: aws.String(s.bucket), Prefix: aws.String(prefix)})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("list photos: %w", err)
		}
		keys := make([]string, 0, len(page.Contents))
		for _, object := range page.Contents {
			keys = append(keys, aws.ToString(object.Key))
		}
		if err := s.Delete(ctx, keys...); err != nil {
			return err
		}
	}
	return nil
}

// ErrPhotosUnavailable answers photo requests where no bucket is set up (locally, by default).
var ErrPhotosUnavailable = fmt.Errorf("%w: recipe photos aren't set up here (set PHOTOS_BUCKET)", apperrors.ErrUnavailable)

// Disabled is the storage used without a bucket: nothing is ever stored, so there's nothing to
// delete, and uploads are refused.
type Disabled struct{}

func (Disabled) PresignUpload(context.Context, string, string) (model.PhotoUpload, error) {
	return model.PhotoUpload{}, ErrPhotosUnavailable
}

func (Disabled) Stat(context.Context, string) (model.StoredPhoto, bool, error) {
	return model.StoredPhoto{}, false, ErrPhotosUnavailable
}

func (Disabled) Delete(context.Context, ...string) error { return nil }
func (Disabled) DeleteAll(context.Context, string) error { return nil }
