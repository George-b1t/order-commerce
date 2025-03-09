package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/streadway/amqp"
)

type Order struct {
	ID       int64  `json:"id"`
	Customer string `json:"customer"`
	Product  string `json:"product"`
	Status   string `json:"status"`
}

var db *sql.DB
var ctx = context.Background()

func initDB() (*sql.DB, error) {
	connStr := os.Getenv("DB_CONN_STRING")
	if connStr == "" {
		connStr = "postgres://user:password@localhost:5432/mydb?sslmode=disable"
	}
	return sql.Open("postgres", connStr)
}

func main() {
	var err error
	db, err = initDB()
	if err != nil {
		log.Fatal("Erro ao conectar ao DB:", err)
	}
	defer db.Close()

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

	ch.QueueDeclare("orders.approved", true, false, false, false, nil)

	msgs, err := ch.Consume(
		"orders.approved",
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatal("Erro ao consumir fila orders.approved:", err)
	}

	log.Println("Delivery Service aguardando pedidos aprovados...")

	go func() {
		for d := range msgs {
			var order Order
			err := json.Unmarshal(d.Body, &order)
			if err != nil {
				log.Println("Erro ao decodificar pedido:", err)
				continue
			}
			log.Printf("Pedido ID=%d aprovado. Iniciando processo de entrega...\n", order.ID)

			time.Sleep(3 * time.Second)
			log.Printf("Entrega concluída para pedido ID=%d\n", order.ID)

			order.Status = "DELIVERED"
			_, err = db.Exec("UPDATE orders SET status=$1 WHERE id=$2", order.Status, order.ID)
			if err != nil {
				log.Println("Erro ao atualizar status no DB:", err)
			}
		}
	}()

	select {}
}
