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

	"cardwars/internal/auth"
	"cardwars/internal/server"
	"cardwars/internal/store"
)

func main() {
	var (
		addr     = flag.String("addr", ":8080", "listen address")
		data     = flag.String("data", "data", "directory with cards/ and effects/")
		delay    = flag.Duration("bot-delay", 700*time.Millisecond, "pause between the bot's moves")
		origins  = flag.String("origins", "", "comma-separated extra browser origins allowed to open a WebSocket (e.g. localhost:*)")
		dbPath   = flag.String("db", os.Getenv("CARDWARS_DB"), "SQLite file for accounts and decks; empty keeps nothing [$CARDWARS_DB]")
		authOpt  = flag.String("auth", envOr("CARDWARS_AUTH", "none"), "who may sign in: none, or oauth (Discord and GitHub; needs -db and -public-url) [$CARDWARS_AUTH]")
		public   = flag.String("public-url", os.Getenv("CARDWARS_PUBLIC_URL"), "where browsers reach this server, e.g. https://cards.example.com (for -auth oauth) [$CARDWARS_PUBLIC_URL]")
		feedback = flag.Bool("feedback", envBool("CARDWARS_FEEDBACK"), "show links to the project's bug, card and feature forms and its repository on the home screen [$CARDWARS_FEEDBACK]")
		dev      = flag.Bool("dev", false, "development mode: clients may choose seeds and play unmodelled cards")
	)
	flag.Parse()
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cfg := server.Config{DataDir: *data, BotDelay: *delay, Feedback: *feedback, AllowSeed: *dev, AllowUnmodelled: *dev, Logger: log}
	if *origins != "" {
		cfg.OriginPatterns = strings.Split(*origins, ",")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var db *store.SQLite
	if *dbPath != "" {
		var err error
		if db, err = store.OpenSQLite(*dbPath); err != nil {
			log.Error("open database", "err", err)
			os.Exit(1)
		}
		defer db.Close()
		log.Info("database ready", "path", *dbPath)
	}
	switch *authOpt {
	case "none":
	case "oauth":
		if db == nil {
			log.Error("-auth oauth needs -db")
			os.Exit(1)
		}
		// Secrets come from the environment, not flags, so they stay out of process listings.
		oa, err := auth.NewOAuth(auth.OAuthOptions{
			Store: db, PublicURL: *public, Logger: log, ReturnHosts: cfg.OriginPatterns,
			Credentials: map[string]auth.Credentials{
				"discord": {ClientID: os.Getenv("CARDWARS_DISCORD_CLIENT_ID"), ClientSecret: os.Getenv("CARDWARS_DISCORD_CLIENT_SECRET")},
				"github":  {ClientID: os.Getenv("CARDWARS_GITHUB_CLIENT_ID"), ClientSecret: os.Getenv("CARDWARS_GITHUB_CLIENT_SECRET")},
			},
		})
		if err != nil {
			log.Error("sign-in", "err", err)
			os.Exit(1)
		}
		cfg.Auth = oa
		go oa.Run(ctx)
		log.Info("sign-in on", "public_url", *public)
	default:
		log.Error("-auth must be none or oauth", "got", *authOpt)
		os.Exit(1)
	}

	if db != nil { // a nil *SQLite inside the interface would not be nil
		cfg.Store = db
	}
	srv, err := server.New(cfg)
	if err != nil {
		log.Error("start", "err", err)
		os.Exit(1)
	}
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

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

// envBool reads a yes/no setting: 1, true, yes or on (any case) mean yes.
func envBool(name string) bool {
	switch strings.ToLower(os.Getenv(name)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
