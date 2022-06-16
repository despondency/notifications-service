package messaging

import (
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"go.uber.org/atomic"
)

type KafkaConsumer struct {
	consumer     *kafka.Consumer
	stop         atomic.Bool
	relayChannel chan []byte
}

func NewKafkaConsumer(bootstrapServers, groupID, topic, offsetReset string) (*KafkaConsumer, error) {
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": bootstrapServers,
		"group.id":          groupID,
		"auto.offset.reset": offsetReset})
	if err != nil {
		return nil, err
	}
	err = c.Subscribe(topic, nil)
	if err != nil {
		return nil, err
	}
	kc := &KafkaConsumer{
		consumer:     c,
		relayChannel: make(chan []byte),
	}
	kc.StartConsuming()
	return kc, nil
}

func (kc KafkaConsumer) Stop() {
	kc.stop.Store(true)
	close(kc.relayChannel)
}

func (kc KafkaConsumer) GetRelayChan() <-chan []byte {
	return kc.relayChannel
}

func (kc *KafkaConsumer) StartConsuming() {
	go func() {
		for kc.stop.Load() == false {
			ev := kc.consumer.Poll(0)
			switch e := ev.(type) {
			case *kafka.Message:
				kc.relayChannel <- e.Value
			case kafka.Error:
				// maybe fatal here?
			default:
				//	fmt.Printf("Ignored %v\n", e)
			}
		}
	}()
}
