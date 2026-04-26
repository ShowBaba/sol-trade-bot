## SolBot Tradebot (Phase 2: Paper + Live)

SolBot is a Solana scalper that supports **paper trading** (simulated) and **live trading** (real Jupiter swaps with wallet signing).

### Screenshots

![SolBot web UI](web/resources/Screenshot%202026-04-26%20at%2023.31.51.png)

![SolBot web UI](web/resources/Screenshot%202026-04-26%20at%2023.32.03.png)

![SolBot web UI](web/resources/Screenshot%202026-04-26%20at%2023.42.29.png)

### Requirements

- Go 1.21+ (module declares 1.24 but any recent Go should work)

### Environment variables

- **`SOLANA_RPC_URL`**: Solana JSON-RPC endpoint for quoting, sending transactions, and fetching balances.
  - Default: `https://api.mainnet-beta.solana.com`
- **`JUPITER_API_KEY`**: Optional Jupiter API key (if you have one).
- **`PORT`**: HTTP port for the UI server.
  - Default: `8080`
- **`REDIS_URL`**: If set, **all data** is persisted in Redis and never deleted:
  - **`tradebot:agents`**: active agents (JSON array).
  - **`tradebot:deleted_agents`**: deleted agents history (JSON array).
  - **`tradebot:trades`**: full trade history (JSON array).
  - **`tradebot:logs`**: every log event is appended (list); full history is kept. “Clear logs” in the UI only clears the in-memory buffer; Redis logs are never cleared.
  Example: `redis://localhost:6379/0`. If unset, state is stored in `data/state.json` and logs are in-memory only.
- **`REDIS_DB`**: Optional. Redis database index (0–15). Overrides the DB in `REDIS_URL` when set. Example: `REDIS_DB=2`.

**Live mode only** (optional; if not set, only paper agents run):

- **`WALLET_KEYPAIR_PATH`**: Path to a JSON keypair file (array of 64 bytes). Used to sign swap transactions.
- **`WALLET_KEYPAIR_BASE64`**: Alternative: base64-encoded keypair. If both are set, `WALLET_KEYPAIR_PATH` takes precedence.
- **`DRY_RUN_LIVE`**: Set to `1` or `true` to build and sign swap transactions but **not send** them. Use this to verify the live path without spending SOL.
- **`CONFIRM_TIMEOUT_SEC`**: Timeout in seconds for transaction confirmation (default: 60).

You can create a `.env` file in the repo root (see `.env.example`); it is loaded automatically via `godotenv`.

### Docker (production)

Build and run with Docker Compose (API + UI + Redis):

```bash
cp .env.example .env
# Edit .env: set SOLANA_RPC_URL, and optionally REDIS_*, WALLET_*
docker compose up -d --build
```

- **solbot** serves the API and the built React UI on `http://localhost:8080` (or `PORT`).
- **redis** is used for state and logs when `REDIS_URL` is set (compose sets `REDIS_URL=redis://redis:6379/0`).
- Set `WALLET_KEYPAIR_PATH` or `WALLET_KEYPAIR_BASE64` in `.env` for live mode; for keypair file, mount the file: `volumes: ["./wallet.json:/app/wallet.json:ro"]` and `WALLET_KEYPAIR_PATH=/app/wallet.json`.

Build image only (e.g. for a registry):

```bash
docker build -t tradebot:latest .
docker run --env-file .env -p 8080:8080 tradebot:latest
```

### Running the API (local)

From the repo root:

```bash
go run ./cmd/solbot
```

or with the provided `Makefile`:

```bash
make run
```

This starts the HTTP API on `http://localhost:8080` (or the port you configured).

### Running the React UI (recommended)

The React UI lives under `web/` and talks to the Go server via JSON/SSE APIs.

In one terminal, run the Go API as above. In another terminal:

```bash
cd web
npm install        # first time only
npm run dev        # starts Vite dev server on http://localhost:5173
```

The Vite dev server is configured to **proxy `/api`** requests to `http://localhost:8080`, so the React app can call the Go APIs (including SSE logs) during development.

For a production build of the UI:

```bash
cd web
npm run build      # outputs static assets to web/dist
```

### React UI overview

- **Create Agent**:
  - **Mode**: Paper (simulated) or Live (real swaps; requires wallet configured).
  - Sections: Trading Settings, Timing Settings, Risk Settings. TP/SL are in percent (e.g. 5 = 5%); validated to [0, 100]. Advanced (collapsible) includes max impact, consecutive/total loss limits, and liquidity gate options.
  - A “Computed values sent to backend” preview shows tradeSizeLamports, maxSlippageBps, pollSeconds, cooldownSeconds, and TP/SL %.
- **Header**: When a wallet is configured, shows truncated wallet pubkey and SOL balance. If any agent is Live, a warning banner is shown (“Real trades enabled”).
- **Agents panel**:
  - Shows base/target mints, trade size, TP/SL, status (including “needs_manual_attention” for failed live restore), PnL, consecutive losses. Live agents show a “Live” badge. Entry / Exit tx links (Solscan) when available.
  - Buttons: **Start** / **Stop** / **Reset stats** / **Delete** (delete only when stopped).
- **Trades panel**:
  - Fixed-height scrollable table with sticky header. ENTER/BUY badge (green), SELL badge (red). PnL % colored. **Tx** column links to Solscan; **Fees** column shows fees in SOL when present. **Download CSV** links to `GET /api/trades.csv`.
- **Live Logs**:
  - **Clear logs**: POST `/api/logs/clear` then clears the in-memory log buffer and UI.
  - **Auto-scroll** toggle: when on, keeps scroll pinned to bottom as new logs arrive.
  - A note under the panel: “Paper positions are cleared on restart.”

### Phase 1 config (per agent)

- **Take-profit / Stop-loss**: Percent, e.g. `takeProfitPct: 5` means 5%. Must be in [0, 100]. Server rejects out-of-range values.
- **Liquidity safety gate** (optional):
  - **maxImpactPct**: Max allowed price impact (existing). Entry is skipped if quote impact exceeds this.
  - **minOutTargetSmallest**: Minimum expected output in target token smallest units. 0 = off.
  - **quoteStabilityWindow**: Number of consecutive quotes to consider (e.g. 2–3). When ≥ 2, stability is enforced.
  - **maxJitterPct**: Max allowed relative change in implied mid price between consecutive quotes (e.g. 0.5 = 0.5%). Entry is skipped with a throttled “liquidity gate” warning when impact, minOut, or stability fails.

### Restart behavior

- **Paper**: On startup, open paper positions are **not** restored; agents start stopped. Log: “Paper positions are cleared on restart.”
- **Live**: When a live agent had an open position, state is restored from `data/state.json` and the wallet’s target token balance is validated. If the balance is missing or too low, the agent is marked **needs_manual_attention** and does not start.

### API

- `GET /api/wallet`: Returns `{ "pubkey": "...", "solBalance": 1.23 }` when a wallet is configured (read-only).
- `GET /api/trades` / `GET /api/trades.csv`: Trades include `entryTxSig`, `exitTxSig`, and `feesLamports` when available (live mode).
- `POST /api/logs/clear`: Clears the in-memory log buffer only. When using Redis, persisted logs are never cleared.

### Tests

- `go test ./internal/agent/... ./internal/executor/... ./internal/ui/...`: Executor selection by mode, create-agent validation (including mode), PaperExecutor Enter/Exit, CSV format, TP/SL and quote stability.

### Live mode and safety

- **Real funds**: Live agents execute real swaps on Solana. Only use with amounts you can afford to lose.
- **Wallet**: Never commit or log `WALLET_KEYPAIR_PATH` / `WALLET_KEYPAIR_BASE64` or the keypair contents. The UI and API expose only the wallet **public key** and SOL balance.
- **Dry run**: Set `DRY_RUN_LIVE=1` to test the full live path (quote → swap build → sign) without sending transactions.
- **Risk controls**: Per-agent options (e.g. max open time, max daily loss, max trades per hour, allowlist/denylist) help limit exposure; configure them in the UI or API when creating live agents.
- **"custom program error: 0x1" (InsufficientFunds)**: The transaction failed because an account had insufficient balance. Ensure your wallet has enough SOL for (1) the trade size, (2) transaction and priority fees, and (3) rent if the swap creates a new token account (~0.002 SOL). For exits (token→SOL), ensure you hold the expected amount of the source token. Use a smaller trade size or add SOL if you see this on entry.

