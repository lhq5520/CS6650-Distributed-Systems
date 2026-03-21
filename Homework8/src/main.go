package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

// ==================== Models ====================

type Cart struct {
	CartID       string     `json:"cart_id"`
	CustomerID   string     `json:"customer_id"`
	CustomerName string     `json:"customer_name,omitempty"`
	Items        []CartItem `json:"items"`
	CreatedAt    string     `json:"created_at"`
	UpdatedAt    string     `json:"updated_at"`
}

type CartItem struct {
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name,omitempty"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
}

type CreateCartRequest struct {
	CustomerID   string `json:"customer_id"`
	CustomerName string `json:"customer_name,omitempty"`
}

type AddItemRequest struct {
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name,omitempty"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// ==================== Cart Store Interface ====================

type CartStore interface {
	CreateCart(ctx context.Context, req CreateCartRequest) (*Cart, error)
	GetCart(ctx context.Context, cartID string) (*Cart, error)
	AddItem(ctx context.Context, cartID string, req AddItemRequest) (*Cart, error)
}

// ==================== MySQL Store ====================

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore() (*MySQLStore, error) {
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "3306")
	user := getEnv("DB_USER", "admin")
	pass := getEnv("DB_PASSWORD", "")
	name := getEnv("DB_NAME", "shopping_cart_db")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&timeout=5s", user, pass, host, port, name)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("mysql open: %w", err)
	}

	// Connection pool tuning
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("mysql ping: %w", err)
	}

	log.Println("[MySQL] Connected successfully, initializing schema...")
	if err := initMySQLSchema(db); err != nil {
		return nil, fmt.Errorf("mysql schema init: %w", err)
	}

	log.Println("[MySQL] Ready")
	return &MySQLStore{db: db}, nil
}

func initMySQLSchema(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS carts (
			cart_id       CHAR(36) NOT NULL,
			customer_id   VARCHAR(100) NOT NULL,
			customer_name VARCHAR(255) DEFAULT NULL,
			created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (cart_id),
			INDEX idx_carts_customer (customer_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS cart_items (
			item_id       BIGINT NOT NULL AUTO_INCREMENT,
			cart_id       CHAR(36) NOT NULL,
			product_id    VARCHAR(100) NOT NULL,
			product_name  VARCHAR(255) DEFAULT NULL,
			quantity      INT NOT NULL DEFAULT 1,
			unit_price    DECIMAL(10,2) NOT NULL DEFAULT 0.00,
			added_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (item_id),
			UNIQUE KEY uk_cart_product (cart_id, product_id),
			INDEX idx_items_cart_id (cart_id),
			CONSTRAINT fk_items_cart FOREIGN KEY (cart_id) REFERENCES carts(cart_id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func (s *MySQLStore) CreateCart(ctx context.Context, req CreateCartRequest) (*Cart, error) {
	cartID := uuid.New().String()
	now := time.Now().UTC()

	_, err := s.db.ExecContext(ctx,
		"INSERT INTO carts (cart_id, customer_id, customer_name, created_at) VALUES (?, ?, ?, ?)",
		cartID, req.CustomerID, req.CustomerName, now)
	if err != nil {
		return nil, fmt.Errorf("insert cart: %w", err)
	}

	return &Cart{
		CartID:       cartID,
		CustomerID:   req.CustomerID,
		CustomerName: req.CustomerName,
		Items:        []CartItem{},
		CreatedAt:    now.Format(time.RFC3339),
		UpdatedAt:    now.Format(time.RFC3339),
	}, nil
}

func (s *MySQLStore) GetCart(ctx context.Context, cartID string) (*Cart, error) {
	// Single query with LEFT JOIN — efficient cart retrieval
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.cart_id, c.customer_id, c.customer_name, c.created_at, c.updated_at,
		       ci.product_id, ci.product_name, ci.quantity, ci.unit_price
		FROM carts c
		LEFT JOIN cart_items ci ON c.cart_id = ci.cart_id
		WHERE c.cart_id = ?`, cartID)
	if err != nil {
		return nil, fmt.Errorf("query cart: %w", err)
	}
	defer rows.Close()

	var cart *Cart
	for rows.Next() {
		var cID, custID string
		var custName sql.NullString
		var createdAt, updatedAt time.Time
		var prodID, prodName sql.NullString
		var qty sql.NullInt64
		var price sql.NullFloat64

		if err := rows.Scan(&cID, &custID, &custName, &createdAt, &updatedAt,
			&prodID, &prodName, &qty, &price); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		if cart == nil {
			cart = &Cart{
				CartID:       cID,
				CustomerID:   custID,
				CustomerName: custName.String,
				Items:        []CartItem{},
				CreatedAt:    createdAt.Format(time.RFC3339),
				UpdatedAt:    updatedAt.Format(time.RFC3339),
			}
		}

		if prodID.Valid {
			cart.Items = append(cart.Items, CartItem{
				ProductID:   prodID.String,
				ProductName: prodName.String,
				Quantity:    int(qty.Int64),
				UnitPrice:   price.Float64,
			})
		}
	}

	if cart == nil {
		return nil, nil // not found
	}
	return cart, nil
}

func (s *MySQLStore) AddItem(ctx context.Context, cartID string, req AddItemRequest) (*Cart, error) {
	// Upsert: insert or update quantity if product already in cart
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO cart_items (cart_id, product_id, product_name, quantity, unit_price)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			quantity = VALUES(quantity),
			product_name = VALUES(product_name),
			unit_price = VALUES(unit_price)`,
		cartID, req.ProductID, req.ProductName, req.Quantity, req.UnitPrice)
	if err != nil {
		return nil, fmt.Errorf("upsert item: %w", err)
	}

	// Return updated cart
	return s.GetCart(ctx, cartID)
}

// ==================== DynamoDB Store ====================

type DynamoStore struct {
	client    *dynamodb.Client
	tableName string
}

func NewDynamoStore() (*DynamoStore, error) {
	region := getEnv("AWS_REGION", "us-west-2")
	tableName := getEnv("DYNAMODB_TABLE", "CS6650HW8-shopping-carts")

	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}

	client := dynamodb.NewFromConfig(cfg)

	log.Printf("[DynamoDB] Connected to table %s in %s", tableName, region)
	return &DynamoStore{client: client, tableName: tableName}, nil
}

// DynamoDB single-table design:
//   PK = "CART#<cart_id>"   SK = "META"              → cart metadata
//   PK = "CART#<cart_id>"   SK = "ITEM#<product_id>" → cart item

func (s *DynamoStore) CreateCart(ctx context.Context, req CreateCartRequest) (*Cart, error) {
	cartID := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &s.tableName,
		Item: map[string]types.AttributeValue{
			"PK":            &types.AttributeValueMemberS{Value: "CART#" + cartID},
			"SK":            &types.AttributeValueMemberS{Value: "META"},
			"cart_id":       &types.AttributeValueMemberS{Value: cartID},
			"customer_id":   &types.AttributeValueMemberS{Value: req.CustomerID},
			"customer_name": &types.AttributeValueMemberS{Value: req.CustomerName},
			"created_at":    &types.AttributeValueMemberS{Value: now},
			"updated_at":    &types.AttributeValueMemberS{Value: now},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("dynamodb put cart: %w", err)
	}

	return &Cart{
		CartID:       cartID,
		CustomerID:   req.CustomerID,
		CustomerName: req.CustomerName,
		Items:        []CartItem{},
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (s *DynamoStore) GetCart(ctx context.Context, cartID string) (*Cart, error) {
	// Query all items with PK = CART#<id> (both META and ITEM# records)
	result, err := s.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              &s.tableName,
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: "CART#" + cartID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("dynamodb query: %w", err)
	}

	if len(result.Items) == 0 {
		return nil, nil // not found
	}

	var cart *Cart
	items := []CartItem{}

	for _, item := range result.Items {
		sk := getAttrS(item, "SK")

		if sk == "META" {
			cart = &Cart{
				CartID:       getAttrS(item, "cart_id"),
				CustomerID:   getAttrS(item, "customer_id"),
				CustomerName: getAttrS(item, "customer_name"),
				CreatedAt:    getAttrS(item, "created_at"),
				UpdatedAt:    getAttrS(item, "updated_at"),
			}
		} else if strings.HasPrefix(sk, "ITEM#") {
			items = append(items, CartItem{
				ProductID:   getAttrS(item, "product_id"),
				ProductName: getAttrS(item, "product_name"),
				Quantity:    getAttrN(item, "quantity"),
				UnitPrice:   getAttrF(item, "unit_price"),
			})
		}
	}

	if cart == nil {
		return nil, nil
	}
	cart.Items = items
	return cart, nil
}

func (s *DynamoStore) AddItem(ctx context.Context, cartID string, req AddItemRequest) (*Cart, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &s.tableName,
		Item: map[string]types.AttributeValue{
			"PK":           &types.AttributeValueMemberS{Value: "CART#" + cartID},
			"SK":           &types.AttributeValueMemberS{Value: "ITEM#" + req.ProductID},
			"product_id":   &types.AttributeValueMemberS{Value: req.ProductID},
			"product_name": &types.AttributeValueMemberS{Value: req.ProductName},
			"quantity":     &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", req.Quantity)},
			"unit_price":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%.2f", req.UnitPrice)},
			"updated_at":   &types.AttributeValueMemberS{Value: now},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("dynamodb put item: %w", err)
	}

	return s.GetCart(ctx, cartID)
}

// ==================== DynamoDB Helpers ====================

func getAttrS(item map[string]types.AttributeValue, key string) string {
	if v, ok := item[key].(*types.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}

func getAttrN(item map[string]types.AttributeValue, key string) int {
	if v, ok := item[key].(*types.AttributeValueMemberN); ok {
		n := 0
		fmt.Sscanf(v.Value, "%d", &n)
		return n
	}
	return 0
}

func getAttrF(item map[string]types.AttributeValue, key string) float64 {
	if v, ok := item[key].(*types.AttributeValueMemberN); ok {
		f := 0.0
		fmt.Sscanf(v.Value, "%f", &f)
		return f
	}
	return 0
}

// ==================== HTTP Handlers ====================

var store CartStore

func main() {
	backend := getEnv("DB_BACKEND", "mysql")
	var err error

	switch backend {
	case "mysql":
		store, err = NewMySQLStore()
	case "dynamodb":
		store, err = NewDynamoStore()
	default:
		log.Fatalf("Unknown DB_BACKEND: %s (use 'mysql' or 'dynamodb')", backend)
	}
	if err != nil {
		log.Fatalf("Failed to init %s store: %v", backend, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/shopping-carts/", corsWrap(handleShoppingCarts))
	mux.HandleFunc("/shopping-carts", corsWrap(handleShoppingCarts))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok", "backend": backend})
	})

	port := getEnv("PORT", "5173")
	log.Printf("Shopping Cart API starting on :%s [backend=%s]", port, backend)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func handleShoppingCarts(w http.ResponseWriter, r *http.Request) {
	// Parse path: /shopping-carts, /shopping-carts/{id}, /shopping-carts/{id}/items
	path := strings.TrimPrefix(r.URL.Path, "/shopping-carts")
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")
	parts := strings.Split(path, "/")

	switch {
	// POST /shopping-carts — create cart
	case r.Method == http.MethodPost && (path == "" || len(parts) == 0 || parts[0] == ""):
		handleCreateCart(w, r)

	// GET /shopping-carts/{id} — get cart
	case r.Method == http.MethodGet && len(parts) == 1 && parts[0] != "":
		handleGetCart(w, r, parts[0])

	// POST /shopping-carts/{id}/items — add item
	case r.Method == http.MethodPost && len(parts) == 2 && parts[1] == "items":
		handleAddItem(w, r, parts[0])

	// OPTIONS for CORS
	case r.Method == http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)

	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// POST /shopping-carts
func handleCreateCart(w http.ResponseWriter, r *http.Request) {
	var req CreateCartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}
	if req.CustomerID == "" {
		writeError(w, http.StatusBadRequest, "customer_id is required")
		return
	}

	cart, err := store.CreateCart(r.Context(), req)
	if err != nil {
		log.Printf("CreateCart error: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create cart")
		return
	}

	writeJSON(w, http.StatusCreated, cart)
}

// GET /shopping-carts/{id}
func handleGetCart(w http.ResponseWriter, r *http.Request, cartID string) {
	cart, err := store.GetCart(r.Context(), cartID)
	if err != nil {
		log.Printf("GetCart error: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to retrieve cart")
		return
	}
	if cart == nil {
		writeError(w, http.StatusNotFound, "Cart not found")
		return
	}

	writeJSON(w, http.StatusOK, cart)
}

// POST /shopping-carts/{id}/items
func handleAddItem(w http.ResponseWriter, r *http.Request, cartID string) {
	// Verify cart exists first
	existing, err := store.GetCart(r.Context(), cartID)
	if err != nil {
		log.Printf("AddItem/GetCart error: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to verify cart")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "Cart not found")
		return
	}

	var req AddItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}
	if req.ProductID == "" {
		writeError(w, http.StatusBadRequest, "product_id is required")
		return
	}
	if req.Quantity <= 0 {
		writeError(w, http.StatusBadRequest, "quantity must be positive")
		return
	}

	cart, err := store.AddItem(r.Context(), cartID, req)
	if err != nil {
		log.Printf("AddItem error: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to add item")
		return
	}

	writeJSON(w, http.StatusCreated, cart)
}

// ==================== Helpers ====================

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{Error: http.StatusText(status), Message: message})
}

func corsWrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
