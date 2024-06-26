package kafka

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v4"
	common_utils "github.com/kholiqdev/go-common/utils"
	"github.com/segmentio/kafka-go"
)

type HandlerFunc func(context.Context, *Message) error

type IClient interface {
	Listen(f HandlerFunc) error
	ListenTopic(topic string, f HandlerFunc) error
	NewConsumer()
	IsWriters() bool
	Close() error

	NewPublisher() error
	Publish(ctx context.Context, topic string, msg Event) error
	PublishWithTracer(ctx context.Context, topic string, msg Event) error
	publishToDLQ(ctx context.Context, m kafka.Message) error
	IsReaderConnected() bool

	CreateTopic(topic string, numPart int) error
}

// Message define message encode/decode sarama message
type Message struct {
	Offset        int64  `json:"offset,omitempty"`
	Partition     int    `json:"partition,omitempty"`
	Topic         string `json:"topic,omitempty"`
	Key           string `json:"key,omitempty"`
	Body          []byte `json:"body,omitempty"`
	Timestamp     int64  `json:"timestamp,omitempty"`
	ConsumerGroup string `json:"consumer_group,omitempty"`
	Retry         int    `json:"retry,omitempty"`
	Commit        func() error
	MoveToDLQ     func() error
	Headers       map[string]string
}

type Client struct {
	writer *kafka.Writer

	readers map[string]*kafka.Reader
	cfg     *common_utils.BaseConfig
	Backoff backoff.BackOff
}

func NewKafkaClient(cfg *common_utils.BaseConfig) IClient {
	backoff := backoff.NewExponentialBackOff()
	backoff.MaxElapsedTime = time.Minute * 5

	return &Client{
		cfg:     cfg,
		readers: make(map[string]*kafka.Reader),
		Backoff: backoff,
	}
}
