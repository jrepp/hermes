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

	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/hashicorp-forge/hermes/pkg/notifications"
	"github.com/hashicorp-forge/hermes/pkg/notifications/backends"
)

// NotifierConfig holds the notifier configuration from HCL
type NotifierConfig struct {
	// Backends configuration (pointer - 8 bytes on 64-bit)
	Backends *backends.Config `hcl:"backends,block"`

	// Strings (16 bytes each on 64-bit due to struct layout)
	Brokers       string `hcl:"brokers,optional"`
	Topic         string `hcl:"topic,optional"`
	ConsumerGroup string `hcl:"consumer_group,optional"`
}

func main() {
	cfg := loadConfig()
	client := setupKafkaClient(cfg)
	defer client.Close()

	registry := initializeBackends(cfg)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	runConsumer(ctx, client, registry)
}

func loadConfig() NotifierConfig {
	configFile := flag.String("config", "", "Path to HCL configuration file")
	flag.Parse()

	if *configFile == "" {
		log.Fatal("Missing required -config flag")
	}

	var cfg NotifierConfig
	err := hclsimple.DecodeFile(*configFile, nil, &cfg)
	if err != nil {
		log.Fatalf("Failed to load configuration from %s: %v", *configFile, err)
	}

	applyDefaults(&cfg)
	return cfg
}

func applyDefaults(cfg *NotifierConfig) {
	if cfg.Brokers == "" {
		cfg.Brokers = "localhost:9092"
	}
	if cfg.Topic == "" {
		cfg.Topic = "hermes.notifications"
	}
	if cfg.ConsumerGroup == "" {
		cfg.ConsumerGroup = "hermes-notifiers"
	}
}

func setupKafkaClient(cfg NotifierConfig) *kgo.Client {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers),
		kgo.ConsumerGroup(cfg.ConsumerGroup),
		kgo.ConsumeTopics(cfg.Topic),
	)
	if err != nil {
		log.Fatalf("Failed to create consumer: %v", err)
	}
	return client
}

func initializeBackends(cfg NotifierConfig) *backends.Registry {
	registry, err := backends.NewRegistry(cfg.Backends)
	if err != nil {
		log.Fatalf("Failed to initialize backend registry: %v", err)
	}

	backendList := registry.GetAll()
	if len(backendList) == 0 {
		log.Fatal("No backends initialized")
	}

	backendNames := registry.GetBackendNames()
	log.Printf("Starting notification worker (backends=%v, group=%s)\n", backendNames, cfg.ConsumerGroup)

	return registry
}

func runConsumer(ctx context.Context, client *kgo.Client, registry *backends.Registry) {
	var inFlight sync.WaitGroup
	backendList := registry.GetAll()

	for {
		select {
		case <-ctx.Done():
			handleGracefulShutdown(&inFlight)
			return
		default:
			processFetches(ctx, client, backendList, &inFlight)
		}
	}
}

func handleGracefulShutdown(inFlight *sync.WaitGroup) {
	log.Println("Shutdown signal received, waiting for in-flight messages...")
	shutdownTimeout := 30 * time.Second

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
}

func processFetches(ctx context.Context, client *kgo.Client, backendList []backends.Backend, inFlight *sync.WaitGroup) {
	fetches := client.PollFetches(ctx)
	if errs := fetches.Errors(); len(errs) > 0 {
		for _, err := range errs {
			log.Printf("Fetch error: %v\n", err)
		}
		return
	}

	fetches.EachPartition(func(p kgo.FetchTopicPartition) {
		for _, record := range p.Records {
			inFlight.Add(1)
			go processRecord(ctx, client, backendList, record, inFlight)
		}
	})
}

func processRecord(ctx context.Context, client *kgo.Client, backendList []backends.Backend, record *kgo.Record, inFlight *sync.WaitGroup) {
	defer inFlight.Done()

	if err := processMessage(ctx, backendList, record); err != nil {
		log.Printf("Failed to process message: %v\n", err)
		// Don't commit offset on failure (RFC-087-ADDENDUM Section 9)
	} else {
		// Commit offset after successful processing
		if err := client.CommitRecords(ctx, record); err != nil {
			log.Printf("Failed to commit record offset: %v\n", err)
		}
	}
}

func processMessage(ctx context.Context, backends []backends.Backend, record *kgo.Record) error {
	msg, err := parseNotificationMessage(record)
	if err != nil {
		return err
	}

	if !shouldProcessMessage(backends, &msg) {
		log.Printf("Skipping message %s (backends=%v, not handled by this notifier)", msg.ID, msg.Backends)
		return nil
	}

	log.Printf("Processing message: id=%s template=%s backends=%v", msg.ID, msg.Template, msg.Backends)
	routeToBackends(ctx, backends, &msg)

	return nil
}

func parseNotificationMessage(record *kgo.Record) (notifications.NotificationMessage, error) {
	var msg notifications.NotificationMessage
	if err := json.Unmarshal(record.Value, &msg); err != nil {
		return msg, fmt.Errorf("failed to unmarshal message: %w", err)
	}
	return msg, nil
}

func shouldProcessMessage(backends []backends.Backend, msg *notifications.NotificationMessage) bool {
	for _, backend := range backends {
		for _, targetBackend := range msg.Backends {
			if backend.SupportsBackend(targetBackend) {
				return true
			}
		}
	}
	return false
}

func routeToBackends(ctx context.Context, backends []backends.Backend, msg *notifications.NotificationMessage) {
	for _, backend := range backends {
		for _, targetBackend := range msg.Backends {
			if backend.SupportsBackend(targetBackend) {
				if err := backend.Handle(ctx, msg); err != nil {
					log.Printf("backend %s failed: %v", backend.Name(), err)
				} else {
					log.Printf("backend %s processed message %s", backend.Name(), msg.ID)
				}
			}
		}
	}
}
