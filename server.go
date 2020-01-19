package main

import (
	"encoding/json"
	"fmt"
	"github.com/derekstavis/go-qs"
	"github.com/mitchellh/mapstructure"
	"github.com/segmentio/ksuid"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
)

type GameServer struct {
	brokers   map[string]*Broker
	clients   map[string]chan string
	notif     chan string
	connector Connector
	lock      sync.Mutex
}

func NewServer() (server *GameServer) {
	connector, err := NewConnector(DynamoDB)
	if err != nil {
		return nil
	}
	server = &GameServer{
		brokers:   make(map[string]*Broker),
		notif:     make(chan string),
		connector: connector,
	}

	go server.deleteBrokers()
	return server
}

func (server *GameServer) getBroker(id string) *Broker {
	server.lock.Lock()
	defer server.lock.Unlock()
	return server.brokers[id]
}

func (server *GameServer) addBroker(id string, broker *Broker) {
	server.lock.Lock()
	defer server.lock.Unlock()
	server.brokers[id] = broker
}

func (server *GameServer) deleteBrokers() {
	for {
		id := <-server.notif
		server.lock.Lock()
		delete(server.brokers, id)
		server.lock.Unlock()
	}
}

func (server *GameServer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	segments := strings.Split(req.URL.Path, "/")

	if len(segments) <= 1 {
		http.Error(rw, "", http.StatusBadRequest)
		return
	}
	switch segments[1] {
	case "user_list":
		var users []User
		users = server.connector.GetUsers()

		js, err := json.Marshal(users)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		rw.Header().Set("Access-Control-Allow-Origin", "*")
		rw.Header().Set("Content-Type", "application/json")
		rw.Write(js)

	case "create_user":
		var user = &User{}
		buf, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Printf("Failed to read body: %s", err.Error())
			return
		}
		query, err := qs.Unmarshal(string(buf))
		if err != nil {
			log.Printf("Failed to unmarshal body: %s", err.Error())
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		err = mapstructure.Decode(query, user)
		if err != nil {
			log.Printf("Failed to decode: %s", err.Error())
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		err = server.connector.AddUser(user)
		if err != nil {
			log.Printf("Failed to add user: %s", err.Error())
			http.Error(rw, err.Error(), 401)
		}
		rw.Header().Set("Access-Control-Allow-Origin", "*")
		rw.Header().Set("Content-Type", "application/json")
		js, _ := json.Marshal(user)
		rw.Write(js)

	case "start_game":
		id := ksuid.New().String()
		response := GameStarResponse{GameID: id}
		js, err := json.Marshal(response)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		broker := NewBroker(server.notif, id)
		server.addBroker(id, broker)
		log.Printf("Started new game, id: %s", id)
		rw.Header().Set("Access-Control-Allow-Origin", "*")
		rw.Header().Set("Content-Type", "application/json")
		rw.Write(js)
	case "log_in":
		var logIn = &LogIn{}
		buf, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Printf("Failed to read body: %s", err.Error())
			return
		}
		query, err := qs.Unmarshal(string(buf))
		if err != nil {
			log.Printf("Failed to unmarshal body: %s", err.Error())
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		err = mapstructure.Decode(query, logIn)
		if err != nil {
			log.Printf("Failed to decode: %s", err.Error())
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}

	case "join_game":
		flusher, ok := rw.(http.Flusher)

		if !ok {
			http.Error(rw, "Streaming unsupported!", http.StatusInternalServerError)
			return
		}

		if len(segments) <= 2 {
			http.Error(rw, "", http.StatusBadRequest)
			return
		}
		id := segments[2]
		broker := server.getBroker(id)
		if broker == nil {
			http.Error(rw, "", http.StatusBadRequest)
			return
		}

		rw.Header().Set("Content-Type", "text/event-stream")
		rw.Header().Set("Access-Control-Allow-Origin", "*")
		rw.Header().Set("Cache-Control", "no-cache")
		rw.Header().Set("Connection", "keep-alive")

		// Each connection registers its own message channel with the Broker's connections registry
		messageChan := make(chan []byte)

		// Signal the broker that we have a new connection
		broker.newClients <- messageChan

		// Remove this client from the map of connected clients
		// when this handler exits.
		defer func() {
			broker.closingClients <- messageChan
		}()

		// Listen to connection close and un-register messageChan
		// notify := rw.(http.CloseNotifier).CloseNotify()
		notify := req.Context().Done()

		go func() {
			<-notify
			broker.closingClients <- messageChan
		}()

		for {

			// Write to the ResponseWriter
			// Server Sent Events compatible
			fmt.Fprintf(rw, "data: %s\n\n", <-messageChan)

			// Flush the data immediatly instead of buffering it for later.
			flusher.Flush()
		}
	case "game_event":
		var event GameEvent
		buf, err := ioutil.ReadAll(req.Body)
		if err != nil {
		}
		query, err := qs.Unmarshal(string(buf))
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		err = mapstructure.Decode(query, &event)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		broker, exists := server.brokers[event.GameID]
		if !exists {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		rw.Header().Set("Access-Control-Allow-Origin", "*")
		broker.Notifier <- []byte(event.Event)
	}
}
