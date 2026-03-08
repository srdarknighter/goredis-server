## 🛠️ Demonstration Steps

We need to stop the actual Redis server and use ours for the demonstration.

### 1. Stop the Official Redis Server

Run the following command in your terminal to stop the Redis service:

```bash
sudo systemctl stop redis-server.service
```


### 2. Start the goredis server

Start the goredis server:

```bash
go run .
```


### 3. Connect to the server as a Redis client

Use the following command to connect to the Redis server as a Redis client in a new terminal instance:

```bash
redis-cli
```



# goredis

A Redis-compatible in-memory datastore written from scratch in Go. Implements the RESP (Redis Serialization Protocol) wire protocol, making it a drop-in replacement compatible with `redis-cli` and any standard Redis client.

## Features

- **RESP protocol** — full serialization/deserialization of arrays, bulk strings, simple strings, integers, errors, and null
- **Core commands** — `GET`, `SET`, `DEL`, `EXISTS`, `KEYS`, `DBSIZE`, `FLUSHDB`, `PING`
- **TTL / expiry** — `EXPIRE`, `TTL` with passive expiry on access
- **Persistence** — AOF (Append-Only File) with `always`/`everysec`/`no` fsync modes; RDB snapshots via `SAVE` and `BGSAVE`
- **AOF rewrite** — `BGREWRITEAOF` compacts the log to the minimal set of `SET` commands
- **Transactions** — `MULTI` / `EXEC` / `DISCARD` command queueing
- **Memory management** — configurable `maxmemory` cap with `allkeys-lru`, `allkeys-lfu`, `allkeys-random`, `volatile-*`, and `noeviction` policies
- **Authentication** — `requirepass` / `AUTH` support
- **Monitoring** — `MONITOR` streams all incoming commands to observer clients
- **Server info** — `INFO` returns server, client, memory, persistence, and stats sections
- **Concurrent clients** — each connection handled in its own goroutine with `sync.RWMutex`-protected store

The server reads `redis.conf` from the working directory on startup and listens on `:6379`.

### Connect

```bash
redis-cli
127.0.0.1:6379> SET name anirudh
OK
127.0.0.1:6379> GET name
"anirudh"
127.0.0.1:6379> EXPIRE name 10
(integer) 1
127.0.0.1:6379> TTL name
(integer) 9
```

## Configuration

All options are set in `redis.conf`. The server falls back to zero-value defaults if the file is missing.

```properties
dir ./data                    # directory for AOF and RDB files

# Persistence
appendonly yes                # enable AOF logging
appendfilename backup.aof
appendfsync everysec          # flush to disk: always | everysec | no

save 900 1                    # RDB snapshot if ≥1 key changed in 900s
save 300 10
dbfilename backup.rdb

# Auth
requirepass foobared

# Memory
maxmemory 64mb                # 0 = unlimited
maxmemory-policy allkeys-lru  # eviction policy when maxmemory is reached
maxmemory-samples 10          # keys sampled per eviction sweep
```

### Memory policies

| Policy | Behaviour |
|---|---|
| `noeviction` | Reject writes when full |
| `allkeys-lru` | Evict least recently used key |
| `allkeys-lfu` | Evict least frequently used key |
| `allkeys-random` | Evict a random key |
| `volatile-lru/lfu/random/ttl` | Same as above, but only among keys with a TTL set |

> **Note:** `maxmemory 256` means 256 **bytes**. Use a suffix: `256mb`, `1gb`.

## Supported Commands

| Command | Syntax |
|---|---|
| `GET` | `GET key` |
| `SET` | `SET key value` |
| `DEL` | `DEL key [key ...]` |
| `EXISTS` | `EXISTS key [key ...]` |
| `KEYS` | `KEYS pattern` |
| `EXPIRE` | `EXPIRE key seconds` |
| `TTL` | `TTL key` |
| `DBSIZE` | `DBSIZE` |
| `FLUSHDB` | `FLUSHDB` |
| `SAVE` | `SAVE` |
| `BGSAVE` | `BGSAVE` |
| `BGREWRITEAOF` | `BGREWRITEAOF` |
| `MULTI` | `MULTI` |
| `EXEC` | `EXEC` |
| `DISCARD` | `DISCARD` |
| `AUTH` | `AUTH password` |
| `MONITOR` | `MONITOR` |
| `INFO` | `INFO` |
| `PING` | `PING [message]` |

## Architecture

```
main.go          → TCP listener, connection loop, goroutine per client
handlers.go      → command dispatch table and handler implementations
value.go         → RESP parser (readArray, readBulk)
writer.go        → RESP serializer (Deserialize → wire bytes)
db.go            → thread-safe store (Get/Set/Delete + eviction)
item.go          → per-key struct (value, expiry, LRU/LFU metadata)
mem.go           → eviction candidate sampling
aof.go           → AOF write, sync, and rewrite logic
rdb.go           → RDB snapshot save/load with SHA-256 checksum verification
conf.go          → redis.conf parser
appstate.go      → shared server state (config, AOF, stats, monitors, tx)
transaction.go   → MULTI/EXEC command queue
info.go          → INFO command response builder
```

For maximum throughput during load testing, disable persistence and auth:

```properties
appendonly no
# requirepass foobared
maxmemory 0
```

## Persistence Behaviour

**AOF** records every `SET` command in RESP format as it happens. On startup, the server replays the file to restore state. `BGREWRITEAOF` rewrites the log to a minimal snapshot (one `SET` per live key) without blocking client connections.

**RDB** snapshots are triggered automatically based on `save` thresholds (keys changed within a time window). `BGSAVE` copies the store under a read lock and serializes it with `encoding/gob` in a background goroutine, leaving the main connection loop unblocked. A SHA-256 checksum is verified after every write to detect corruption.
