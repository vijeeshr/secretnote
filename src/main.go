package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
)

type Message struct {
	Id        string `redis:"id"`
	Content   string `redis:"content"`
	ViewsLeft uint8  `redis:"viewsLeft"`
}

type CreateMessageReq struct {
	Content         string `json:"content"`
	Views           uint8  `json:"views"`
	ExpiryInMinutes uint8  `json:"expiryInMinutes"`
}
type CreateMessageRes struct {
	Id string `json:"id"`
}

type ViewMessageRes struct {
	Content string `json:"content"`
}

func main() {
	debug := flag.Bool("debug", false, "set to true to run in debug mode")
	flag.Parse()

	// Prepare Logger
	var logger *slog.Logger
	if *debug {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
	} else {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	slog.SetDefault(logger)

	// Connect to Redis
	red := NewRedisConnector(context.Background())
	defer red.Close()

	router := http.NewServeMux()

	router.HandleFunc("POST /create", func(w http.ResponseWriter, r *http.Request) {
		// Origin check
		// if !IsValidOrigin(r) {
		// 	http.Error(w, "invalid origin", http.StatusForbidden)
		// 	return
		// }

		// Parse request
		var createReq CreateMessageReq
		err := decodeJSONBody(w, r, &createReq)
		if err != nil {
			var mr *malformedRequest
			if errors.As(err, &mr) {
				http.Error(w, mr.msg, mr.status)
			} else {
				slog.Error("Error parsing CreateMessageRequest", "details", err.Error())
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
			return
		}

		// Validate
		content := strings.TrimSpace(createReq.Content)
		if content == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if len(content) > 2048 {
			http.Error(w, "content is restricted to 2048 bytes", http.StatusRequestEntityTooLarge)
			return
		}
		if createReq.Views == 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if createReq.ExpiryInMinutes < 1 || createReq.ExpiryInMinutes > 60 {
			http.Error(w, "argument 'expiryInMinutes' must be in 1-60 range", http.StatusBadRequest)
			return
		}

		// Start creation
		id := uuid.NewString()
		message := &Message{Id: id, Content: content, ViewsLeft: createReq.Views}

		// Save to Redis
		if ok := red.Create(message, createReq.ExpiryInMinutes); !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Prepare response
		data, err := json.Marshal(CreateMessageRes{Id: id})
		if err != nil {
			slog.Error("Error marshalling CreateMessageRes", "details", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write(data)
	})

	router.HandleFunc("GET /show/{id}", func(w http.ResponseWriter, r *http.Request) {
		// Origin check
		// if !IsValidOrigin(r) {
		// 	http.Error(w, "invalid origin", http.StatusForbidden)
		// 	return
		// }

		id := r.PathValue("id")
		if id == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		msg, ok := red.Show(id)
		if !ok || msg == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Prepare response
		data, err := json.Marshal(ViewMessageRes{Content: msg.Content})
		if err != nil {
			slog.Error("Error marshalling ViewMessageRes", "details", err.Error(), "id", id)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	logger.Info("Server listening on port 8085")
	if err := http.ListenAndServe(":8085", router); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

func IsValidOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	switch origin {
	case "http://localhost:8085", "https://localhost:8085", "http://localhost", "https://localhost":
		return true
	}
	return false
}
