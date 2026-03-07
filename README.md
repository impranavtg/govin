<div align="center">

<img src="./assets/logo.png" alt="govin logo" width="250" />

# govin

**Split group expenses with friends — no account, no cloud, fully local.**

_A privacy-first Splitwise alternative built in Go._

**CLI** · **Telegram Bot** · **Server Mode** · **Export/Import**

</div>

---

## Why govin?

Splitwise requires an account, has a paid tier, and stores your financial data on their servers. **govin** keeps everything on your machine in a single SQLite file — no sign-up, no subscription, no internet required.

Use it from the **terminal** when you're at your desk, or from **Telegram** when you're on the go.

---

## Features

|                            |                                                                     |
| -------------------------- | ------------------------------------------------------------------- |
| 🏷️ **Currency-aware**      | Set any currency per group — `₹`, `$`, `€`, `IDR`, anything         |
| ⚡ **Active group**        | Set a default group so you skip `--group` on every command          |
| 👤 **Auto-create members** | Members are created on the fly — no separate setup step             |
| ➗ **3 split modes**       | Equal, exact amounts, or percentage                                 |
| 🧮 **Smart settlement**    | Debt minimization — finds the fewest payments needed to settle up   |
| 📤 **Export**              | JSON or CSV — format auto-detected from file extension              |
| 📥 **Import + merge**      | UUID deduplication — safe to re-import the same file multiple times |
| 🤖 **Telegram bot**        | Manage expenses from your phone — no laptop needed                  |

---

## Table of Contents

- [Install](#install)
- [Quick Start (CLI)](#quick-start-cli)
- [CLI Commands Reference](#cli-commands-reference)
- [Telegram Bot](#telegram-bot-)
- [CLI vs Telegram — Command Cheatsheet](#cli-vs-telegram--command-cheatsheet)
- [Sharing with Friends](#sharing-with-friends)
- [Data Storage](#data-storage)

---

## Install

```bash
go install github.com/impranavtg/govin@latest
```

Make sure `~/go/bin` is in your PATH (add to `~/.zshrc` or `~/.bashrc`):

```bash
echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc
```

**Or build from source:**

```bash
git clone https://github.com/impranavtg/govin
cd govin
go build -o govin .
```

Verify installation:

```bash
govin --help
```

---

## Quick Start (CLI)

```bash
# 1. Create a group with your currency
govin group create "Goa Trip" --currency "₹"

# 2. Set it as active — skip --group on every command from now on
govin use "Goa Trip"

# 3. Add expenses — members are auto-created, no setup needed
govin add "Hotel"  --paid-by Alice   --amount 9000 --equal --with "Alice,Bob,Charlie"
govin add "Dinner" --paid-by Bob     --amount 1200 --amounts "Alice:400,Bob:400,Charlie:400"
govin add "Taxi"   --paid-by Charlie --amount 600  --percent "Alice:50,Bob:25,Charlie:25"

# 4. See who owes what
govin balance

# 5. Get the minimum payment plan
govin settle

# 6. Record payments as they happen
govin paid --from Bob --to Alice --amount 2200
```

---

## CLI Commands Reference

### `govin use` — Set active group

```bash
govin use "Goa Trip"
# All commands now work without --group flag
```

> 💡 Every command below accepts `--group "Name"` to override the active group if needed.

---

### Groups

| Command                                        | What it does                               |
| ---------------------------------------------- | ------------------------------------------ |
| `govin group create "Goa Trip" --currency "₹"` | Create a group (`--currency` is optional)  |
| `govin group list`                             | List all groups (`▶` marks the active one) |
| `govin group show "Goa Trip"`                  | Show members + expense count               |
| `govin group delete "Goa Trip"`                | Delete group and all its data              |

---

### Members

| Command                              | What it does                 |
| ------------------------------------ | ---------------------------- |
| `govin member add Alice Bob Charlie` | Add members to active group  |
| `govin member list`                  | List members in active group |

> 💡 You can skip `member add` entirely — members are auto-created when you reference them in an expense.

---

### Expenses

**Equal split** — list who to split with:

```bash
govin add "Hotel" --paid-by Alice --amount 9000 --equal --with "Alice,Bob,Charlie"
```

**Exact amounts** per person (must sum to total):

```bash
govin add "Dinner" --paid-by Bob --amount 1200 --amounts "Alice:400,Bob:400,Charlie:400"
```

**Percentage split** (must sum to 100):

```bash
govin add "Taxi" --paid-by Charlie --amount 600 --percent "Alice:50,Bob:25,Charlie:25"
```

**View and delete:**

```bash
govin list                    # List all expenses in active group
govin delete <expense-id>     # Delete by first 8 chars of ID (shown in list)
```

---

### Balances & Settlement

```bash
govin balance                                    # Net per person (green = owed, red = owes)
govin settle                                     # Minimum payments to settle all debts
govin paid --from Bob --to Alice --amount 2200   # Record a payment once made
```

---

### Export & Import

```bash
# Export — format auto-detected from file extension
govin export --output trip.json    # → JSON  (shareable, importable)
govin export --output trip.csv     # → CSV   (Excel / Google Sheets)

# Import from a friend's JSON — duplicates automatically skipped
govin import trip.json
```

---

## Telegram Bot 🤖

Don't have your laptop? Use govin from your phone via Telegram. The bot uses the **same database** as the CLI — expenses added from Telegram show up in CLI and vice versa.

### Step 1 — Create your bot (one-time, 30 seconds)

1. Open **Telegram** on your phone or desktop
2. Search for **@BotFather** and start a chat
3. Send `/newbot`
4. Choose a **name** (e.g. `Govin Expenses`) and a **username** (must end in `bot`, e.g. `govin_expenses_bot`)
5. BotFather replies with a **token** like `7123456789:AAHk...xyz` — copy it

### Step 2 — Start the bot

```bash
# Option A: pass token directly
govin bot --token "YOUR_TOKEN_HERE"

# Option B: save as env var (recommended — no re-typing)
echo 'export GOVIN_BOT_TOKEN="YOUR_TOKEN_HERE"' >> ~/.zshrc
source ~/.zshrc
govin bot
```

You should see:

```
🤖 govin bot is running — press Ctrl+C to stop
```

> ⚠️ The bot runs as a long-polling process — it needs to stay running. Use `tmux`, `screen`, or run on a server for always-on access.

### Step 3 — Open Telegram and start chatting

1. Open **Telegram**
2. Search for the **username** you chose (e.g. `@govin_expenses_bot`)
3. Tap **Start** or send `/start`
4. You're ready!

---

### Bot Commands

| Command                                         | What it does                   | Example                                                   |
| ----------------------------------------------- | ------------------------------ | --------------------------------------------------------- |
| `/start`                                        | Welcome message + instructions | `/start`                                                  |
| `/help`                                         | Show all available commands    | `/help`                                                   |
| `/newgroup <name> [currency]`                   | Create a new group             | `/newgroup Goa Trip ₹`                                    |
| `/groups`                                       | List all groups (`▶` = active) | `/groups`                                                 |
| `/use <group>`                                  | Set the active group           | `/use Goa Trip`                                           |
| `/members`                                      | List members in active group   | `/members`                                                |
| `/add <desc> <amount> <paidBy> <people>`        | Add expense (equal split)      | `/add Hotel 9000 Alice Alice,Bob,Charlie`                 |
| `/addexact <desc> <amount> <paidBy> <splits>`   | Add expense (exact amounts)    | `/addexact Dinner 1200 Bob Alice:400,Bob:400,Charlie:400` |
| `/addpercent <desc> <amount> <paidBy> <splits>` | Add expense (% split)          | `/addpercent Taxi 600 Charlie Alice:50,Bob:25,Charlie:25` |
| `/expenses`                                     | List all expenses              | `/expenses`                                               |
| `/balance`                                      | Show who owes what             | `/balance`                                                |
| `/settle`                                       | Minimum payment plan           | `/settle`                                                 |
| `/paid <from> <to> <amount>`                    | Record a settlement payment    | `/paid Bob Alice 3000`                                    |

---

### Full Example Conversation

```
You:  /start
Bot:  👋 Welcome to govin! ...

You:  /newgroup Goa Trip ₹
Bot:  ✅ Group Goa Trip created (currency: ₹)
      It's now your active group. Start adding expenses!

You:  /add Hotel 9000 Alice Alice,Bob,Charlie
Bot:  ✅ Hotel — ₹ 9000.00 paid by Alice

      Split:
        Alice: ₹ 3000.00
        Bob: ₹ 3000.00
        Charlie: ₹ 3000.00

You:  /add Dinner 1500 Bob Alice,Bob,Charlie
Bot:  ✅ Dinner — ₹ 1500.00 paid by Bob

      Split:
        Alice: ₹ 500.00
        Bob: ₹ 500.00
        Charlie: ₹ 500.00

You:  /balance
Bot:  💰 Balances — Goa Trip

        🟢 Alice  +₹ 6500.00 (gets back)
        🟢 Bob    +₹ 500.00 (gets back)
        🔴 Charlie -₹ 3500.00 (owes)

You:  /settle
Bot:  🧮 Settlement Plan — Goa Trip
      2 payment(s) needed

        1. Charlie → Alice  ₹ 3500.00
        2. Charlie → Bob    ₹ 500.00

      Record with: /paid <from> <to> <amount>

You:  /paid Charlie Alice 3500
Bot:  ✅ Recorded: Charlie paid ₹ 3500.00 → Alice

You:  /balance
Bot:  ✅ All settled up!
```

---

### Using the Bot in a Telegram Group Chat

You can add the bot to a **Telegram group** so all friends can add expenses together:

1. Create a Telegram group with your friends
2. Add your bot to the group (search its username → Add to Group)
3. Anyone in the group can send commands — they all share the same active group

> 💡 The active group is tracked **per chat** — your DM with the bot and each Telegram group chat can have different active groups.

---

## CLI vs Telegram — Command Cheatsheet

> Set `export GOVIN_SERVER=http://host:8080` to transparently use a remote server for all CLI commands.

| Action            | CLI                                                                            | Telegram                                      |
| ----------------- | ------------------------------------------------------------------------------ | --------------------------------------------- |
| Create group      | `govin group create "Trip" --currency "₹"`                                     | `/newgroup Trip ₹`                            |
| List groups       | `govin group list`                                                             | `/groups`                                     |
| Set active group  | `govin use "Trip"`                                                             | `/use Trip`                                   |
| Add equal expense | `govin add "Hotel" --paid-by Alice --amount 9000 --equal --with "Alice,Bob"`   | `/add Hotel 9000 Alice Alice,Bob`             |
| Add exact expense | `govin add "Dinner" --paid-by Bob --amount 1200 --amounts "Alice:400,Bob:800"` | `/addexact Dinner 1200 Bob Alice:400,Bob:800` |
| Add % expense     | `govin add "Taxi" --paid-by Alice --amount 600 --percent "Alice:60,Bob:40"`    | `/addpercent Taxi 600 Alice Alice:60,Bob:40`  |
| List expenses     | `govin list`                                                                   | `/expenses`                                   |
| View balances     | `govin balance`                                                                | `/balance`                                    |
| Settlement plan   | `govin settle`                                                                 | `/settle`                                     |
| Record payment    | `govin paid --from Bob --to Alice --amount 500`                                | `/paid Bob Alice 500`                         |
| List members      | `govin member list`                                                            | `/members`                                    |
| Export            | `govin export --output trip.json`                                              | _(CLI only)_                                  |
| Import            | `govin import trip.json`                                                       | _(CLI only)_                                  |

---

## Sharing with Friends

### Option 1 — `govin serve` _(real-time, recommended)_

One person runs the server, everyone else points their CLI at it. All changes are instant — no import/export needed.

**Host (runs the server):**

```bash
govin serve --port 8080
# Server starts: 🌐 govin server listening on :8080
```

**Friends (connect to the server):**

```bash
export GOVIN_SERVER=http://<host-ip>:8080
# Now all commands talk to the server automatically
govin group list
govin balance
govin add "Hotel" --paid-by Alice --amount 9000 --equal --with "Alice,Bob,Charlie"
```

**How to find your IP:**

```bash
# macOS / Linux
ipconfig getifaddr en0   # WiFi IP — share this with friends on the same network
# or for internet access, use your public IP or run on Railway/VPS
```

> 💡 All govin commands work identically in remote mode — same flags, same output. The only difference is `export GOVIN_SERVER=...` at the start.

---

### Option 2 — Telegram Bot _(best for phone users)_

Run the bot on your machine or a server. Everyone chats with the same bot, and all expenses go into the same database. No setup for your friends — they just open Telegram and type commands.

See [Telegram Bot](#telegram-bot-) section above for setup.

### Option 3 — JSON Export/Import _(async, works offline)_

```
You:    govin export --output trip.json
        → send via WhatsApp / email / AirDrop

Friend: govin import trip.json
        → merges your expenses into their local DB, no duplicates
```

Repeat in both directions to stay in sync. Re-importing is always safe.

---

## Data Storage

|                           | Path                         |
| ------------------------- | ---------------------------- |
| Default database          | `~/.govin/data.db`           |
| Active group config (CLI) | `~/.govin/config.json`       |
| Bot sessions (Telegram)   | Stored in the same `data.db` |

---
