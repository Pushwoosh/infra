# Infra
Infra is a set of go libraries to unify the way we write grpc servers, access various databases, etc.

# Installation
```bash
go get github.com/pushwoosh/infra
```

# Libraries

All databases and brokers libraries have similar design: each library has `NewContainer` function that creates a container
that holds named connections to the database.

To create a new connection, call `Connect` method. To fetch a connection, call `Get` method.

## Databases
- [Postgres](postgres) - based on jackc/pgx
- [Clickhouse](clickhouse)
- [MongoDB](mongodb)
- [Redis](redis) - based on go-redis v9 driver

## Message Brokers
- [RabbitMQ](rabbitmq)
- [Apache Kafka](kafka) - based on segmentio/kafka-go
- [NATS](nats)

## Servers
- [HTTP](http) - http server helpers
- [gRPC](grpc/grpcserver) - gRPC server utilities for creating gRPC servers and gRPC gateways
  - [gRPC middlewares](grpc/grpcserver/middleware) - set of standard middlewares
- [Info](infoserver) - server info endpoint. provides endpoints for k8s liveness and readiness probes, pprof, build info 

## Other
- [GRPC Client](grpc/grpcclient) - has same interface as database and broker libraries
- [Log](log) - zap logger wrapper
- [Netretry](netretry) - retry lib for temporary network errors
- [Must](must) - helper function to panic on error
- [Prometheus pushgateway client](prompushgw) - client for pushgateway, mostly used in cronjobs
- [Operator](operator)
- [System](system) - OS signal handler
