package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/sust-cse/queuestorm-investigator/internal/adjudicator"
	"github.com/sust-cse/queuestorm-investigator/internal/analyzer"
	"github.com/sust-cse/queuestorm-investigator/internal/api"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	service := api.NewServerWithAdjudicator(analyzer.New(), adjudicator.NewFromEnv())

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           service.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("queuestorm investigator listening on 0.0.0.0:%s", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
}
