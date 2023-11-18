package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	internalMiddleware "github.com/meziaris/golang-gracefull-shutdown/middleware"
)

var STATUS atomic.Int32
var KUBE_PERIOD_SECONDS = 2
var KUBE_FAILURE_THRESHOLD = 2
var DELTA_SECONDS = 2
var WAIT_SECONDS = (KUBE_FAILURE_THRESHOLD*KUBE_PERIOD_SECONDS + DELTA_SECONDS)

func main() {
	r := chi.NewRouter()
	r.Use(internalMiddleware.Heartbeat("/health", &STATUS))
	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})

	srv := &http.Server{
		Addr:    ":3333",
		Handler: r,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		time.AfterFunc(1*time.Second, func() {
			STATUS.Add(1)
		})
		log.Println("starting server on port 3333")
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			// Error starting or closing listener:
			log.Fatalf("listen and serve returned err: %v", err)
		}
	}()

	<-ctx.Done()
	// We received an interrupt signal, shut down.
	STATUS.Store(0) // Start to fail readiness probes
	log.Printf("received interrupt signal, http server will shut down in %v seconds", WAIT_SECONDS)
	time.Sleep(time.Duration(WAIT_SECONDS) * time.Second)

	log.Println("gracefully shutting down...")
	if err := srv.Shutdown(context.TODO()); err != nil {
		log.Printf("server shutdown returned an err: %v\n", err)
	}

	log.Println("server gracefully stopped")
}
