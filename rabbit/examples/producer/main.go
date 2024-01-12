package main

import (
	"context"
	"encoding/json"

	"github.com/pushwoosh/infra/rabbit"
)

func main() {
	connName := "name"
	container := infrarabbit.NewContainer()
	err := container.AddConnection(connName, &infrarabbit.ConnectionConfig{
		Address:  "rabbit-12.r2v.nue:5672",
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
				Exchange:   "test-exchange",
				RoutingKey: "test-routing-key",
				Queue:      "test-queue",
			},
		},
	})
	if err != nil {
		panic(err)
	}

	data, _ := json.Marshal(map[string]string{
		"key": "value",
	})

	if err = producer.Produce(context.Background(), data, "test-exchange", "test-routing-key"); err != nil {
		panic(err)
	}
}
