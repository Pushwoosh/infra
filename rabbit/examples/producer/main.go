package main

import (
	"context"
	"encoding/json"
	"time"

	infrarabbit "github.com/pushwoosh/infra/rabbit"
)

func main() {
	connName := "name"
	container := infrarabbit.NewContainer()
	err := container.AddConnection(connName, &infrarabbit.ConnectionConfig{
		Address:  "rabbit-host:5672",
		Username: "guest",
		Password: "guest",
		Vhost:    "/",
	})
	if err != nil {
		panic(err)
	}

	producer, err := container.CreateProducer(&infrarabbit.ProducerConfig{
		ConnectionName: connName,
		Bindings: []*infrarabbit.BindConfig{ // optional, will create exchange/queue and bindings
			{
				Exchange:        "test-exchange",
				RoutingKey:      "test-routing-key",
				Queue:           "test-queue",
				ExchangeDurable: true,
				QueueDurable:    true,
				QueueArgs: map[string]interface{}{
					"x-queue-type": "quorum",
				},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	ticker := time.NewTicker(time.Second / 100)
	defer ticker.Stop()

	for range ticker.C {
		var priority uint8 = 0

		data, _ := json.Marshal(map[string]interface{}{
			"priority": priority,
		})

		if err = producer.Produce(context.Background(), &infrarabbit.ProducerMessage{
			Body:       data,
			Exchange:   "test-exchange",
			RoutingKey: "test-routing-key",
			Priority:   priority,
		}); err != nil {
			panic(err)
		}
	}
}
