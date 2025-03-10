package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

/*
	Redis is a in-memory data structure store

the frist thing to do is build out this type
*/
type Command struct {
	Name string
	Args []string
}

type RedisStore struct {
	data            map[string]StoredValue
	listData        map[string][]string
	mutex           sync.RWMutex
	aofFile         *os.File
	aofWriter       *bufio.Writer
	writeAOFEnabled bool
}

type StoredValue struct {
	value      string
	expiration time.Time
}

func NewRedisStore() (*RedisStore, error) {
	aofFile, err := os.OpenFile("redisstore.aof", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	aofWriter := bufio.NewWriter(aofFile)
	return &RedisStore{
		data:      make(map[string]StoredValue),
		listData:  make(map[string][]string),
		aofFile:   aofFile,
		aofWriter: aofWriter,
	}, nil
}

func (r *RedisStore) ListPush(key, value string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.listData[key] = append(r.listData[key], value)
}

func (r *RedisStore) ListPop(key string) (string, bool) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if len(r.listData[key]) == 0 {
		return "", false
	}
	value := r.listData[key][0]
	r.listData[key] = r.listData[key][1:]
	return value, true
}

func (r *RedisStore) SetWithExpiration(key, value string, duration time.Duration) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.data[key] = StoredValue{
		value:      value,
		expiration: time.Now().Add(duration),
	}
	r.writeAOF("SET", key, value)
}

// we want to be able to set a value in the redis store
func (r *RedisStore) Set(key string, value StoredValue) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.data[key] = value
	r.writeAOF("SET", key, value.value)
}

// we want to be able to get a value from the redis store
func (r *RedisStore) Get(key string) (string, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	StoredValue, exists := r.data[key]
	if !exists {
		return "", false
	}
	if !StoredValue.expiration.IsZero() && time.Now().After(StoredValue.expiration) {
		fmt.Println("Key has expired")
		return "", false
	}
	return StoredValue.value, true
}

func (r *RedisStore) Del(key string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(r.data, key)
	r.writeAOF("DEL", key)
}

func (r *RedisStore) HMSet(key string, values map[string]string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for k, v := range values {
		r.data[key+"."+k] = StoredValue{value: v}
		r.writeAOF("HMSET", key, k, v)
	}
}

func (r *RedisStore) HMGet(key string, fields []string) map[string]string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	result := make(map[string]string)
	for _, field := range fields {
		if val, exists := r.data[key+"."+field]; exists {
			result[field] = val.value
		}
	}
	return result
}

func StartServer(store *RedisStore) error {
	listener, err := net.Listen("tcp", ":6379")
	if err != nil {
		return err
	}
	defer listener.Close()
	fmt.Printf("Server started on %s\n", listener.Addr())

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Connection error: ", err)
			continue
		}
		go handleConnection(conn, store)
	}
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
	case "GET":
		if len(cmd.Args) == 1 {
			val, exists := store.Get(cmd.Args[0])
			if exists {
				return val
			}
			return "(nil)"
		}
	case "SET":
		if len(cmd.Args) >= 2 {
			val := StoredValue{value: cmd.Args[1]}
			store.Set(cmd.Args[0], val)
			return "OK"
		}
	case "EXPIRE":
		if len(cmd.Args) >= 3 {
			seconds, err := strconv.Atoi(cmd.Args[2])
			if err != nil {
				return "ERR invalid expire time"
			}
			store.SetWithExpiration(cmd.Args[0], cmd.Args[1], time.Duration(seconds)*time.Second)
			return "OK"
		}
	case "DEL":
		if len(cmd.Args) == 1 {
			store.Del(cmd.Args[0])
			return "OK"
		}
	case "HMSET":
		if len(cmd.Args) >= 2 {
			values := make(map[string]string)
			for i := 1; i < len(cmd.Args); i += 2 {
				values[cmd.Args[i]] = cmd.Args[i+1]
			}
			store.HMSet(cmd.Args[0], values)
			return "OK"
		}
	case "HMGET":
		if len(cmd.Args) >= 2 {
			fields := cmd.Args[1:]
			values := store.HMGet(cmd.Args[0], fields)
			result := make([]string, 0, len(values)*2)
			for k, v := range values {
				result = append(result, k, v)
			}
			return fmt.Sprintf("%v", result)
		}
	case "EXIT":
		fmt.Println("Exiting...")
		os.Exit(0)
	}
	return ""

}

func (r *RedisStore) writeAOF(command string, args ...string) {
	if !r.writeAOFEnabled {
		return
	}
	line := fmt.Sprintf("%s %s\n", command, strings.Join(args, " "))
	r.aofWriter.WriteString(line)
	r.aofWriter.Flush()
}

func (r *RedisStore) handleCommands(commands []Command) []string {
	responses := make([]string, len(commands))
	for i, cmd := range commands {
		responses[i] = processCommand(r, cmd)
	}
	return responses
}
func (r *RedisStore) loadAOF() error {
	r.writeAOFEnabled = false
	file, err := os.Open("redisstore.aof")
	if err != nil {
		if os.IsNotExist(err) {
			return nil // AOF doesn't exist, start empty
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, " ")
		if len(parts) < 2 {
			continue
		}
		command := parts[0]
		args := parts[1:]

		cmd := Command{Name: command, Args: args}
		processCommand(r, cmd)
	}

	if err := scanner.Err(); err != nil {
		r.writeAOFEnabled = true
		return err
	}
	r.writeAOFEnabled = true
	return nil

}
func handleConnection(conn net.Conn, store *RedisStore) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		command := parseCommand(scanner.Text())
		response := processCommand(store, command)
		conn.Write([]byte(response + "\n"))
	}

}
func (r *RedisStore) Close() {
	if r.aofWriter != nil {
		r.aofWriter.Flush()
		r.aofFile.Close()
	}
}

func main() {
	rs, err := NewRedisStore()
	if err != nil {
		fmt.Println("Error creating redis store: ", err)
		return
	}
	defer rs.Close()
	if err := rs.loadAOF(); err != nil {
		fmt.Println("Error loading AOF: ", err)
		return
	}

	// Start server in a goroutine
	go func() {
		if err := StartServer(rs); err != nil {
			log.Fatal(err)
		}
	}()
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		parts := strings.Split(line, " ")
		if len(parts) == 0 {
			continue
		}

		command := strings.ToUpper(parts[0])
		args := parts[1:]

		cmd := Command{Name: command, Args: args}
		response := processCommand(rs, cmd)
		fmt.Println(response)

		if command == "EXIT" {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading input:", err)
	}
}
