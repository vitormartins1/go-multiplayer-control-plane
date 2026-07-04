package consumer

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/assert"

	"github.com/vitormartins1/go-game-backend/shared/logger"
)

type fakeSQS struct {
	deleted int
}

func (f *fakeSQS) ReceiveMessage(context.Context, *sqs.ReceiveMessageInput, ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	return &sqs.ReceiveMessageOutput{}, nil
}

func (f *fakeSQS) DeleteMessage(context.Context, *sqs.DeleteMessageInput, ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
	f.deleted++
	return &sqs.DeleteMessageOutput{}, nil
}

func msg() sqsMessage {
	return types.Message{Body: aws.String(`{"matchId":"m1"}`), ReceiptHandle: aws.String("rh")}
}

// Sucesso do handler -> a mensagem é deletada (confirmada).
func TestProcess_Sucesso_DeletaMensagem(t *testing.T) {
	f := &fakeSQS{}
	c := New(f, "q", func(context.Context, []byte) (bool, error) { return false, nil }, logger.New("t"), nil)
	c.process(context.Background(), msg())
	assert.Equal(t, 1, f.deleted)
}

// Erro do handler -> a mensagem NÃO é deletada (volta à fila / DLQ).
func TestProcess_Erro_NaoDeleta(t *testing.T) {
	f := &fakeSQS{}
	c := New(f, "q", func(context.Context, []byte) (bool, error) { return false, errors.New("boom") }, logger.New("t"), nil)
	c.process(context.Background(), msg())
	assert.Equal(t, 0, f.deleted)
}

// Duplicada (dup=true, sem erro) -> mensagem é deletada normalmente (no-op ok).
func TestProcess_Duplicada_Deleta(t *testing.T) {
	f := &fakeSQS{}
	c := New(f, "q", func(context.Context, []byte) (bool, error) { return true, nil }, logger.New("t"), nil)
	c.process(context.Background(), msg())
	assert.Equal(t, 1, f.deleted)
}
