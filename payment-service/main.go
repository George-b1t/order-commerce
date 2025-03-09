package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/streadway/amqp"
)

type Order struct {
	ID       int64  `json:"id"`
	Customer string `json:"customer"`
	Product  string `json:"product"`
	Status   string `json:"status"`
}

func main() {
	amqpURL := os.Getenv("RABBITMQ_URL")
	if amqpURL == "" {
		amqpURL = "amqp://guest:guest@localhost:5672/"
	}
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		log.Fatal("Erro ao conectar RabbitMQ:", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatal("Erro ao abrir canal RabbitMQ:", err)
	}
	defer ch.Close()

	ch.QueueDeclare("orders.new", true, false, false, false, nil)
	ch.QueueDeclare("orders.approved", true, false, false, false, nil)
	ch.QueueDeclare("orders.failed", true, false, false, false, nil)

	msgs, err := ch.Consume(
		"orders.new",
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatal("Erro ao consumir fila orders.new:", err)
	}

	log.Println("Payment Service aguardando mensagens...")

	go func() {
		for d := range msgs {
			var order Order
			err := json.Unmarshal(d.Body, &order)
			if err != nil {
				log.Println("Erro ao decodificar pedido:", err)
				continue
			}
			log.Printf("Recebido pedido ID=%d para pagamento\n", order.ID)

			time.Sleep(time.Second * 2)
			if rand.Intn(100) < 80 {
				order.Status = "APPROVED"
				body, _ := json.Marshal(order)
				err = ch.Publish(
					"",
					"orders.approved",
					false,
					false,
					amqp.Publishing{
						ContentType: "application/json",
						Body:        body,
					},
				)
				if err != nil {
					log.Println("Erro ao publicar em orders.approved:", err)
				} else {
					log.Printf("Pagamento aprovado para pedido ID=%d\n", order.ID)
				}
			} else {
				order.Status = "FAILED"
				body, _ := json.Marshal(order)
				err = ch.Publish(
					"",
					"orders.failed",
					false,
					false,
					amqp.Publishing{
						ContentType: "application/json",
						Body:        body,
					},
				)
				if err != nil {
					log.Println("Erro ao publicar em orders.failed:", err)
				} else {
					log.Printf("Pagamento reprovado para pedido ID=%d\n", order.ID)
				}
			}
		}
	}()

	select {}
}
