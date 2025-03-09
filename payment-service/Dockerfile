FROM golang:1.20-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod tidy
COPY . .
RUN go build -o /payment-service main.go

FROM alpine:3.17
WORKDIR /app
COPY --from=builder /payment-service /app/
CMD ["/app/payment-service"]
