package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/ErwinSalas/go-eda/common/broker"
	"github.com/ErwinSalas/go-eda/common/queue"
	"github.com/ErwinSalas/go-eda/services/order-service/pkg/api"
	"github.com/ErwinSalas/go-eda/services/order-service/pkg/app"
	"github.com/ErwinSalas/go-eda/services/order-service/pkg/datastore"
)

func initDb() (*sql.DB, error) {
	dbName := os.Getenv("MYSQL_DATABASE")
	dbUser := os.Getenv("MYSQL_USER")
	dbPassword := os.Getenv("MYSQL_PASSWORD")
	dbhost := os.Getenv("MYSQL_HOST")

	if dbName == "" || dbUser == "" || dbPassword == "" {
		log.Fatal("missing required environment variables")
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:3306)/%s", dbUser, dbPassword, dbhost, dbName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("error opening connection %v", err)
		return nil, err
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("db ping error : %v", err)
	}

	return db, nil
}

func main() {
	db, err := initDb()

	if err != nil {
		log.Fatal("Error initializing database")
		return
	}
	defer db.Close()

	mysqlDatastore, err := datastore.NewDataStore(db)
	if err != nil {
		fmt.Println("Error creating datastore:", err)
		return
	}

	awsRegion := os.Getenv("AWS_REGION")
	localstackEndpoint := os.Getenv("LOCALSTACK_ENDPOINT")
	ordersTopic := os.Getenv("SNS_ORDER_TOPIC_ARN")
	paymentsTopic := os.Getenv("SNS_PAYMENT_TOPIC_ARN")

	paymentsQueue := os.Getenv("PAYMENTS_QUEUE_URL")

	orderPublisher, err := broker.NewSNSPublisherAWS(ordersTopic, awsRegion, localstackEndpoint)
	if err != nil {
		fmt.Println("Error creating publisher:", err)
		return
	}

	paymentSubscriber, err := broker.NewSNSSubscriberAWS(paymentsTopic, awsRegion, localstackEndpoint)
	if err != nil {
		fmt.Println("Error subscribing to SNS topic:", err)
		return
	}

	err = paymentSubscriber.Subscribe(paymentsQueue, "sqs")
	if err != nil {
		fmt.Println("Error subscribing to SNS topic:", err)
		return
	}

	paymentsConsumer, err := queue.NewSQSService(paymentsQueue, awsRegion, localstackEndpoint)

	if err != nil {
		fmt.Println("Error subscribing to SNS topic:", err)
		return
	}

	ctx := context.Background()

	mysqlDatastore.Migrate(ctx)
	app := app.NewApp(mysqlDatastore, orderPublisher, paymentsConsumer)

	api.NewRouter(3001, *app)
}
