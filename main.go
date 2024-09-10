package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mehriddinnozimov/wsgo/websocket"
)

type message struct {
	Message string
}

func createLongJsonMessage(length int) (string, error) {
	objs := make([]message, length)

	for index := range length {
		objs[index] = message{"LongJsonMessage"}
	}

	b, err := json.Marshal(objs)
	if err != nil {
		return "", err
	} else {
		return string(b), nil
	}
}

func onConnection(s *websocket.Socket) {
	fmt.Printf("New Connection. Socket %d \n", s.ID)

	s.OnError(func(err error) {
		fmt.Printf("Socket: %d, Error: %v \n", s.ID, err.Error())
	})
	s.OnMessage(func(message string) {
		fmt.Printf("Socket: %d, New Message: %v \n", s.ID, message)

		if message == "close" {
			s.Close()
			return
		}

		s.Send(fmt.Sprintf("I receive message, ID: %d", s.ID))
		s.Send(fmt.Sprintf("this is another message, ID: %d", s.ID))

		longMessage, err := createLongJsonMessage(10000)
		if err != nil {
			s.Send(err.Error())
		}

		s.Send(fmt.Sprintf("Long message len: %v", len(longMessage)))

		s.Send(longMessage)
	})

	s.OnClose(func() {
		fmt.Printf("Socket: %d, Closed, socket State: %v \n", s.ID, s.State)
	})
}

func main() {
	ws := websocket.NewWebSocket()

	ws.OnConnection(onConnection)

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws.Upgrade(w, r)
	})

	fmt.Println("Server started")
	http.ListenAndServe(":3000", nil)
}
