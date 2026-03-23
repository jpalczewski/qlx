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
	"github.com/erxyi/qlx/internal/store/sqlite"
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
	trace := flag.Bool("trace", false, "enable trace logging (hex dump of printer communication)")
	flag.Parse()

	if err := run(*device, *port, *host, *dataDir, *trace); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run(device, port, host, dataDir string, trace bool) error {
	webutil.TraceEnabled = trace

	//nolint:gosec // G301: intentional permissions for data directory (readable by owner)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	if trace {
		//nolint:gosec // G302: intentional permissions for trace log (readable by owner)
		traceFile, err := os.OpenFile(filepath.Join(dataDir, "trace.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open trace log: %w", err)
		}
		defer func() { _ = traceFile.Close() }()
		webutil.SetTraceFile(traceFile)
	}

	// Initialize SQLite database (runs migrations)
	sqlDB, err := sqlite.New(dataDir)
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()

	// TODO(Task 12): Wire SQLiteStore as store.Store once all interfaces are implemented.
	// Using MemoryStore as temporary placeholder until Tasks 5-9 complete.
	s := store.NewMemoryStore()

	pm := qlprint.NewPrinterManager(s)
	pm.RegisterEncoder(&brother.BrotherEncoder{})
	pm.RegisterEncoder(&niimbot.NiimbotEncoder{})
	pm.Start()
	defer pm.Stop()

	server := app.NewServer(s, pm)

	addr := fmt.Sprintf("%s:%s", host, port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           server,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
	}

	go func() {
		if trace {
			webutil.LogInfo("trace logging enabled")
		}
		webutil.LogInfo("QLX starting on %s (device: %s, data: %s)", addr, device, dataDir)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	webutil.LogInfo("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}

	webutil.LogInfo("server stopped")
	return nil
}
