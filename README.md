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