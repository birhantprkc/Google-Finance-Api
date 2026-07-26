package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kilimcininkoroglu/google-finance-api/internal/api"
	"github.com/kilimcininkoroglu/google-finance-api/internal/gfrpc"
	"github.com/kilimcininkoroglu/google-finance-api/web"
)

func main() {
	// PORT is operator-controlled, but validate it is purely numeric so the
	// value logged and bound is never tainted by unexpected input (CWE-117).
	port := os.Getenv("PORT")
	if !isNumericPort(port) {
		port = "8080"
	}

	client := gfrpc.NewClient()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := api.NewServer(ctx, client, port, web.Content)

	go func() {
		// port is validated numeric by isNumericPort above; gosec cannot follow
		// the custom validator, so this is a verified false positive.
		log.Printf("server starting on :%s", port) // #nosec G706
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}
	log.Println("server stopped")
}

// isNumericPort reports whether s is a non-empty string of ASCII digits.
func isNumericPort(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
