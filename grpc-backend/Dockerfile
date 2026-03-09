# Stage 1: Generate proto + sqlc code
FROM golang:1.25-alpine AS generate

RUN apk add --no-cache git
RUN go install github.com/bufbuild/buf/cmd/buf@v1.66.0
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.6
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.1
RUN go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0

WORKDIR /build

# buf generate (skip if no .proto files)
COPY buf.gen.yaml ./
COPY protos/ protos/
RUN find protos/ -name '*.proto' | grep -q . && buf dep update protos/ && buf generate protos/ || mkdir -p gen/sdk

# sqlc generate
COPY sqlc.yaml ./
COPY sql ./sql
RUN grep -q 'sql: \[\]' sqlc.yaml && mkdir -p gen/db || sqlc generate

# Stage 2: Build
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY . .
COPY --from=generate /build/gen ./gen
RUN go mod tidy
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

# Stage 3: Runtime
FROM alpine:3.21

RUN apk add --no-cache curl
COPY --from=builder /server /server

EXPOSE 8080
CMD ["/server"]
