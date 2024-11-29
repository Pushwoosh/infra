package main

import (
	"fmt"
	"sync"

	infrarabbit "github.com/pushwoosh/infra/rabbit"
)

func main() {
	container := infrarabbit.NewContainer()
	err := container.AddConnection("name", &infrarabbit.ConnectionConfig{
		Address:  "rabbit-host:5672",
		Username: "guest",
		Password: "guest",
		Vhost:    "/",
	})
	if err != nil {
		panic(err)
	}

	wg := sync.WaitGroup{}
	queues := []string{"test-queue"}
	for _, queue := range queues {
		i := 0
		consumer, err := container.CreateConsumer(&infrarabbit.ConsumerConfig{
			ConnectionName: "name",
			Queue:          queue,
			QueuePriority:  10,
			PrefetchCount:  16,
			Tag:            "test-consumer",
		})
		if err != nil {
			panic(err)
		}

		wg.Add(1)
		go func(queue string) {
			defer wg.Done()
			for msg := range consumer.Consume() {
				msgErr := msg.Ack()
				if msgErr != nil {
					fmt.Printf("msg error: %s\n", msgErr)
				}
				i++
				fmt.Printf("consumed from %s, index %d, data: %s\n",
					queue,
					i,
					string(msg.Body()))
			}
		}(queue)
	}
	wg.Wait()
}
