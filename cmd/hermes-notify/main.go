package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/notifications"
	"github.com/hashicorp-forge/hermes/pkg/notifications/backends"
	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/twmb/franz-go/pkg/kgo"
)

// NotifierConfig holds the notifier configuration from HCL
type NotifierConfig struct {
	Brokers       string `hcl:"brokers,optional"`
	Topic         string `hcl:"topic,optional"`
	ConsumerGroup string `hcl:"consumer_group,optional"`

	// Backends configuration
	Backends *backends.Config `hcl:"backends,block"`
}

func main() {
	// Parse command-line flags
	configFile := flag.String("config", "", "Path to HCL configuration file")
	flag.Parse()

	if *configFile == "" {
		log.Fatal("Missing required -config flag")
	}

	// Load configuration from HCL file
	var cfg NotifierConfig
	err := hclsimple.DecodeFile(*configFile, nil, &cfg)
	if err != nil {
		log.Fatalf("Failed to load configuration from %s: %v", *configFile, err)
	}

	// Apply defaults
	if cfg.Brokers == "" {
		cfg.Brokers = "localhost:9092"
	}
	if cfg.Topic == "" {
		cfg.Topic = "hermes.notifications"
	}
	if cfg.ConsumerGroup == "" {
		cfg.ConsumerGroup = "hermes-notifiers"
	}

	// Initialize backend registry from configuration
	registry, err := backends.NewRegistry(cfg.Backends)
	if err != nil {
		log.Fatalf("Failed to initialize backend registry: %v", err)
	}

	backendList := registry.GetAll()
	if len(backendList) == 0 {
		log.Fatal("No backends initialized")
	}

	// Create Kafka consumer
	client, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers),
		kgo.ConsumerGroup(cfg.ConsumerGroup),
		kgo.ConsumeTopics(cfg.Topic),
	)
	if err != nil {
		log.Fatalf("Failed to create consumer: %v", err)
	}
	defer client.Close()

	// Setup signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	backendNames := registry.GetBackendNames()
	log.Printf("Starting notification worker (backends=%v, group=%s)\n", backendNames, cfg.ConsumerGroup)

	// RFC-087-ADDENDUM Section 7: Graceful Shutdown
	// Track in-flight messages for graceful shutdown
	var inFlight sync.WaitGroup
	shutdownTimeout := 30 * time.Second

	// Consume messages
	for {
		select {
		case <-ctx.Done():
			log.Println("Shutdown signal received, waiting for in-flight messages...")

			// Wait for in-flight messages with timeout
			done := make(chan struct{})
			go func() {
				inFlight.Wait()
				close(done)
			}()

			select {
			case <-done:
				log.Println("All in-flight messages completed")
			case <-time.After(shutdownTimeout):
				log.Printf("Shutdown timeout (%v) reached, some messages may be incomplete", shutdownTimeout)
			}

			log.Println("Shutting down notification worker")
			return

		default:
			fetches := client.PollFetches(ctx)
			if errs := fetches.Errors(); len(errs) > 0 {
				for _, err := range errs {
					log.Printf("Fetch error: %v\n", err)
				}
				continue
			}

			fetches.EachPartition(func(p kgo.FetchTopicPartition) {
				for _, record := range p.Records {
					// Track message processing
					inFlight.Add(1)
					go func(rec *kgo.Record) {
						defer inFlight.Done()

						if err := processMessage(ctx, backendList, rec); err != nil {
							log.Printf("Failed to process message: %v\n", err)
							// Don't commit offset on failure (RFC-087-ADDENDUM Section 9)
						} else {
							// Commit offset after successful processing
							client.CommitRecords(ctx, rec)
						}
					}(record)
				}
			})
		}
	}
}

func processMessage(ctx context.Context, backends []backends.Backend, record *kgo.Record) error {
	// Parse notification message
	var msg notifications.NotificationMessage
	if err := json.Unmarshal(record.Value, &msg); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	// Check if this notifier should process this message
	// Filter messages based on configured backends to avoid head-of-queue blocking
	shouldProcess := false
	for _, backend := range backends {
		for _, targetBackend := range msg.Backends {
			if backend.SupportsBackend(targetBackend) {
				shouldProcess = true
				break
			}
		}
		if shouldProcess {
			break
		}
	}

	if !shouldProcess {
		log.Printf("Skipping message %s (backends=%v, not handled by this notifier)", msg.ID, msg.Backends)
		return nil
	}

	log.Printf("Processing message: id=%s template=%s backends=%v", msg.ID, msg.Template, msg.Backends)

	// Route to appropriate backends based on message.Backends field
	for _, backend := range backends {
		for _, targetBackend := range msg.Backends {
			if backend.SupportsBackend(targetBackend) {
				if err := backend.Handle(ctx, &msg); err != nil {
					log.Printf("backend %s failed: %v", backend.Name(), err)
					// Continue with other backends
				} else {
					log.Printf("backend %s processed message %s", backend.Name(), msg.ID)
				}
			}
		}
	}

	return nil
}
