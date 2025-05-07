package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
)

type Command struct {
	Name string
	Args []string
}

type RedisStore struct {
	data      map[string]string
	mutex     sync.RWMutex
	aofFile   *os.File
	aofWriter *bufio.Writer
}

func NewRedisStore() (*RedisStore, error) {
	fmt.Println("Creating RedisStore...")
	aofFile, err := os.OpenFile("redisstore.aof", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	aofWriter := bufio.NewWriter(aofFile)
	return &RedisStore{
		data:      make(map[string]string),
		aofFile:   aofFile,
		aofWriter: aofWriter,
	}, nil
}

func (r *RedisStore) Close() {
	if r.aofFile != nil {
		r.aofWriter.Flush()
		r.aofFile.Close()
	}
}

func (r *RedisStore) writeAOF(command string, args ...string) {
	line := fmt.Sprintf("%s %s\n", command, strings.Join(args, " "))
	r.aofWriter.WriteString(line)
	r.aofWriter.Flush()
}

func (r *RedisStore) loadAOF() error {
	file, err := os.Open("redisstore.aof")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	// Replace inputCapture with processAOFCommands
	return r.processAOFCommands(file)
}

// New function to process AOF commands without entering an infinite loop
func (r *RedisStore) processAOFCommands(file io.Reader) error {
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		command := parseCommand(line)
		// Only process SET commands when loading from AOF
		if command.Name == "SET" && len(command.Args) >= 2 {
			// Set directly to the data map without writing to AOF again
			r.mutex.Lock()
			r.data[command.Args[0]] = command.Args[1]
			r.mutex.Unlock()
		}
	}

	return scanner.Err()
}

func (r *RedisStore) Get(key string) (string, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	val, exists := r.data[key]
	if !exists {
		return "", false
	}
	return val, true
}

func (r *RedisStore) Set(key string, val string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.data[key] = val
	r.writeAOF("SET", key, val)
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

func handleConnection(conn net.Conn, rs *RedisStore) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		command := parseCommand(scanner.Text())
		response := processCommand(command, rs)
		conn.Write([]byte(response + "\n"))
	}
}

func StartServer(rs *RedisStore) error {
	listener, err := net.Listen("tcp", ":6379")
	if err != nil {
		return err
	}
	defer listener.Close()
	fmt.Printf("server started on %s\n", listener.Addr())
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("connection error: ", err)
			continue
		}
		go handleConnection(conn, rs)
	}
}

func processCommand(cmd Command, rs *RedisStore) string {
	switch cmd.Name {
	case "GET":
		if len(cmd.Args) == 1 {
			val, exists := rs.Get(cmd.Args[0])
			if exists {
				return val
			}
			return "nil"
		}
	case "SET":
		if len(cmd.Args) >= 2 {
			rs.Set(cmd.Args[0], cmd.Args[1])
			return "OK"
		}
	}
	return ""
}

func inputCapture(input io.Reader, rs *RedisStore) {
	scanner := bufio.NewScanner(input)
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
		response := processCommand(cmd, rs)
		fmt.Println(response)
		if err := scanner.Err(); err != nil {
			fmt.Println("error reading input: ", err)
		}
	}
}

func main() {
	rs, err := NewRedisStore()
	if err != nil {
		log.Fatal(err)
		return
	}
	defer rs.Close()

	if err := rs.loadAOF(); err != nil {
		fmt.Println("Error loading AOF: ", err)
		return
	}

	go func() {
		if err := StartServer(rs); err != nil {
			log.Fatal(err)
		}
	}()

	// input -> redis store.
	inputCapture(os.Stdin, rs)
}
