package main

import (
	"log"
	"net/http"
	"os"

	"github.com/rendis/senda/internal/teststack/awssim"
)

func main() {
	backendURL := os.Getenv("AWS_SIM_BACKEND_URL")
	if backendURL == "" {
		log.Fatal("AWS_SIM_BACKEND_URL is required")
	}

	bridge, err := awssim.NewBridge(awssim.Config{
		BackendBaseURL:  backendURL,
		Region:          os.Getenv("AWS_SIM_REGION"),
		AccessKeyID:     os.Getenv("AWS_SIM_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("AWS_SIM_SECRET_ACCESS_KEY"),
	})
	if err != nil {
		log.Fatalf("create aws-sim bridge: %v", err)
	}

	addr := os.Getenv("AWS_SIM_LISTEN_ADDR")
	if addr == "" {
		addr = ":4566"
	}

	log.Printf("aws-sim bridge listening on %s -> %s", addr, backendURL)
	log.Fatal(http.ListenAndServe(addr, bridge.Handler()))
}
