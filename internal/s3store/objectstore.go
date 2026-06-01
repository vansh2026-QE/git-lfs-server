// Package s3store implements ports.ObjectStore against an S3-compatible
// bucket (AWS S3, MinIO, Cloudflare R2, ...). Unlike memstore.LocalFSObjectStore
// it mints pre-signed URLs only: the client transfers bytes directly to the
// bucket, so this backend has no BlobStore and there are no open transfer
// endpoints. See docs/auth-design.md §4.5.
package s3store

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/vansh2026/git-lfs-server/internal/ports"
)

// defaultURLExpiry is used when Config.URLExpiry is unset or non-positive.
const defaultURLExpiry = 10 * time.Minute

// Config holds the settings needed to construct an S3ObjectStore.
type Config struct {
	Bucket          string
	Region          string
	Endpoint        string // custom endpoint for MinIO/R2; empty means AWS
	AccessKeyID     string
	SecretAccessKey string
	UsePathStyle    bool // path-style addressing; required for MinIO
	Prefix          string
	URLExpiry       time.Duration
}

// presigner is the subset of the S3 presign client used here. It is an
// interface so tests can substitute a fake without reaching MinIO/AWS.
type presigner interface {
	PresignPutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
	PresignGetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

// S3ObjectStore mints pre-signed PUT/GET URLs for objects keyed by
// {prefix}{repo}/{oid}. It satisfies ports.ObjectStore (minting only).
type S3ObjectStore struct {
	presigner presigner
	bucket    string
	prefix    string
	expiry    time.Duration
}

// New constructs an S3ObjectStore. Static credentials are used when both
// AccessKeyID and SecretAccessKey are set; otherwise the AWS default
// credential chain applies. A non-empty Endpoint targets MinIO/R2, and
// UsePathStyle must be true for MinIO.
func New(ctx context.Context, cfg Config) (*S3ObjectStore, error) {
	loadOpts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(cfg.Region)}
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("s3store: loading AWS config: %w", err)
	}

	var s3Opts []func(*s3.Options)
	if cfg.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) { o.BaseEndpoint = aws.String(cfg.Endpoint) })
	}
	if cfg.UsePathStyle {
		s3Opts = append(s3Opts, func(o *s3.Options) { o.UsePathStyle = true })
	}
	client := s3.NewFromConfig(awsCfg, s3Opts...)

	expiry := cfg.URLExpiry
	if expiry <= 0 {
		expiry = defaultURLExpiry
	}
	return &S3ObjectStore{
		presigner: s3.NewPresignClient(client),
		bucket:    cfg.Bucket,
		prefix:    cfg.Prefix,
		expiry:    expiry,
	}, nil
}

// MintUpload returns a pre-signed PUT URL. path and size are unused: objects
// are content-addressed by oid and S3 validates the body itself.
func (s *S3ObjectStore) MintUpload(repo, oid, _ string, _ int64) (ports.ObjectAction, error) {
	req, err := s.presigner.PresignPutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.objectKey(repo, oid)),
	}, s3.WithPresignExpires(s.expiry))
	if err != nil {
		return ports.ObjectAction{}, fmt.Errorf("s3store: presign upload %s: %w", oid, err)
	}
	return s.action(req.URL), nil
}

// MintDownload returns a pre-signed GET URL.
func (s *S3ObjectStore) MintDownload(repo, oid, _ string) (ports.ObjectAction, error) {
	req, err := s.presigner.PresignGetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.objectKey(repo, oid)),
	}, s3.WithPresignExpires(s.expiry))
	if err != nil {
		return ports.ObjectAction{}, fmt.Errorf("s3store: presign download %s: %w", oid, err)
	}
	return s.action(req.URL), nil
}

// action wraps a pre-signed URL in an ObjectAction. The signature is carried
// in the URL's query string, so no extra headers are forwarded to the client.
func (s *S3ObjectStore) action(href string) ports.ObjectAction {
	return ports.ObjectAction{Href: href, ExpiresIn: int(s.expiry.Seconds())}
}

// objectKey is the bucket key for an object: {prefix}{repo}/{oid}. The prefix
// should include a trailing slash if a folder is desired.
func (s *S3ObjectStore) objectKey(repo, oid string) string {
	return s.prefix + repo + "/" + oid
}

var _ ports.ObjectStore = (*S3ObjectStore)(nil)
