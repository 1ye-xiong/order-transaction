package rocketmqx

import (
	"context"
	"errors"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type failingSender struct{ err error }

func (s failingSender) Start() error    { return s.err }
func (s failingSender) Shutdown() error { return s.err }
func (s failingSender) SendSync(context.Context, ...*primitive.Message) (*primitive.SendResult, error) {
	return nil, s.err
}
func (s failingSender) SendMessageInTransaction(context.Context, *primitive.Message) (*primitive.TransactionSendResult, error) {
	return nil, s.err
}

type callbackConsumer struct {
	err error
	fn  func(context.Context, ...*primitive.MessageExt) (consumer.ConsumeResult, error)
}

func (c *callbackConsumer) Start() error    { return c.err }
func (c *callbackConsumer) Shutdown() error { return c.err }
func (c *callbackConsumer) Subscribe(_ string, _ consumer.MessageSelector, fn func(context.Context, ...*primitive.MessageExt) (consumer.ConsumeResult, error)) error {
	c.fn = fn
	return c.err
}
func TestNilAndFailureBoundaries(t *testing.T) {
	ctx := context.Background()
	var s *SimpleProducer
	require.Error(t, s.Start())
	_, e := s.Publish(ctx, Message{})
	require.Error(t, e)
	require.NoError(t, s.Close())
	var d *DelayProducer
	require.Error(t, d.Start())
	_, e = d.PublishDelay(ctx, DelayMessage{})
	require.Error(t, e)
	require.NoError(t, d.Close())
	var tx *TransactionProducer
	require.Error(t, tx.Start())
	_, e = tx.PublishTransaction(ctx, TransactionMessage{})
	require.Error(t, e)
	require.NoError(t, tx.Close())
	var c *PushConsumer
	require.Error(t, c.Subscribe(ctx, Subscription{}, nil))
	require.NoError(t, c.Close())
	err := errors.New("transport failed")
	fs := failingSender{err}
	s = NewSimpleProducerWithSender(fs)
	require.ErrorIs(t, s.Start(), err)
	_, e = s.Publish(ctx, Message{})
	require.ErrorIs(t, e, err)
	require.ErrorIs(t, s.Close(), err)
	d = NewDelayProducerWithSender(fs)
	require.ErrorIs(t, d.Start(), err)
	_, e = d.PublishDelay(ctx, DelayMessage{})
	require.ErrorIs(t, e, err)
	require.ErrorIs(t, d.Close(), err)
	tx = NewTransactionProducerWithSender(fs, TransactionCallbacks{})
	require.ErrorIs(t, tx.Start(), err)
	_, e = tx.PublishTransaction(ctx, TransactionMessage{})
	require.ErrorIs(t, e, err)
	require.ErrorIs(t, tx.Close(), err)
	raw := &callbackConsumer{}
	c = NewPushConsumerWithClient(raw)
	require.Error(t, c.Subscribe(ctx, Subscription{}, nil))
	require.NoError(t, c.Subscribe(ctx, Subscription{Topic: "t"}, func(context.Context, MessageExt) error { return err }))
	result, e := raw.fn(ctx, &primitive.MessageExt{})
	require.Equal(t, consumer.ConsumeRetryLater, result)
	require.ErrorIs(t, e, err)
	raw.err = err
	require.ErrorIs(t, c.Subscribe(ctx, Subscription{}, func(context.Context, MessageExt) error { return nil }), err)
	require.ErrorIs(t, c.Close(), err)
}
func TestConfigurationAndMessageMappings(t *testing.T) {
	cfg := (Config{}).Normalize()
	require.Equal(t, DefaultConfig(), cfg)
	cfg.Namespace = "ns"
	cfg.AccessKey = "a"
	require.Len(t, cfg.producerOptions(), 6)
	require.Equal(t, "*", tagExpression([]string{" "}))
	require.Equal(t, "a || b", tagExpression([]string{" a ", "", "b"}))
	require.Empty(t, toSendResult(nil))
	require.Empty(t, toMessageExt(nil))
	require.Empty(t, fromPrimitiveMessage(nil))
	require.Equal(t, time.UnixMilli(42), unixMillis(42))
	require.True(t, unixMillis(0).IsZero())
	for state, want := range map[primitive.SendStatus]string{primitive.SendOK: "send_ok", primitive.SendFlushDiskTimeout: "flush_disk_timeout", primitive.SendFlushSlaveTimeout: "flush_slave_timeout", primitive.SendSlaveNotAvailable: "slave_not_available", 99: "unknown"} {
		require.Equal(t, want, sendStatus(state))
	}
	require.Equal(t, primitive.UnknowState, toPrimitiveTransactionState(UnknownTransaction))
	l := transactionListener{}
	require.Equal(t, primitive.UnknowState, l.ExecuteLocalTransaction(nil))
	require.Equal(t, primitive.UnknowState, l.CheckLocalTransaction(nil))
	l.callbacks.ExecuteLocal = func(context.Context, Message) (TransactionState, error) {
		return CommitTransaction, errors.New("rollback")
	}
	require.Equal(t, primitive.RollbackMessageState, l.ExecuteLocalTransaction(nil))
}
