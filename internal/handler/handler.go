package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"context"

	"github.com/example/auth/internal/models"
	"github.com/example/auth/internal/token"
)

// Handler holds dependencies for HTTP handlers

type Handler struct {
	DB    *sqlx.DB
	Redis *redis.Client
}

func New(db *sqlx.DB, r *redis.Client) *Handler {
	return &Handler{DB: db, Redis: r}
}

func (h *Handler) Register(r *mux.Router) {
	r.HandleFunc("/signup", h.SignUp).Methods("POST")
	r.HandleFunc("/login", h.Login).Methods("POST")
	r.HandleFunc("/refresh", h.Refresh).Methods("POST")
	r.HandleFunc("/logout", h.Logout).Methods("POST")
}

type signUpRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) SignUp(w http.ResponseWriter, r *http.Request) {
	var req signUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var id int64
	err := h.DB.QueryRowx("INSERT INTO users(email, password) VALUES($1,$2) RETURNING id", req.Email, req.Password).Scan(&id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]int64{"id": id})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var user models.User
	err := h.DB.Get(&user, "SELECT * FROM users WHERE email=$1", req.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if user.Password != req.Password {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	jwtToken, err := token.Generate(user.ID, time.Hour)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	refresh, err := token.Generate(user.ID, time.Hour*24)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = h.DB.Exec("INSERT INTO refresh_tokens(user_id, token) VALUES($1,$2)", user.ID, refresh)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"access": jwtToken, "refresh": refresh})
}

type refreshRequest struct {
	Token string `json:"token"`
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var rt models.RefreshToken
	err := h.DB.Get(&rt, "SELECT * FROM refresh_tokens WHERE token=$1", req.Token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	jwtToken, err := token.Generate(rt.UserID, time.Hour)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"access": jwtToken})
}

type logoutRequest struct {
	Token string `json:"token"`
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req logoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// store token in Redis blacklist with TTL
	err := h.Redis.Set(context.Background(), req.Token, "blacklisted", time.Hour).Err()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
