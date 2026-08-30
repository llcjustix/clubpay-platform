FROM golang:1.22-alpine

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go test ./internal/core ./cmd/api ./cmd/edge-wol
RUN go build -o clubpay-api ./cmd/api

EXPOSE 8080

CMD ["./clubpay-api"]
