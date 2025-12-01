package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	ODINMarketFeed "github.com/SIPL-Dev/go-odinmarketfeedclient"
)

func main() {
	// Create a new WebSocket client
	client := ODINMarketFeed.NewODINMarketFeedClient()

	client.OnOpen = func() {
		fmt.Println("✓ WebSocket connection established successfully")
		fmt.Println("  Connected and ready to receive market data")
	}

	// Set up message handler
	client.OnMessage = func(message string) {
		fmt.Printf("📊 Market Data: %s\n", message)
		// Parse and process your market data here
	}

	// Set up error handler
	client.OnError = func(err string) {
		log.Printf("❌ Error: %s\n", err)
	}

	// Set up close handler
	client.OnClose = func(code int, reason string) {
		fmt.Printf("🔌 Connection closed: Code=%d, Reason=%s\n", code, reason)
	}

	// Configuration - Replace with your actual values
	host := "YOUR-SERVER-IP"
	port := 4509
	useSSL := false
	userID := "DEMO_TEST"

	fmt.Println("🚀 Starting Trading WebSocket Client...")
	fmt.Printf("   Host: %s:%d\n", host, port)
	fmt.Printf("   SSL: %v\n", useSSL)
	fmt.Printf("   User ID: %s\n", userID)
	fmt.Println()

	// Connect to the WebSocket server
	err := client.Connect(host, port, useSSL, userID, "")
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	// Wait a moment for connection to stabilize
	time.Sleep(2 * time.Second)

	// Set up graceful shutdown
	fmt.Println("\n✓ Client is now running. Press Ctrl+C to exit...")
	fmt.Println("  Listening for market data...\n")

	// Subscribe to touchline data for specific tokens
	// Format: "MarketSegmentID_Token"
	// Example: "1_1234" where 1 is market segment ID and 1234 is token
	tokens := []string{
		"1_22",   // Replace with actual token
		"1_2885", // Replace with actual token
	}

	fmt.Println("📡 Subscribing to touchline data...")
	//err = client.SubscribeTouchline(tokens, "0", false)
	//err = client.SubscribeTouchline(tokens, "0", true)
	//err = client.SubscribeTouchline(tokens, "1", false)
	//err = client.SubscribeTouchline(tokens, "1", true)
	//err = client.SubscribeBestFive("2885", 1) // token, marketSegmentID
	err = client.SubscribeLTPTouchline(tokens)
	// if err != nil {
	// 	log.Printf("Failed to subscribe to touchline: %v", err)
	// }

	time.Sleep(15 * time.Second)

	client.SubscribePauseResume(true)

	time.Sleep(5 * time.Second)

	client.SubscribePauseResume(false)

	// Subscribe to market depth (Best Five) for a specific token
	// fmt.Println("📊 Subscribing to market depth...")
	// err = client.SubscribeBestFive("1234", 1) // token, marketSegmentID
	// if err != nil {
	// 	log.Printf("Failed to subscribe to market depth: %v", err)
	// }

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	// Cleanup
	fmt.Println("\n🛑 Shutting down gracefully...")

	// Unsubscribe from touchline
	// fmt.Println("   Unsubscribing from touchline...")
	// err = client.UnsubscribeTouchline(tokens)
	// if err != nil {
	// 	log.Printf("Error unsubscribing from touchline: %v", err)
	// }

	// Unsubscribe from market depth
	// fmt.Println("   Unsubscribing from market depth...")
	// err = client.UnsubscribeBestFive("1234", 1)
	// if err != nil {
	// 	log.Printf("Error unsubscribing from market depth: %v", err)
	// }

	// Disconnect
	fmt.Println("   Disconnecting...")
	err = client.Disconnect()
	if err != nil {
		log.Printf("Error during disconnect: %v", err)
	}

	// Dispose resources
	client.Dispose()

	fmt.Println("✓ Shutdown complete. Goodbye!")
}
