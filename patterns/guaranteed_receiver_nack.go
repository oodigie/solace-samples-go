package main

import (
	"fmt"
	"os"
	"os/signal"
	"time"

	"solace.dev/go/messaging"
	"solace.dev/go/messaging/pkg/solace/config"
	"solace.dev/go/messaging/pkg/solace/message"
	"solace.dev/go/messaging/pkg/solace/resource"
)

func getEnv(key, def string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return def
}

func main() {
	// logging.SetLogLevel(logging.LogLevelInfo)

	// Configuration parameters
	brokerConfig := config.ServicePropertyMap{
		config.TransportLayerPropertyHost:                getEnv("SOLACE_HOST", "tcp://localhost:55555,tcp://localhost:55554"),
		config.ServicePropertyVPNName:                    getEnv("SOLACE_VPN", "default"),
		config.AuthenticationPropertySchemeBasicPassword: getEnv("SOLACE_PASSWORD", "default"),
		config.AuthenticationPropertySchemeBasicUserName: getEnv("SOLACE_USERNAME", "default"),
	}

	messagingService, err := messaging.NewMessagingServiceBuilder().FromConfigurationProvider(brokerConfig).Build()

	if err != nil {
		panic(err)
	}

	// Connect to the messaging serice
	if err := messagingService.Connect(); err != nil {
		panic(err)
	}

	fmt.Println("Connected to the broker? ", messagingService.IsConnected())

	if err != nil {
		panic(err)
	}

	queueName := "durable-queue"
	durableExclusiveQueue := resource.QueueDurableExclusive(queueName)

	// Build a Gauranteed message receiver with NACK support and bind to the given queue
	// Set the required message settlement outcome(s) on the flow using the WithRequiredMessageOutcomeSupport() builder method
	persistentReceiver, err := messagingService.CreatePersistentMessageReceiverBuilder().
		WithMessageClientAcknowledgement().
		// Add message settlement outcomes support on the created Flow here (Failed and Reject for NACK(ing) messages)
		WithRequiredMessageOutcomeSupport(config.PersistentReceiverFailedOutcome, config.PersistentReceiverRejectedOutcome).
		Build(durableExclusiveQueue)

	// // Build a Gauranteed message receiver with NACK support and bind to the given queue
	// // Set the required message settlement outcome(s) on the flow using the configuration provider
	// persistentReceiver, err := messagingService.CreatePersistentMessageReceiverBuilder().
	// 	WithMessageClientAcknowledgement().
	// 	// Add message settlement outcomes support on the created Flow here (Failed and Reject for NACK(ing) messages)
	// 	FromConfigurationProvider(config.ReceiverPropertyMap{
	// 		config.ReceiverPropertyPersistentMessageRequiredOutcomeSupport: fmt.Sprintf("%s,%s",config.PersistentReceiverFailedOutcome, config.PersistentReceiverRejectedOutcome),
	// 	}).
	// 	Build(durableExclusiveQueue)

	// Handling a panic from a non existing queue
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("Make sure queue name '%s' exists on the broker.\nThe following error occurred when attempting to connect to create a Persistent Message Receiver:\n%s", queueName, err)
		}
	}()

	// Start Persistent Message Receiver
	if err := persistentReceiver.Start(); err != nil {
		panic(err)
	}

	fmt.Println("Persistent Receiver running? ", persistentReceiver.IsRunning())

	// Message Handler
	messageHandler := func(message message.InboundMessage) {
		var messageBody string

		if payload, ok := message.GetPayloadAsString(); ok {
			messageBody = payload
		} else if payload, ok := message.GetPayloadAsBytes(); ok {
			messageBody = string(payload)
		}

		fmt.Printf("Received Message Body %s \n", messageBody)
		// fmt.Printf("Message Dump %s \n", message)

		// Settle the message here with one of the three supported settlement outcomes: ACCEPTED, FAILED and REJECTED
		messageSettlementError := persistentReceiver.Settle(message, config.PersistentReceiverAcceptedOutcome) // Accept(acknowlegde) the message
		// messageSettlementError := persistentReceiver.Settle(message, config.PersistentReceiverFailedOutcome) // fail the message
		// messageSettlementError := persistentReceiver.Settle(message, config.PersistentReceiverRejectedOutcome) // reject the message
		fmt.Println("Message Settlement Error: ", messageSettlementError)
	}

	// Register Message callback handler to the Message Receiver
	if regErr := persistentReceiver.ReceiveAsync(messageHandler); regErr != nil {
		panic(regErr)
	}
	fmt.Printf("\n Bound to queue: %s\n", queueName)
	fmt.Println("\n===Interrupt (CTR+C) to handle graceful termination of the receiver===\n")

	// Run forever until an interrupt signal is received
	// Handle interrupts

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	// Block until a signal is received.
	<-c

	// Terminate the Persistent Receiver
	persistentReceiver.Terminate(1 * time.Second)
	fmt.Println("\nPersistent Receiver Terminated? ", persistentReceiver.IsTerminated())
	// Disconnect the Message Service
	messagingService.Disconnect()
	fmt.Println("Messaging Service Disconnected? ", !messagingService.IsConnected())

}
