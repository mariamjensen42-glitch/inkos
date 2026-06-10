// Package main is the inkos-api HTTP server entry point.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/narcooo/inkos/internal/api"
)

func main() {
	var (
		root  = flag.String("root", ".", "Project root directory containing inkos.json")
		host  = flag.String("host", "127.0.0.1", "Listen host")
		port  = flag.Int("port", 4567, "Listen port")
		mode  = flag.String("mode", "release", "Gin mode: debug | release | test")
	)
	flag.Parse()

	if abs, err := filepath.Abs(*root); err == nil {
		*root = abs
	}
	if _, err := os.Stat(*root); err != nil {
		log.Fatalf("project root not accessible: %v", err)
	}

	gin.SetMode(*mode)
	srv := api.NewServer(*root)
	srv.WithResolverFactory(srv.DefaultResolverFactory())
	engine := srv.Engine()

	addr := fmt.Sprintf("%s:%d", *host, *port)
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Graceful shutdown.
	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint
		log.Println("shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(ctx); err != nil {
			log.Printf("http shutdown: %v", err)
		}
		close(idleConnsClosed)
	}()

	log.Printf("inkos-api listening on http://%s (root=%s)", addr, *root)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("listen: %v", err)
	}
	<-idleConnsClosed
}
