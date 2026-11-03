// Command cardwarsd serves Card Wars matches over WebSocket.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"cardwars/internal/server"
)

func main() {
	var (
		addr    = flag.String("addr", ":8080", "listen address")
		data    = flag.String("data", "data", "directory with cards/ and effects/")
		delay   = flag.Duration("bot-delay", 700*time.Millisecond, "pause between the bot's moves")
		origins = flag.String("origins", "", "comma-separated extra browser origins allowed to open a WebSocket (e.g. localhost:*)")
		dev     = flag.Bool("dev", false, "development mode: clients may choose seeds and play unmodelled cards")
	)
	flag.Parse()
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cfg := server.Config{DataDir: *data, BotDelay: *delay, AllowSeed: *dev, AllowUnmodelled: *dev, Logger: log}
	if *origins != "" {
		cfg.OriginPatterns = strings.Split(*origins, ",")
	}
	srv, err := server.New(cfg)
	if err != nil {
		log.Error("start", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go srv.Run(ctx)

	hs := &http.Server{Addr: *addr, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = hs.Shutdown(sctx)
	}()
	log.Info("listening", "addr", *addr, "debug_page", "http://localhost"+*addr+"/")
	if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("serve", "err", err)
		os.Exit(1)
	}
}
