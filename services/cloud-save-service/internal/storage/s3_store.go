// Package storage implementa a Port SaveStore em object storage S3-compatible
// (S3/MinIO) via aws-sdk-go-v2 (ADR-0015). O mesmo código serve S3 real e MinIO
// — só muda o endpoint. Ver ADR-0002.
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/c130n/game-backend/services/cloud-save-service/internal/domain"
)

// garante em tempo de compilação que satisfazemos a Port.
var _ domain.SaveStore = (*S3Store)(nil)

// S3Store guarda saves como objetos num bucket S3/MinIO. A key é escopada por
// jogo: `{gameID}/{playerID}` (multi-tenant por jogo, ADR-0017).
type S3Store struct {
	client *s3.Client
	bucket string
}

// objectKey monta a key do save escopada por jogo (prefixo {gameID}/).
func objectKey(gameID, playerID string) string {
	return gameID + "/" + playerID
}

// NewS3Store cria o store apontando para um endpoint S3-compatible e garante
// a existência do bucket. UsePathStyle é obrigatório para o MinIO.
func NewS3Store(ctx context.Context, endpoint, accessKey, secretKey, bucket string) (*S3Store, error) {
	client := s3.New(s3.Options{
		Region:       "us-east-1",
		BaseEndpoint: aws.String(endpoint),
		UsePathStyle: true,
		Credentials:  credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
	})
	store := &S3Store{client: client, bucket: bucket}
	if err := store.ensureBucket(ctx); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *S3Store) ensureBucket(ctx context.Context) error {
	if _, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)}); err == nil {
		return nil
	}
	_, err := s.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(s.bucket)})
	if err != nil {
		var owned *types.BucketAlreadyOwnedByYou
		var exists *types.BucketAlreadyExists
		if errors.As(err, &owned) || errors.As(err, &exists) {
			return nil // criado por outra instância — ok
		}
		return fmt.Errorf("cloud-save storage: ensure bucket: %w", err)
	}
	return nil
}

func (s *S3Store) Put(ctx context.Context, gameID, playerID string, snapshot domain.Snapshot) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(objectKey(gameID, playerID)),
		Body:        bytes.NewReader(snapshot.Blob),
		ContentType: aws.String(snapshot.ContentType),
	})
	if err != nil {
		return fmt.Errorf("cloud-save storage: put: %w", err)
	}
	return nil
}

func (s *S3Store) Get(ctx context.Context, gameID, playerID string) (domain.Snapshot, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey(gameID, playerID)),
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return domain.Snapshot{}, domain.ErrNotFound
		}
		return domain.Snapshot{}, fmt.Errorf("cloud-save storage: get: %w", err)
	}
	defer func() { _ = out.Body.Close() }()

	blob, err := io.ReadAll(out.Body)
	if err != nil {
		return domain.Snapshot{}, fmt.Errorf("cloud-save storage: read: %w", err)
	}
	return domain.Snapshot{
		Blob:        blob,
		ContentType: aws.ToString(out.ContentType),
		UpdatedAt:   aws.ToTime(out.LastModified),
	}, nil
}
