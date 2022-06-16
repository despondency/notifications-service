package messaging

import (
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/rs/zerolog/log"
)

type KafkaProducer struct {
	producer *kafka.Producer
	receiver chan kafka.Event
}

func NewKafkaProducer(bootstrapServers,
	clientID, acks string) (*KafkaProducer, error) {
	receiver := make(chan kafka.Event)
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
		receiver: receiver,
	}
	kp.listenForResults()
	return kp, nil
}

func (kp *KafkaProducer) Produce(payload []byte, topic string) error {
	err := kp.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          payload,
	},
		kp.receiver,
	)
	if err != nil {
		return err
	}
	return nil
}

func (kp *KafkaProducer) listenForResults() {
	go func() {
		for e := range kp.producer.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					// try to produce it again?
					// raise an error?
					// maybe kafka cluster unhealthy?
					log.Error().Msg(fmt.Sprintf("Failed to deliver message: %v\n", ev.TopicPartition))
				} else {
					log.Debug().Msg(fmt.Sprintf("Successfully produced record to topic %s partition [%d] @ offset %v\n",
						*ev.TopicPartition.Topic, ev.TopicPartition.Partition, ev.TopicPartition.Offset))
				}
			}
		}
	}()
}
