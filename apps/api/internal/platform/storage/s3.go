package storage

import (
    "context"
    "fmt"
    "time"

    "github.com/aws/aws-sdk-go-v2/aws"
    awsconfig "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials"
    "github.com/aws/aws-sdk-go-v2/service/s3"
)

type Config struct {
    Endpoint  string
    Region    string
    Bucket    string
    AccessKey string
    SecretKey string
}

type Client struct {
    s3      *s3.Client
    presign *s3.PresignClient
    bucket  string
}

func New(ctx context.Context, cfg Config) (*Client, error) {
    awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
        awsconfig.WithRegion(cfg.Region),
        awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
            cfg.AccessKey, cfg.SecretKey, "",
        )),
    )
    if err != nil {
        return nil, fmt.Errorf("aws config: %w", err)
    }

    client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
        o.BaseEndpoint = aws.String(cfg.Endpoint)
        o.UsePathStyle = true
    })

    return &Client{
        s3:      client,
        presign: s3.NewPresignClient(client),
        bucket:  cfg.Bucket,
    }, nil
}

func (c *Client) PresignPut(ctx context.Context, key, contentType string, expiry time.Duration) (string, error) {
    req, err := c.presign.PresignPutObject(ctx, &s3.PutObjectInput{
        Bucket:      aws.String(c.bucket),
        Key:         aws.String(key),
        ContentType: aws.String(contentType),
    }, s3.WithPresignExpires(expiry))
    if err != nil {
        return "", fmt.Errorf("presign put: %w", err)
    }
    return req.URL, nil
}

func (c *Client) PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error) {
    req, err := c.presign.PresignGetObject(ctx, &s3.GetObjectInput{
        Bucket: aws.String(c.bucket),
        Key:    aws.String(key),
    }, s3.WithPresignExpires(expiry))
    if err != nil {
        return "", fmt.Errorf("presign get: %w", err)
    }
    return req.URL, nil
}

func (c *Client) Head(ctx context.Context, key string) (*s3.HeadObjectOutput, error) {
    return c.s3.HeadObject(ctx, &s3.HeadObjectInput{
        Bucket: aws.String(c.bucket),
        Key:    aws.String(key),
    })
}