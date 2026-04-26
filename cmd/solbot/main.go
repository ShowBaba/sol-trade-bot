package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"

	"github.com/ShowBaba/tradebot/internal/agent"
	"github.com/ShowBaba/tradebot/internal/executor"
	"github.com/ShowBaba/tradebot/internal/jupiter"
	"github.com/ShowBaba/tradebot/internal/logbus"
	"github.com/ShowBaba/tradebot/internal/solana"
	"github.com/ShowBaba/tradebot/internal/store"
	"github.com/ShowBaba/tradebot/internal/token"
	"github.com/ShowBaba/tradebot/internal/ui"
	"github.com/ShowBaba/tradebot/internal/wallet"
)

func main() {
	_ = godotenv.Load()

	bus := logbus.New(1000)
	jup := jupiter.New()
	rpc := solana.NewRPCClient("")
	tokens := token.NewService(rpc)

	var stateBackend agent.StoreBackend
	var redisLogStore *store.RedisLogStore
	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		redisDB := -1
		if s := os.Getenv("REDIS_DB"); s != "" {
			if d, err := strconv.Atoi(s); err == nil && d >= 0 && d <= 15 {
				redisDB = d
			}
		}
		client, err := store.NewRedisClient(redisURL, redisDB)
		if err != nil {
			log.Fatalf("Redis: %v", err)
		}
		stateBackend = store.NewRedisBackendWithClient(client)
		redisLogStore = store.NewRedisLogStore(client)
		bus.SetPersister(redisLogStore.Append)
		if redisDB >= 0 {
			log.Printf("State, logs, and history persisted to Redis (db=%d)", redisDB)
		} else {
			log.Printf("State, logs, and history persisted to Redis")
		}
	} else {
		stateBackend = &agent.FileBackend{Path: "data/state.json"}
		log.Printf("State persisted to data/state.json")
	}
	mgr := agent.NewManager(jup, bus, stateBackend)

	var walletProvider ui.WalletProvider
	if w, err := wallet.LoadFromEnv(); err == nil {
		walletProvider = &walletInfoProvider{w: w, rpc: rpc}
		log.Printf("Wallet loaded: %s…%s", w.PublicKeyBase58()[:4], w.PublicKeyBase58()[len(w.PublicKeyBase58())-4:])
		liveCfg := executor.LiveConfig{
			MaxRetries:      3,
			ConfirmTimeout:  60 * time.Second,
			Commitment:      "confirmed",
			DryRun:          os.Getenv("DRY_RUN_LIVE") == "1" || os.Getenv("DRY_RUN_LIVE") == "true",
		}
		if s := os.Getenv("CONFIRM_TIMEOUT_SEC"); s != "" {
			if sec, _ := strconv.Atoi(s); sec > 0 {
				liveCfg.ConfirmTimeout = time.Duration(sec) * time.Second
			}
		}
		validatePosition := func(ctx context.Context, walletPubkey, targetMint string, expectedTargetQty uint64) (ok bool, err error) {
			ata, err := solana.GetAssociatedTokenAddress(walletPubkey, targetMint)
			if err != nil {
				return false, err
			}
			bal, err := rpc.GetTokenAccountBalance(ctx, ata)
			if err != nil {
				return false, err
			}
			return bal >= expectedTargetQty, nil
		}
		mgr.SetLiveDeps(w, rpc, liveCfg, validatePosition)
		log.Printf("Live mode enabled (wallet loaded); dry-run=%v", liveCfg.DryRun)
	} else {
		log.Printf("Wallet not loaded: %v (set WALLET_KEYPAIR_PATH or WALLET_KEYPAIR_BASE64 in .env)", err)
	}

	mgr.Load()

	srv := ui.New(mgr, bus, tokens, walletProvider)
	if redisLogStore != nil {
		srv.SetLogStore(redisLogStore)
	}
	if staticDir := os.Getenv("STATIC_DIR"); staticDir != "" {
		srv.SetStaticDir(staticDir)
		log.Printf("Serving UI from %s", staticDir)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	log.Printf("TradeBot API listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, srv.Routes()); err != nil {
		log.Fatal(err)
	}
}

// walletInfoProvider implements ui.WalletProvider for GET /api/wallet.
type walletInfoProvider struct {
	w   *wallet.Wallet
	rpc *solana.RPCClient
}

func (p *walletInfoProvider) PublicKey() string {
	return p.w.PublicKeyBase58()
}

func (p *walletInfoProvider) Balance(ctx context.Context) (uint64, error) {
	return p.rpc.GetBalance(ctx, p.w.PublicKeyBase58())
}
