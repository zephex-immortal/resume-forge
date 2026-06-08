package main

import (
	"strings"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

var razorpayKey = "rzp_test_ROWBYwf6K2oOey"
var razorpaySecret = "XwvS8NHXAGY2bLbmhLQg1qYo"

type OrderRequest struct {
	Amount int `json:"amount"`
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

// HTTP client with timeout
var client = &http.Client{Timeout: 15 * time.Second}

// Logging middleware
func corsLog(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(200)
			return
		}
		start := time.Now()
		h(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	}
}

func main() {
	port := "7001"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	// Override with env vars if set
	if k := os.Getenv("RAZORPAY_KEY"); k != "" {
		razorpayKey = k
	}
	if s := os.Getenv("RAZORPAY_SECRET"); s != "" {
		razorpaySecret = s
	}

	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("/api/health", corsLog(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "resume-forge"})
	}))

	// Create Razorpay order
	mux.HandleFunc("/api/create-order", corsLog(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, `{"error":"POST required"}`, 405)
			return
		}

		var req OrderRequest
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &req)

		if req.Amount <= 0 {
			req.Amount = 1000
		}

		// Call Razorpay API to create order
		payload := fmt.Sprintf(`{"amount":%d,"currency":"INR","receipt":"rf_%d"}`, req.Amount, time.Now().UnixMilli())
		apiReq, _ := http.NewRequest("POST", "https://api.razorpay.com/v1/orders", strings.NewReader(payload))
		apiReq.SetBasicAuth(razorpayKey, razorpaySecret)
		apiReq.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(apiReq)
		if err != nil {
			log.Printf("Razorpay API call failed: %v", err)
			http.Error(w, `{"error":"Payment gateway unavailable"}`, 502)
			return
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(respBody, &result)

		if resp.StatusCode != 200 {
			errMsg, _ := json.Marshal(result)
			log.Printf("Razorpay order error: %s", errMsg)
			http.Error(w, fmt.Sprintf(`{"error":"Razorpay: %s"}`, result["message"]), 502)
			return
		}

		orderID, _ := result["id"].(string)
		log.Printf("Order created: %s (₹%.2f)", orderID, float64(req.Amount)/100)

		json.NewEncoder(w).Encode(OrderResponse{
			OrderID: orderID,
			Key:     razorpayKey,
			Amount:  req.Amount,
		})
	}))

	// Verify payment signature
	mux.HandleFunc("/api/verify-payment", corsLog(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, `{"error":"POST required"}`, 405)
			return
		}

		var req VerifyRequest
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, `{"error":"Invalid request body"}`, 400)
			return
		}

		if req.OrderID == "" || req.PaymentID == "" || req.Signature == "" {
			http.Error(w, `{"error":"Missing payment fields"}`, 400)
			return
		}

		// Generate expected HMAC signature
		msg := req.OrderID + "|" + req.PaymentID
		mac := hmac.New(sha256.New, []byte(razorpaySecret))
		mac.Write([]byte(msg))
		expectedSig := hex.EncodeToString(mac.Sum(nil))

		valid := hmac.Equal([]byte(expectedSig), []byte(req.Signature))

		status := "verified"
		if !valid {
			status = "invalid"
		}
		log.Printf("Payment verify: %s | order=%s payment=%s", status, req.OrderID, req.PaymentID)

		json.NewEncoder(w).Encode(VerifyResponse{
			Valid: valid,
			Msg:   fmt.Sprintf("payment %s", status),
		})
	}))

	log.Printf("✦ Resume Forge backend running on port %s", port)
	log.Printf("✦ Razorpay key: %s", razorpayKey[:12]+"...")
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
