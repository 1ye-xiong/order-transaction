package kafkax

import (
	"context"
	"errors"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestConfigurationDefaultsAndValidation(t *testing.T) {
	cfg, err := (Config{}).Normalize()
	require.NoError(t, err)
	require.Equal(t, DefaultConfig().Topic, cfg.Topic)
	require.Positive(t, cfg.Producer.BatchSize)
	require.Positive(t, cfg.Consumer.MaxBytes)
	cfg, err = (Config{BatchTimeout: time.Second}).Normalize()
	require.NoError(t, err)
	require.Equal(t, time.Second, cfg.Producer.BatchTimeout)
	for _, cfg := range []Config{{Producer: ProducerConfig{Compression: "bad"}}, {Consumer: ConsumerConfig{StartOffset: "bad"}}, {SASL: SASLConfig{Enable: true, Mechanism: "bad"}}} {
		_, err = cfg.Normalize()
		require.Error(t, err)
		_, err = NewProducer(cfg)
		require.Error(t, err)
		_, err = NewConsumer(cfg)
		require.Error(t, err)
	}
	for _, mechanism := range []string{"plain", "scram-sha-256", "scram-sha-512"} {
		cfg := Config{SASL: SASLConfig{Enable: true, Mechanism: mechanism, Username: "u", Password: "p"}, TLS: TLSConfig{Enable: true, ServerName: "broker"}}
		m, e := cfg.saslMechanism()
		require.NoError(t, e)
		require.NotNil(t, m)
		require.NotNil(t, cfg.transport().SASL)
		require.NotNil(t, cfg.dialer().SASLMechanism)
		require.Equal(t, "broker", cfg.tlsConfig().ServerName)
	}
	for _, value := range []string{"", "none", "gzip", "snappy", "lz4", "zstd"} {
		_, err := parseCompression(value)
		require.NoError(t, err)
	}
	for _, value := range []string{"first", "earliest", "oldest", "last", "latest", "newest"} {
		_, err := parseStartOffset(value)
		require.NoError(t, err)
	}
	require.Equal(t, kafka.FirstOffset, (Config{Consumer: ConsumerConfig{StartOffset: "bad"}}).startOffset())
	p, e := NewProducer(Config{})
	require.NoError(t, e)
	require.NoError(t, p.Close())
	r := NewReader(Config{})
	require.NoError(t, r.Close())
	c, e := NewConsumer(Config{})
	require.NoError(t, e)
	require.NoError(t, c.Close())
}

type fakeReader struct {
	fetchErr, commitErr error
	committed           int
}

func (r *fakeReader) FetchMessage(context.Context) (kafka.Message, error) {
	return kafka.Message{Value: []byte("payload")}, r.fetchErr
}
func (r *fakeReader) CommitMessages(context.Context, ...kafka.Message) error {
	r.committed++
	return r.commitErr
}
func (r *fakeReader) Close() error { return r.commitErr }
func TestConsumerCommitsOnlyAfterSuccessfulHandler(t *testing.T) {
	sentinel := errors.New("failure")
	r := &fakeReader{}
	c := NewConsumerWithReader(r)
	require.NoError(t, c.Consume(context.Background(), func(_ context.Context, m kafka.Message) error {
		require.Equal(t, "payload", string(m.Value))
		return nil
	}))
	require.Equal(t, 1, r.committed)
	require.ErrorIs(t, c.Consume(context.Background(), func(context.Context, kafka.Message) error { return sentinel }), sentinel)
	require.Equal(t, 1, r.committed)
	r.fetchErr = sentinel
	require.ErrorIs(t, c.Consume(context.Background(), func(context.Context, kafka.Message) error { return nil }), sentinel)
	r.fetchErr = nil
	r.commitErr = sentinel
	require.ErrorIs(t, c.Consume(context.Background(), func(context.Context, kafka.Message) error { return nil }), sentinel)
	require.Error(t, c.Consume(context.Background(), nil))
	require.ErrorIs(t, c.Run(context.Background(), func(context.Context, kafka.Message) error { return sentinel }), sentinel)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, c.Run(ctx, nil), context.Canceled)
	var nilC *Consumer
	require.Error(t, nilC.Consume(context.Background(), nil))
	require.NoError(t, nilC.Close())
	var nilP *Producer
	require.Error(t, nilP.Publish(context.Background(), Message{}))
	require.Error(t, nilP.PublishRaw(context.Background()))
	require.NoError(t, nilP.Close())
	writer := &captureWriter{}
	p := NewProducerWithWriter(writer)
	require.NoError(t, p.PublishRaw(context.Background(), kafka.Message{Value: []byte("raw")}))
	require.NoError(t, p.Close())
	msg := toKafkaMessage(Message{Headers: map[string]string{"z": "2", "a": "1"}}, false)
	require.Equal(t, "a", msg.Headers[0].Key)
}
