# Building a Simple Redis Clone in Go

## Project Goals
Create a basic in-memory key-value store with core Redis-like functionality, including:
- Simple key-value storage
- Basic data types (strings, lists)
- Expiration mechanisms
- Basic server architecture

## Step-by-Step Implementation Guide

### 1. Basic Key-Value Store Structure
First, let's create the core data storage mechanism:

```go
type RedisStore struct {
    data map[string]string
    mutex sync.RWMutex
}

func NewRedisStore() *RedisStore {
    return &RedisStore{
        data: make(map[string]string),
    }
}

func (rs *RedisStore) Set(key, value string) {
    rs.mutex.Lock()
    defer rs.mutex.Unlock()
    rs.data[key] = value
}

func (rs *RedisStore) Get(key string) (string, bool) {
    rs.mutex.RLock()
    defer rs.mutex.RUnlock()
    value, exists := rs.data[key]
    return value, exists
}
```

### 2. Add Expiration Mechanism
Implement key expiration to mimic Redis's time-based key management:

```go
type StoredValue struct {
    value      string
    expiration time.Time
}

type RedisStore struct {
    data map[string]StoredValue
    mutex sync.RWMutex
}

func (rs *RedisStore) SetWithExpiration(key, value string, duration time.Duration) {
    rs.mutex.Lock()
    defer rs.mutex.Unlock()
    rs.data[key] = StoredValue{
        value:      value,
        expiration: time.Now().Add(duration),
    }
}

func (rs *RedisStore) Get(key string) (string, bool) {
    rs.mutex.Lock()
    defer rs.mutex.Unlock()
    
    storedValue, exists := rs.data[key]
    if !exists {
        return "", false
    }
    
    if time.Now().After(storedValue.expiration) {
        delete(rs.data, key)
        return "", false
    }
    
    return storedValue.value, true
}
```

### 3. List Data Type Support
Add support for list operations:

```go
type RedisStore struct {
    stringData map[string]StoredValue
    listData   map[string][]string
    mutex      sync.RWMutex
}

func (rs *RedisStore) ListPush(key string, value string) {
    rs.mutex.Lock()
    defer rs.mutex.Unlock()
    rs.listData[key] = append(rs.listData[key], value)
}

func (rs *RedisStore) ListPop(key string) (string, bool) {
    rs.mutex.Lock()
    defer rs.mutex.Unlock()
    
    if len(rs.listData[key]) == 0 {
        return "", false
    }
    
    value := rs.listData[key][0]
    rs.listData[key] = rs.listData[key][1:]
    return value, true
}
```

### 4. Network Server Implementation
Create a simple TCP server to handle Redis-like commands:

```go
func StartServer(store *RedisStore) error {
    listener, err := net.Listen("tcp", ":6379")
    if err != nil {
        return err
    }
    defer listener.Close()

    for {
        conn, err := listener.Accept()
        if err != nil {
            log.Println("Connection error:", err)
            continue
        }
        go handleConnection(conn, store)
    }
}

func handleConnection(conn net.Conn, store *RedisStore) {
    defer conn.Close()
    scanner := bufio.NewScanner(conn)
    
    for scanner.Scan() {
        // Parse and handle commands
        // Implement SET, GET, EXPIRE, LPUSH, LPOP etc.
        command := parseCommand(scanner.Text())
        response := processCommand(store, command)
        conn.Write([]byte(response + "\n"))
    }
}
```

### 5. Command Parsing and Processing
Implement a basic command parser:

```go
type Command struct {
    Name   string
    Args   []string
}

func parseCommand(input string) Command {
    parts := strings.Fields(input)
    if len(parts) == 0 {
        return Command{}
    }
    return Command{
        Name: strings.ToUpper(parts[0]),
        Args: parts[1:],
    }
}

func processCommand(store *RedisStore, cmd Command) string {
    switch cmd.Name {
    case "SET":
        if len(cmd.Args) >= 2 {
            store.Set(cmd.Args[0], cmd.Args[1])
            return "OK"
        }
    case "GET":
        if len(cmd.Args) == 1 {
            value, exists := store.Get(cmd.Args[0])
            if exists {
                return value
            }
            return "(nil)"
        }
    // Add more command handlers
    }
    return "ERR unknown command"
}
```

## Learning Objectives
1. Understand in-memory data storage
2. Implement concurrent access with mutexes
3. Create time-based expiration mechanisms
4. Build a simple network server
5. Implement basic command parsing

## Recommended Extensions
- Add more data types (hashes, sets)
- Implement persistence
- Add more complex expiration strategies
- Improve error handling
- Create a CLI client
```

## Project Structure and Implementation Steps

### Conceptual Learning Path
1. **Core Data Storage**
   - Understand how key-value stores work in memory
   - Learn about concurrent access and thread safety
   - Implement basic get/set operations

2. **Data Persistence and Expiration**
   - Add time-based key expiration
   - Understand memory management techniques
   - Implement automatic key cleanup

3. **Network Communication**
   - Create a TCP server
   - Implement basic network protocol
   - Handle client connections
   - Parse and process commands

4. **Advanced Features**
   - Add support for more complex data types
   - Implement more sophisticated command parsing
   - Add error handling and logging

### Learning Recommendations
- Start with the basic in-memory store
- Incrementally add features
- Test each component thoroughly
- Compare your implementation with Redis source code
- Experiment with different design approaches

### Key Challenges to Explore
- Concurrent access and thread safety
- Memory management
- Network protocol design
- Command parsing and execution
- Performance optimization
