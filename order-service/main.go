package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/streadway/amqp"

	"github.com/redis/go-redis/v9"
)

type Order struct {
	ID       int64  `json:"id"`
	Customer string `json:"customer"`
	Product  string `json:"product"`
	Status   string `json:"status"`
}

var (
	db     *sql.DB
	rdb    *redis.Client
	amqpCh *amqp.Channel
	ctx    = context.Background()
)

func initDB() (*sql.DB, error) {
	connStr := os.Getenv("DB_CONN_STRING")
	if connStr == "" {
		connStr = "postgres://user:password@localhost:5432/mydb?sslmode=disable"
	}
	return sql.Open("postgres", connStr)
}

func initRedis() *redis.Client {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	return redis.NewClient(&redis.Options{
		Addr: addr,
	})
}

func initRabbitMQ() (*amqp.Connection, *amqp.Channel, error) {
	amqpURL := os.Getenv("RABBITMQ_URL")
	if amqpURL == "" {
		amqpURL = "amqp://guest:guest@localhost:5672/"
	}
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		return nil, nil, err
	}
	_, err = ch.QueueDeclare(
		"orders.new",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, nil, err
	}
	return conn, ch, nil
}

func createOrder(w http.ResponseWriter, r *http.Request) {
	var o Order
	if err := json.NewDecoder(r.Body).Decode(&o); err != nil {
		http.Error(w, "Erro ao decodificar JSON", http.StatusBadRequest)
		return
	}
	o.Status = "CREATED"

	err := db.QueryRow(`
		INSERT INTO orders (customer, product, status) 
		VALUES ($1, $2, $3) RETURNING id
	`, o.Customer, o.Product, o.Status).Scan(&o.ID)
	if err != nil {
		http.Error(w, "Erro ao inserir no DB: "+err.Error(), http.StatusInternalServerError)
		return
	}

	orderKey := "order:" + strconv.FormatInt(o.ID, 10)
	orderData, _ := json.Marshal(o)
	rdb.Set(ctx, orderKey, string(orderData), 30*time.Minute)

	body, _ := json.Marshal(o)
	err = amqpCh.Publish(
		"",
		"orders.new",
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
	if err != nil {
		log.Println("Erro ao publicar na fila:", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(o)
}

func getOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderID := vars["id"]

	orderKey := "order:" + orderID
	if cached, err := rdb.Get(ctx, orderKey).Result(); err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(cached))
		return
	}

	var o Order
	err := db.QueryRow("SELECT id, customer, product, status FROM orders WHERE id = $1", orderID).
		Scan(&o.ID, &o.Customer, &o.Product, &o.Status)
	if err != nil {
		http.Error(w, "Pedido não encontrado", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(o)
}

func main() {
	var err error
	db, err = initDB()
	if err != nil {
		log.Fatal("Erro ao conectar DB:", err)
	}
	defer db.Close()

	rdb = initRedis()
	defer rdb.Close()

	connRabbit, chRabbit, err := initRabbitMQ()
	if err != nil {
		log.Fatal("Erro ao conectar RabbitMQ:", err)
	}
	defer connRabbit.Close()
	defer chRabbit.Close()

	amqpCh = chRabbit

	router := mux.NewRouter()
	router.HandleFunc("/orders", createOrder).Methods("POST")
	router.HandleFunc("/orders/{id}", getOrder).Methods("GET")
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK - order-service"))
	}).Methods("GET")

	log.Println("Order Service rodando na porta 8080...")
	log.Fatal(http.ListenAndServe(":8080", router))
}
