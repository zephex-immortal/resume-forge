package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

var razorpayKey = "rzp_test_ROWBYwf6K2oOey"
var razorpaySecret = "XwvS8NHXAGY2bLbmhLQg1qYo"

type OrderRequest struct {
	Amount int `json:"amount"` // in paise (₹10 = 1000)
}

type OrderResponse struct {
	OrderID string `json:"order_id"`
	Key     string `json:"key"`
	Amount  int    `json:"amount"`
}

type VerifyRequest struct {
	OrderID   string `json:"razorpay_order_id"`
	PaymentID string `json:"razorpay_payment_id"`
	Signature string `json:"razorpay_signature"`
}

type VerifyResponse struct {
	Valid  bool   `json:"valid"`
	Amount int    `json:"amount"`
	Msg    string `json:"msg"`
}

func main() {
	port := "7001"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	mux := http.NewServeMux()

	// CORS middleware
	handler := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				w.WriteHeader(200)
				return
			}
			h(w, r)
		}
	}

	// Health check
	mux.HandleFunc("/api/health", handler(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))

	// Create order
	mux.HandleFunc("/api/create-order", handler(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}

		var req OrderRequest
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &req)

		if req.Amount <= 0 {
			req.Amount = 1000 // default ₹10 = 1000 paise
		}

		// Call Razorpay API
		payload := fmt.Sprintf(`{"amount":%d,"currency":"INR","receipt":"rcpt_%d"}`, req.Amount, os.Getpid())
		apiReq, _ := http.NewRequest("POST", "https://api.razorpay.com/v1/orders", strings.NewReader(payload))
		apiReq.SetBasicAuth(razorpayKey, razorpaySecret)
		apiReq.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(apiReq)
		if err != nil {
			http.Error(w, "Razorpay API error: "+err.Error(), 500)
			return
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		if resp.StatusCode != 200 {
			http.Error(w, fmt.Sprintf("Razorpay error: %v", result), 500)
			return
		}

		orderID := result["id"].(string)
		json.NewEncoder(w).Encode(OrderResponse{
			OrderID: orderID,
			Key:     razorpayKey,
			Amount:  req.Amount,
		})
	}))

	// Verify payment
	mux.HandleFunc("/api/verify-payment", handler(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}

		var req VerifyRequest
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &req)

		// Generate expected signature
		msg := req.OrderID + "|" + req.PaymentID
		mac := hmac.New(sha256.New, []byte(razorpaySecret))
		mac.Write([]byte(msg))
		expectedSig := hex.EncodeToString(mac.Sum(nil))

		valid := hmac.Equal([]byte(expectedSig), []byte(req.Signature))

		json.NewEncoder(w).Encode(VerifyResponse{
			Valid: valid,
			Msg:   fmt.Sprintf("payment %s", map[bool]string{true: "verified", false: "invalid"}[valid]),
		})
	}))

	fmt.Printf("✦ Backend running on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
