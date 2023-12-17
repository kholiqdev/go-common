package kafka

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	common_utils "github.com/dispenal/go-common/utils"
	"github.com/segmentio/kafka-go"
)

func (k *Client) NewConsumer() {
	batchSize := int(10e6) // 10MB
	dialer := &kafka.Dialer{
		Timeout:   3 * time.Second,
		DualStack: true,
		KeepAlive: 5 * time.Second,
		ClientID:  RandStringBytes(5),
	}
	k.readers = make(map[string]*kafka.Reader)
	for _, topic := range k.cfg.KafkaTopics {
		r := kafka.NewReader(kafka.ReaderConfig{
			Brokers:  k.cfg.KafkaBrokers,
			GroupID:  k.cfg.KafkaGroupID,
			Topic:    topic,
			Dialer:   dialer,
			MaxBytes: batchSize,
		})
		if r == nil {
			common_utils.LogError("empty reader, please check kafka connection")
		}
		common_utils.LogInfo(fmt.Sprintf("Listen: %s, %d, [%s]", r.Stats().Partition, r.Stats().QueueCapacity, r.Stats().Topic))
		k.readers[topic] = r
	}
}

func (k *Client) IsWriters() bool {
	return k.writer != nil
}
func (k *Client) Close() error {
	for _, r := range k.readers {
		r.Close()
	}
	return nil
}

// Listen manual listen
// need call msg.Commit() when process done
// recommend for this process
func (k *Client) Listen(ctx context.Context, handler KafkaHandler) error {
	for _, r := range k.readers {
		go func(r *kafka.Reader) {
			for {
				m, err := r.FetchMessage(ctx) // is not auto commit
				if err != nil && errors.Is(err, io.ErrUnexpectedEOF) {
					break
				}
				if err != nil && errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					log.Print(err)
					continue
				}
				retries := 1
				var errorMsg string

				msg := &Message{
					Offset:    m.Offset,
					Partition: m.Partition,
					Topic:     m.Topic,
					Body:      m.Value,
					Timestamp: m.Time.Unix(),
					Key:       string(m.Key),
					Commit: func() error {
						if err := r.CommitMessages(ctx, m); err != nil {
							return err
						}
						return nil
					},
					MoveToDLQ: func() error {
						return k.publishToDLQ(ctx, m)
					},
				}

				for {
					if retries >= k.cfg.KafkaDlqRetry {
						common_utils.LogError(fmt.Sprintf("failed process message: %s, will move to DLQ", string(m.Key)))

						m.Headers = append(m.Headers, kafka.Header{
							Key:   "error",
							Value: []byte(errorMsg),
						})

						if err := k.publishToDLQ(ctx, m); err != nil {
							common_utils.LogError(fmt.Sprintf("failed move message to DLQ: %s", string(m.Key)))
						}
						break
					}

					if err := handler.Process(ctx, msg); err != nil {
						common_utils.LogError(fmt.Sprintf("failed process message %s with error %v, will retry %d/%d", string(m.Key), err, retries, k.cfg.KafkaDlqRetry))
						errorMsg = err.Error()
						time.Sleep(k.Backoff.NextBackOff())
						retries++
						continue
					}
					break
				}

			}
		}(r)
	}
	return nil
}