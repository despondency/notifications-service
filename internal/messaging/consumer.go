package messaging

import (
	"github.com/confluentinc/confluent-kafka-go/kafka"
)

type KafkaConsumer[T any] struct {
	consumer     *kafka.Consumer
	relayChannel chan T
}

func NewKafkaConsumer[T any](bootstrapServers, groupID, offsetReset string, relayChannel chan T) (*KafkaConsumer[T], error) {
	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": bootstrapServers,
		"group.id":          groupID,
		"auto.offset.reset": offsetReset})
	if err != nil {
		return nil, err
	}
	return &KafkaConsumer[T]{
		consumer:     consumer,
		relayChannel: relayChannel,
	}, nil
}
