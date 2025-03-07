all:
	GO_PACKAGE="github.com/pushwoosh/infra/grpc/grpcclient/testpb"
	protoc \
		--go_out=. --go_opt=module=$(GO_PACKAGE) --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=module=$(GO_PACKAGE) --go-grpc_opt=paths=source_relative \
		 service.proto

deps-install:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
