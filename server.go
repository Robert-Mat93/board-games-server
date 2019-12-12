package main

import (
	"encoding/json"
	"fmt"
	"github.com/segmentio/ksuid"
	"log"
	"net/http"
	"strings"
)

type GameServer struct {
	brokers map[string]*Broker
}

func NewServer() (server *GameServer) {
	return &GameServer{
		brokers: make(map[string]*Broker),
	}
}

func (server *GameServer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	log.Printf("got request %s", req.URL)
	segments := strings.Split(req.URL.Path, "/")

	if len(segments) <= 1 {
		http.Error(rw, "", http.StatusBadRequest)
		return
	}
	log.Printf("request: %s", segments[1])
	switch segments[1] {
	case "start_game":
		id := ksuid.New().String()
		response := GameStarResponse{GameID: id}
		js, err := json.Marshal(response)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		broker := NewBroker()
		server.brokers[id] = broker
		log.Printf("Started new game, id: %s %#v", id, server.brokers)
		rw.Header().Set("Access-Control-Allow-Origin", "*")
		rw.Header().Set("Content-Type", "application/json")
		rw.Write(js)

	case "join_game":
		// Make sure that the writer supports flushing.
		//
		flusher, ok := rw.(http.Flusher)

		if !ok {
			log.Printf("missing streaming")
			http.Error(rw, "Streaming unsupported!", http.StatusInternalServerError)
			return
		}

		if len(segments) <= 2 {
			http.Error(rw, "", http.StatusBadRequest)
			return
		}
		id := segments[2]
		log.Printf("id: %s", id)
		broker, exists := server.brokers[id]
		if !exists {
			log.Printf("missing broker %#v", server.brokers)
			http.Error(rw, "", http.StatusBadRequest)
			return
		}

		rw.Header().Set("Content-Type", "text/event-stream")
		rw.Header().Set("Cache-Control", "no-cache")
		rw.Header().Set("Connection", "keep-alive")
		rw.Header().Set("Access-Control-Allow-Origin", "*")

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
		err := json.NewDecoder(req.Body).Decode(&event)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		broker, exists := server.brokers[event.GameID]
		if !exists {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		broker.Notifier <- []byte(event.Event)
	}
}
