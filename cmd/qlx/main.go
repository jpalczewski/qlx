package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/erxyi/qlx/internal/app"
	qlprint "github.com/erxyi/qlx/internal/print"
	"github.com/erxyi/qlx/internal/print/encoder/brother"
	"github.com/erxyi/qlx/internal/print/encoder/niimbot"
	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
)

func init() {
	debug.SetMemoryLimit(16 * 1024 * 1024)
	debug.SetGCPercent(20)
}

func main() {
	device := flag.String("device", "/dev/usb/lp0", "printer device path")
	port := flag.String("port", "8080", "server port")
	host := flag.String("host", "0.0.0.0", "host to bind")
	dataDir := flag.String("data", "./data", "data directory for JSON store")
	flag.Parse()

	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create data directory: %v\n", err)
		os.Exit(1)
	}

	storePath := filepath.Join(*dataDir, "data.json")
	s, err := store.NewStore(storePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load store: %v\n", err)
		os.Exit(1)
	}

	ps := qlprint.NewPrintService(s)
	ps.RegisterEncoder(&brother.BrotherEncoder{})
	ps.RegisterEncoder(&niimbot.NiimbotEncoder{})

	server := app.NewServer(s, ps)

	addr := fmt.Sprintf("%s:%s", *host, *port)
	srv := &http.Server{
		Addr:    addr,
		Handler: server,
	}

	go func() {
		webutil.LogInfo("QLX starting on %s (device: %s, data: %s)", addr, *device, *dataDir)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	webutil.LogInfo("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "shutdown error: %v\n", err)
		os.Exit(1)
	}

	webutil.LogInfo("server stopped")
}
