package weaviate

import (
	"context"
	"fmt"
	"saas-api/cmd/configs"

	"github.com/weaviate/weaviate-go-client/v5/weaviate"
)

func NewWeaviateClient(config *configs.Config) *WeaviateClient {
	// Build host with port - weaviate-go-client expects "host:port" format
	host := config.WeaviateHost
	if host == "" {
		host = "localhost"
	}
	port := config.WeaviatePort
	if port == "" {
		port = "7080"
	}
	hostWithPort := fmt.Sprintf("%s:%s", host, port)

	scheme := config.WeaviateScheme
	if scheme == "" {
		scheme = "http"
	}

	cfg := weaviate.Config{
		Host:   hostWithPort,
		Scheme: scheme,
	}

	client, err := weaviate.NewClient(cfg)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize weaviate client: %v (connecting to %s://%s)", err, scheme, hostWithPort))
	}

	// Check if Weaviate is ready
	ready, err := client.Misc().ReadyChecker().Do(context.Background())
	if !ready {
		if err != nil {
			panic(fmt.Sprintf("Weaviate is not ready at %s://%s: %v", scheme, hostWithPort, err))
		}
		panic(fmt.Sprintf("Weaviate is not ready at %s://%s", scheme, hostWithPort))
	}

	return &WeaviateClient{
		Client: client,
	}
}
