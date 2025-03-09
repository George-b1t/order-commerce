FROM golang:1.20-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /order-service main.go

FROM alpine:3.17
WORKDIR /app
COPY --from=builder /order-service /app/
EXPOSE 8080
CMD ["/app/order-service"]
