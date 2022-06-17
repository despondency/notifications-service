package messaging

import (
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"
)

var ErrAlreadyStopped = fmt.Errorf("producer already stopped")

type KafkaProducer struct {
	producer *kafka.Producer
	topic    string
	stopped  *atomic.Bool
}

func NewKafkaProducer(bootstrapServers, topic,
	clientID, acks string) (*KafkaProducer, error) {
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
		stopped:  atomic.NewBool(false),
	}
	return kp, nil
}

func (kp *KafkaProducer) Stop() {
	kp.stopped.Store(true)
	kp.producer.Flush(5000)
	kp.producer.Close()
}

func (kp *KafkaProducer) Produce(payload []byte) error {
	if kp.stopped.Load() {
		return ErrAlreadyStopped
	}
	receiver := make(chan kafka.Event, 1)
	err := kp.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &kp.topic, Partition: kafka.PartitionAny},
		Value:          payload,
	},
		receiver,
	)
	if err != nil {
		return err
	}
	e := <-receiver
	m := e.(*kafka.Message)
	if m.TopicPartition.Error != nil {
		log.Err(m.TopicPartition.Error).Msg("Delivery failed")
		return m.TopicPartition.Error
	} else {
		//log.Info().Msg(fmt.Sprintf(
		//	"Delivered message to topic %s [%d] at offset %v\n",
		//	*m.TopicPartition.Topic, m.TopicPartition.Partition, m.TopicPartition.Offset))
	}
	return nil
}
