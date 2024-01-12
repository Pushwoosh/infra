package main

import (
	"fmt"
	"sync"

	"github.com/pushwoosh/infra/rabbit"
)

func main() {
	container := infrarabbit.NewContainer()
	err := container.AddConnection("name", &infrarabbit.ConnectionConfig{
		Address:  "rabbit-12.r2v.nue:5672",
		Username: "guest",
		Password: "guest",
		Vhost:    "/",
	})
	if err != nil {
		panic(err)
	}

	wg := sync.WaitGroup{}
	queues := []string{"notifications-3-channels", "notifications-1-channels"}
	for _, queue := range queues {
		i := 0
		consumer, err := container.CreateConsumer(&infrarabbit.ConsumerConfig{
			ConnectionName: "name",
			Queue:          queue,
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
				msgErr := msg.Nack()
				if msgErr != nil {
					fmt.Printf("msg error: %s\n", msgErr)
				}
				i++
				fmt.Printf("consumed from %s, index %d\n", queue, i)
			}
		}(queue)
	}
	wg.Wait()
}
