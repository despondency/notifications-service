package messaging

import (
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/rs/zerolog/log"
)

type KafkaProducer struct {
	producer *kafka.Producer
	topic    string
	receiver chan kafka.Event
}

func NewKafkaProducer(bootstrapServers, topic,
	clientID, acks string) (*KafkaProducer, error) {
	receiver := make(chan kafka.Event, 1000)
	producer, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": bootstrapServers,
		"client.id":         clientID,
		"acks":              acks,
	})
	if err != nil {
		return nil, err
	}

	kp := &KafkaProducer{
		producer: producer,
		topic:    topic,
		receiver: receiver,
	}
	kp.deliveredAsync()
	return kp, nil
}

func (kp *KafkaProducer) Produce(payload []byte) error {
	err := kp.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &kp.topic, Partition: kafka.PartitionAny},
		Value:          payload,
	},
		kp.receiver,
	)
	if err != nil {
		return err
	}
	return nil
}

func (kp *KafkaProducer) deliveredAsync() {
	go func() {
		for e := range kp.receiver {
			m := e.(*kafka.Message)
			if m.TopicPartition.Error != nil {
				log.Err(m.TopicPartition.Error).Msg("delivery failed")
			} else {
				log.Info().Msg(fmt.Sprintf(
					"Delivered message to topic %s [%d] at offset %v\n",
					*m.TopicPartition.Topic, m.TopicPartition.Partition, m.TopicPartition.Offset))
			}
		}
	}()
}
