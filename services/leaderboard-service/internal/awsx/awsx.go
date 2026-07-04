// Package awsx resolve a config comum dos clientes AWS (SNS/SQS) a partir do
// ambiente, seguindo o mesmo padrão do MinIO/S3 (ADR-0004): AWS_ENDPOINT_URL
// setado => LocalStack (credenciais de dev); vazio => AWS real (cadeia padrão).
// Réplica local por serviço (não toca shared/, ver plan da spec 006).
package awsx

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

// Config monta o aws.Config comum aos clientes. Com AWS_ENDPOINT_URL usa
// credenciais fixas de dev (LocalStack aceita qualquer par); sem ele, cai na
// cadeia de credenciais padrão da AWS (role/env/perfil).
func Config(ctx context.Context) (aws.Config, error) {
	region := getenv("AWS_REGION", "us-east-1")
	if Endpoint() != "" {
		return config.LoadDefaultConfig(ctx,
			config.WithRegion(region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
		)
	}
	return config.LoadDefaultConfig(ctx, config.WithRegion(region))
}

// Endpoint devolve o endpoint override (LocalStack) ou "" para AWS real.
func Endpoint() string { return os.Getenv("AWS_ENDPOINT_URL") }

// BaseEndpoint aplica o override de endpoint num Options de cliente (SNS/SQS)
// quando AWS_ENDPOINT_URL está setado. No-op para AWS real.
func BaseEndpoint(endpoint *string) *string {
	if e := Endpoint(); e != "" {
		return aws.String(e)
	}
	return endpoint
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
