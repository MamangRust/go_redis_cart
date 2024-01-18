package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
)

// var redisClient *redis.Client

// Product struct represents a product in the shopping cart
type Product struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

// ShoppingCart struct represents the user's shopping cart
type ShoppingCart struct {
	UserID   string    `json:"userID"`
	Products []Product `json:"products"`
}

type RedisClient struct {
	redis *redis.Client
	ctx   context.Context
}

func NewRedisClient(addr, password string, db int) *RedisClient {
	options := &redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	}

	client := redis.NewClient(options)

	// Ping Redis to check the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		log.Fatal("Error connecting to Redis:", err)
	} else {
		log.Println("Successfully connected to Redis.")
	}

	return &RedisClient{
		redis: client,
		ctx:   context.Background(), // You can customize the context here if needed
	}
}

type Handler struct {
	redisClient *redis.Client
	ctx         context.Context
}

func NewHandler(redisClient *redis.Client, ctx context.Context) *Handler {
	return &Handler{
		redisClient: redisClient,
		ctx:         ctx,
	}
}

func (h *Handler) addToCartHandler(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var product Product
	err := json.NewDecoder(r.Body).Decode(&product)
	if err != nil {
		http.Error(w, "Error decoding request body", http.StatusBadRequest)
		return
	}

	// Generate a random userID (in a real-world scenario, you would use user authentication)
	userID := uuid.New().String()

	// Get the user's shopping cart from Redis
	key := fmt.Sprintf("cart:%s", userID)

	fmt.Println("userID: ", userID)

	productsJSON, err := h.redisClient.LRange(h.ctx, key, 0, -1).Result()
	if err != nil {
		http.Error(w, "Error retrieving shopping cart", http.StatusInternalServerError)
		return
	}

	// Deserialize the products from JSON
	var products []Product
	for _, productJSON := range productsJSON {
		var existingProduct Product
		err := json.Unmarshal([]byte(productJSON), &existingProduct)
		if err != nil {
			http.Error(w, "Error decoding product from JSON", http.StatusInternalServerError)
			return
		}
		products = append(products, existingProduct)
	}

	// Add the new product to the shopping cart
	products = append(products, product)

	// Serialize the updated shopping cart to JSON
	updatedCart, err := json.Marshal(products)
	if err != nil {
		http.Error(w, "Error encoding updated cart to JSON", http.StatusInternalServerError)
		return
	}

	// Update the user's shopping cart in Redis
	err = h.redisClient.Del(h.ctx, key).Err()
	if err != nil {
		http.Error(w, "Error clearing existing cart", http.StatusInternalServerError)
		return
	}
	err = h.redisClient.RPush(h.ctx, key, updatedCart).Err()
	if err != nil {
		http.Error(w, "Error updating shopping cart", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Product added to cart: " + userID))
}

func (h *Handler) viewCartHandler(w http.ResponseWriter, r *http.Request) {
	userId := mux.Vars(r)["userId"]
	key := fmt.Sprintf("cart:%s", userId)

	// Get the user's shopping cart from Redis
	productsJSON, err := h.redisClient.LRange(h.ctx, key, 0, -1).Result()
	if err != nil {
		http.Error(w, "Error retrieving shopping cart", http.StatusInternalServerError)
		return
	}

	// Deserialize the products from JSON
	var products []Product
	for _, productJSON := range productsJSON {
		var productArray []Product
		err := json.Unmarshal([]byte(productJSON), &productArray)
		if err != nil {
			http.Error(w, "Error decoding product array from JSON", http.StatusInternalServerError)
			return
		}

		// Append the products from the array to the main products slice
		products = append(products, productArray...)
	}

	// Return the user's shopping cart as JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)
}

func main() {
	// Example routes

	redisClient := NewRedisClient("localhost:6379", "", 0)

	handler := NewHandler(redisClient.redis, context.Background())

	router := mux.NewRouter()

	router.HandleFunc("/add-to-cart", handler.addToCartHandler).Methods("POST")
	router.HandleFunc("/view-cart/{userId}", handler.viewCartHandler).Methods("GET")

	// Start the server
	serverAddr := "localhost:8080"
	fmt.Printf("Server is running on http://%s\n", serverAddr)

	log.Fatal(http.ListenAndServe(":8080", router))
}
