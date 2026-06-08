package main

import (
	"bytes"
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
	"time"
)

var razorpayKey = "rzp_test_ROWBYwf6K2oOey"
var razorpaySecret = "XwvS8NHXAGY2bLbmhLQg1qYo"
var geminiKey = "" // Set via env var GEMINI_API_KEY
var corsOrigin = "" // Set via env var CORS_ORIGIN (defaults to *)

var client = &http.Client{Timeout: 30 * time.Second}

type exp struct {
	Company     string
	Role        string
	From        string
	To          string
	Description string
}

type edu struct {
	School string
	Degree string
	From   string
	To     string
}

// ── Request types ──────────────────────────────────────────
type GenerateRequest struct {
	Name             string            `json:"name"`
	Role             string            `json:"role"`
	Email            string            `json:"email"`
	Phone            string            `json:"phone"`
	Location         string            `json:"location"`
	Portfolio        string            `json:"portfolio"`
	Summary          string            `json:"summary"`
	Skills           string            `json:"skills"`
	SkillsCategories SkillsCategories  `json:"skills_categories"`
	Experience       []ExpEntry        `json:"experience"`
	Education        []EduEntry        `json:"education"`
	Template         string            `json:"template"`
}

type SkillsCategories struct {
	Languages  string `json:"skills_languages"`
	Frameworks string `json:"skills_frameworks"`
	Tools      string `json:"skills_tools"`
	Databases  string `json:"skills_databases"`
	Cloud      string `json:"skills_cloud"`
}

type ExpEntry struct {
	Company string `json:"company"`
	Role    string `json:"role"`
	From    string `json:"from"`
	To      string `json:"to"`
	Desc    string `json:"desc"`
}

type EduEntry struct {
	School string `json:"school"`
	Degree string `json:"degree"`
	From   string `json:"from"`
	To     string `json:"to"`
}

type GenerateResponse struct {
	Success   bool   `json:"success"`
	HTML      string `json:"html,omitempty"`
	OrderID   string `json:"order_id,omitempty"`
	Key       string `json:"key,omitempty"`
	Amount    int    `json:"amount,omitempty"`
	PaymentID string `json:"payment_id,omitempty"`
	Message   string `json:"message,omitempty"`
}

// ── CORS middleware ────────────────────────────────────────
func cors(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if corsOrigin != "" && origin == corsOrigin {
			w.Header().Set("Access-Control-Allow-Origin", corsOrigin)
			w.Header().Set("Vary", "Origin")
		} else if corsOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", corsOrigin)
			w.Header().Set("Vary", "Origin")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		h(w, r)
	}
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, msg string, status int) {
	w.WriteHeader(status)
	writeJSON(w, GenerateResponse{Success: false, Message: msg})
}

// ── Step 1: Create Razorpay order ──────────────────────────
func createRazorpayOrder() (string, error) {
	payload := fmt.Sprintf(`{"amount":1000,"currency":"INR","receipt":"rf_%d"}`, time.Now().UnixMilli())
	req, _ := http.NewRequest("POST", "https://api.razorpay.com/v1/orders", strings.NewReader(payload))
	req.SetBasicAuth(razorpayKey, razorpaySecret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("razorpay unreachable: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("razorpay error: %v", result["message"])
	}

	orderID, _ := result["id"].(string)
	if orderID == "" {
		return "", fmt.Errorf("razorpay returned empty order id")
	}
	return orderID, nil
}

// ── Step 2: Verify payment ─────────────────────────────────
func verifyPayment(orderID, paymentID, signature string) bool {
	if orderID == "" || paymentID == "" || signature == "" {
		return false
	}
	msg := orderID + "|" + paymentID
	mac := hmac.New(sha256.New, []byte(razorpaySecret))
	mac.Write([]byte(msg))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// ── Step 3: Generate content with Gemini ───────────────────
func generateWithGemini(data GenerateRequest) (map[string]interface{}, error) {
	if geminiKey == "" {
		return nil, fmt.Errorf("Gemini API key not configured")
	}

	prompt := fmt.Sprintf(`You are a professional resume writer. Generate a JSON resume for a person with these details:

Name: %s
Role: %s
Email: %s
Phone: %s
Location: %s
Portfolio: %s

Summary: %s

Experience: %s
Education: %s
Skills: %s

Return ONLY valid JSON with this exact structure (no markdown, no code fences):
{
  "summary": "well written professional summary...",
  "experience": [
    {"company": "...", "role": "...", "from": "...", "to": "...", "description": "expanded description with achievements..."}
  ],
  "education": [
    {"school": "...", "degree": "...", "from": "...", "to": "..."}
  ]
}

For each experience entry, expand the description with specific achievements and results. Make it compelling.`,
		data.Name, data.Role, data.Email, data.Phone, data.Location, data.Portfolio,
		data.Summary, toJSON(data.Experience), toJSON(data.Education), data.Skills)

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"parts": []map[string]interface{}{
				{"text": prompt},
			}},
		},
		"generationConfig": map[string]interface{}{
			"temperature":    0.7,
			"maxOutputTokens": 2048,
		},
	}
	bodyBytes, _ := json.Marshal(body)

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=%s", geminiKey)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Gemini API unreachable: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	if resp.StatusCode != 200 {
		errMsg, _ := json.Marshal(result)
		return nil, fmt.Errorf("Gemini API error (%d): %s", resp.StatusCode, errMsg)
	}

	// Extract text from response
	candidates, _ := result["candidates"].([]interface{})
	if len(candidates) == 0 {
		return nil, fmt.Errorf("Gemini returned no candidates")
	}
	candidate, _ := candidates[0].(map[string]interface{})
	content, _ := candidate["content"].(map[string]interface{})
	parts, _ := content["parts"].([]interface{})
	if len(parts) == 0 {
		return nil, fmt.Errorf("Gemini returned no content parts")
	}
	part, _ := parts[0].(map[string]interface{})
	text, _ := part["text"].(string)

	// Extract JSON from response
	jsonStr := text
	if idx := strings.Index(text, "```"); idx >= 0 {
		end := strings.LastIndex(text, "```")
		if end > idx {
			jsonStr = text[idx+3 : end]
			jsonStr = strings.TrimPrefix(jsonStr, "json")
			jsonStr = strings.TrimSpace(jsonStr)
		}
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		// If parsing fails, return the raw text as summary
		return map[string]interface{}{
			"summary":    text[:min(len(text), 500)],
			"experience": toSliceInterface(data.Experience),
			"education":  toSliceInterface(data.Education),
		}, nil
	}

	return parsed, nil
}

// ── Step 4: Render HTML ───────────────────────────────────
func renderResumeHTML(data GenerateRequest, aiContent map[string]interface{}) string {
	skills := []string{}
	if data.Skills != "" {
		for _, s := range strings.Split(data.Skills, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				skills = append(skills, s)
			}
		}
	}

	categorizedSkills := parseCategorizedSkills(data.SkillsCategories)
	hasCategorized := len(categorizedSkills) > 0

	summary := ""
	if aiContent != nil {
		if s, ok := aiContent["summary"].(string); ok {
			summary = s
		}
	}
	if summary == "" {
		summary = data.Summary
	}
	if summary == "" {
		summary = "Professional with experience in software development and technology."
	}

	// Build experiences from AI content or fallback to form data
	var experiences []exp

	if aiContent != nil {
		if aiExp, ok := aiContent["experience"].([]interface{}); ok {
			for _, e := range aiExp {
				if em, ok := e.(map[string]interface{}); ok {
					experiences = append(experiences, exp{
						Company:     toString(em["company"]),
						Role:        toString(em["role"]),
						From:        toString(em["from"]),
						To:          toString(em["to"]),
						Description: toString(em["description"]),
					})
				}
			}
		}
	}
	if len(experiences) == 0 {
		for _, e := range data.Experience {
			experiences = append(experiences, exp{
				Company: e.Company, Role: e.Role,
				From: e.From, To: e.To,
				Description: e.Desc,
			})
		}
	}

	var educations []edu
	if aiContent != nil {
		if aiEdu, ok := aiContent["education"].([]interface{}); ok {
			for _, e := range aiEdu {
				if em, ok := e.(map[string]interface{}); ok {
					educations = append(educations, edu{
						School: toString(em["school"]),
						Degree: toString(em["degree"]),
						From:   toString(em["from"]),
						To:     toString(em["to"]),
					})
				}
			}
		}
	}
	if len(educations) == 0 {
		for _, e := range data.Education {
			educations = append(educations, edu{
				School: e.School, Degree: e.Degree,
				From: e.From, To: e.To,
			})
		}
	}

	escape := func(s string) string {
		s = strings.ReplaceAll(s, "&", "&amp;")
		s = strings.ReplaceAll(s, "<", "&lt;")
		s = strings.ReplaceAll(s, ">", "&gt;")
		s = strings.ReplaceAll(s, "\"", "&quot;")
		s = strings.ReplaceAll(s, "'", "&#39;")
		return s
	}

	switch data.Template {
	case "terminal":
		return fmt.Sprintf(`<div class="resume-output resume-terminal" style="width:210mm;height:297mm;background:#fff;color:#111;font-family:'Inter','Segoe UI',sans-serif;box-shadow:0 0 30px rgba(0,0,0,0.3);border-radius:2px;overflow:hidden;">
			<div style="background:#0a0a0e;color:#00ffc8;padding:0.8rem 1.5rem 0.4rem;font-family:'Courier New',monospace;">
				<h1 style="font-size:1.4rem;font-weight:bold;letter-spacing:-0.5px;">%s</h1>
				<div style="color:#6a6a7a;font-size:0.75rem;margin-top:0.15rem;">%s</div>
				<div style="display:flex;flex-wrap:wrap;gap:0.8rem;margin-top:0.6rem;font-size:0.6rem;color:#6a6a7a;">%s%s%s%s</div>
			</div>
			<div style="padding:0.3rem 1.5rem 0.8rem;color:#1a1a1a;">
				<div style="margin-bottom:0.5rem;"><div style="font-size:0.7rem;font-weight:bold;color:#00cc9e;text-transform:uppercase;letter-spacing:1px;border-bottom:1px solid #e0e0e0;padding-bottom:0.2rem;margin-bottom:0.4rem;">// summary</div><div style="font-size:0.65rem;color:#333;line-height:1.35;">%s</div></div>
				%s
				%s
				%s
			</div>
		</div>`,
			escape(data.Name), escape(data.Role),
			cond(data.Email != "", `<span>✉ `+escape(data.Email)+`</span>`, ""),
			cond(data.Phone != "", `<span>📞 `+escape(data.Phone)+`</span>`, ""),
			cond(data.Location != "", `<span>📍 `+escape(data.Location)+`</span>`, ""),
			cond(data.Portfolio != "", `<span>🔗 `+escape(data.Portfolio)+`</span>`, ""),
			escape(summary),
			renderExpBlock("experience", experiences, escape, "rt"),
			renderEduBlock("education", educations, escape, "rt"),
			renderSkillsBlockCategorized(skills, categorizedSkills, hasCategorized, escape, "rt"),
		)

	case "minimal":
		return fmt.Sprintf(`<div class="resume-output resume-minimal" style="width:210mm;height:297mm;background:#fff;color:#111;font-family:'Inter','Segoe UI',sans-serif;box-shadow:0 0 30px rgba(0,0,0,0.3);border-radius:2px;overflow:hidden;">
			<div style="text-align:center;padding:0.8rem 1.5rem 0.4rem;border-bottom:2px solid #111;">
				<h1 style="font-size:1.3rem;font-weight:700;letter-spacing:1px;">%s</h1>
				<div style="color:#555;font-size:0.75rem;margin-top:0.1rem;">%s</div>
				<div style="font-size:0.6rem;color:#555;margin-top:0.3rem;display:flex;justify-content:center;flex-wrap:wrap;gap:0.4rem;">%s%s%s%s</div>
			</div>
			<div style="padding:0.3rem 1.5rem 0.8rem;">
				<div style="font-size:0.75rem;font-weight:700;text-transform:uppercase;letter-spacing:1.5px;color:#111;margin-bottom:0.3rem;">%s</div>
				<p style="font-size:0.65rem;margin:0.3rem 0;line-height:1.35;color:#333;">%s</p>
				%s%s%s
			</div>
		</div>`,
			escape(data.Name), escape(data.Role),
			cond(data.Email != "", `<span>`+escape(data.Email)+`</span>`, ""),
			cond(data.Phone != "", `<span>`+escape(data.Phone)+`</span>`, ""),
			cond(data.Location != "", `<span>`+escape(data.Location)+`</span>`, ""),
			cond(data.Portfolio != "", `<span>`+escape(data.Portfolio)+`</span>`, ""),
			strings.Repeat("\u2500", 30), escape(summary),
			renderExpBlock("Experience", experiences, escape, "rm"),
			renderEduBlock("Education", educations, escape, "rm"),
			renderSkillsBlockCategorized(skills, categorizedSkills, hasCategorized, escape, "rm"),
		)

	case "modern":
		return fmt.Sprintf(`<div class="resume-output resume-modern" style="width:210mm;height:297mm;display:flex;background:#fff;box-shadow:0 0 30px rgba(0,0,0,0.3);border-radius:2px;overflow:hidden;">
			<div style="width:35%%;background:#1a1a2e;color:#eee;padding:1.5rem;">
				<h1 style="font-size:1.2rem;font-weight:700;color:#fff;">%s</h1>
				<div style="font-size:0.68rem;color:#00ffc8;margin-top:0.1rem;">%s</div>
				<div style="font-size:0.6rem;margin-top:0.3rem;line-height:1.4;color:#aaa;">%s%s%s%s</div>
				%s%s
			</div>
			<div style="width:65%%;padding:1.5rem;color:#1a1a1a;">
				<div style="margin-bottom:1rem;"><div style="font-size:0.68rem;font-weight:700;text-transform:uppercase;letter-spacing:1px;color:#1a1a2e;border-bottom:1px solid #eee;padding-bottom:0.15rem;margin-bottom:0.2rem;">About</div><p style="font-size:0.65rem;color:#333;line-height:1.35;">%s</p></div>
				%s
			</div>
		</div>`,
			escape(data.Name), escape(data.Role),
			cond(data.Email != "", `<div>✉ `+escape(data.Email)+`</div>`, ""),
			cond(data.Phone != "", `<div>📞 `+escape(data.Phone)+`</div>`, ""),
			cond(data.Location != "", `<div>📍 `+escape(data.Location)+`</div>`, ""),
			cond(data.Portfolio != "", `<div>🔗 `+escape(data.Portfolio)+`</div>`, ""),
			renderSidebarSkillsCategorized(skills, categorizedSkills, hasCategorized, escape),
			renderSidebarEdu(educations, escape),
			escape(summary),
			renderExpBlock("Experience", experiences, escape, "mod"),
		)


	case "executive":
		return fmt.Sprintf(`<div class="resume-output resume-executive" style="width:210mm;height:297mm;background:#fff;color:#222;font-family:'Georgia','Times New Roman',serif;box-shadow:0 0 30px rgba(0,0,0,0.3);border-radius:2px;overflow:hidden;">
			<div style="background:#0a1628;padding:1.8rem 2rem 1.2rem;text-align:center;">
				<h1 style="font-size:1.8rem;font-weight:700;color:#fff;margin:0;letter-spacing:1px;font-family:'Georgia',serif;">%s</h1>
				<div style="font-size:0.9rem;color:#c9a93e;margin-top:0.2rem;font-weight:600;font-style:italic;">%s</div>
				<div style="font-size:0.7rem;color:#8899aa;margin-top:0.5rem;display:flex;justify-content:center;flex-wrap:wrap;gap:0.8rem;">%s%s%s%s</div>
			</div>
			<div style="padding:1.2rem 2rem 2rem;">
				<div style="margin-bottom:1rem;"><div style="font-size:0.7rem;font-weight:700;color:#c9a93e;text-transform:uppercase;letter-spacing:1.5px;border-bottom:1px solid #c9a93e;padding-bottom:0.2rem;margin-bottom:0.4rem;font-family:'Georgia',serif;">Professional Summary</div><p style="font-size:0.72rem;color:#333;line-height:1.6;font-family:'Georgia',serif;">%s</p></div>
				%s
				%s
				%s
			</div>
		</div>`,
			escape(data.Name), escape(data.Role),
			cond(data.Email != "", `<span style="color:#c9a93e;">✉</span> `+escape(data.Email)+``, ""),
			cond(data.Phone != "", `<span style="color:#c9a93e;">📞</span> `+escape(data.Phone)+``, ""),
			cond(data.Location != "", `<span style="color:#c9a93e;">📍</span> `+escape(data.Location)+``, ""),
			cond(data.Portfolio != "", `<span style="color:#c9a93e;">🔗</span> `+escape(data.Portfolio)+``, ""),
			escape(summary),
			renderExpBlockExecutive(experiences, escape),
			renderEduBlockExecutive(educations, escape),
			renderSkillsBlockCategorized(skills, categorizedSkills, hasCategorized, escape, "exec"),
		)

	case "creative":
		return fmt.Sprintf(`<div class="resume-output resume-creative" style="width:210mm;height:297mm;background:#fff;color:#222;font-family:'Poppins','Segoe UI',sans-serif;box-shadow:0 0 30px rgba(0,0,0,0.3);border-radius:2px;overflow:hidden;">
			<div style="background:linear-gradient(135deg,#ff6b6b,#ffd93d);padding:1.5rem 2rem 1rem;text-align:center;border-radius:0 0 20px 20px;">
				<h1 style="font-size:2rem;font-weight:800;color:#fff;margin:0;text-shadow:2px 2px 4px rgba(0,0,0,0.2);">%s</h1>
				<div style="font-size:0.95rem;color:#fff;margin-top:0.15rem;font-weight:600;text-shadow:1px 1px 2px rgba(0,0,0,0.2);">%s</div>
				<div style="font-size:0.7rem;color:#fff;margin-top:0.5rem;display:flex;justify-content:center;flex-wrap:wrap;gap:0.6rem;text-shadow:1px 1px 2px rgba(0,0,0,0.1);">%s%s%s%s</div>
			</div>
			<div style="padding:1.2rem 2rem 2rem;">
				<div style="margin-bottom:1rem;background:#fef9ef;border-radius:12px;padding:0.8rem;"><div style="font-size:0.7rem;font-weight:700;color:#ff6b6b;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.3rem;">✨ About Me</div><p style="font-size:0.65rem;color:#333;line-height:1.35;">%s</p></div>
				%s
				%s
				%s
			</div>
		</div>`,
			escape(data.Name), escape(data.Role),
			cond(data.Email != "", `<span>📧 `+escape(data.Email)+`</span>`, ""),
			cond(data.Phone != "", `<span>📱 `+escape(data.Phone)+`</span>`, ""),
			cond(data.Location != "", `<span>📍 `+escape(data.Location)+`</span>`, ""),
			cond(data.Portfolio != "", `<span>🌐 `+escape(data.Portfolio)+`</span>`, ""),
			escape(summary),
			renderExpBlockCreative(experiences, escape),
			renderEduBlockCreative(educations, escape),
			renderSkillsBlockCategorized(skills, categorizedSkills, hasCategorized, escape, "cr"),
		)

	case "timeline":
		return fmt.Sprintf(`<div class="resume-output resume-timeline" style="width:210mm;height:297mm;background:#fff;color:#222;font-family:'Inter','Segoe UI',sans-serif;box-shadow:0 0 30px rgba(0,0,0,0.3);border-radius:2px;overflow:hidden;">
			<div style="background:#2d3436;padding:1.5rem 2rem 1rem;text-align:center;">
				<h1 style="font-size:1.6rem;font-weight:700;color:#fff;margin:0;letter-spacing:0.5px;">%s</h1>
				<div style="font-size:0.85rem;color:#dfe6e9;margin-top:0.15rem;">%s</div>
				<div style="font-size:0.65rem;color:#b2bec3;margin-top:0.4rem;display:flex;justify-content:center;flex-wrap:wrap;gap:0.6rem;">%s%s%s%s</div>
			</div>
			<div style="padding:1.2rem 2rem 2rem;">
				<div style="margin-bottom:1rem;"><div style="font-size:0.7rem;font-weight:700;color:#6c5ce7;text-transform:uppercase;letter-spacing:1.5px;border-bottom:2px solid #6c5ce7;padding-bottom:0.2rem;margin-bottom:0.4rem;">About</div><p style="font-size:0.65rem;color:#333;line-height:1.35;">%s</p></div>
				%s
				%s
				%s
			</div>
		</div>`,
			escape(data.Name), escape(data.Role),
			cond(data.Email != "", `<span>✉ `+escape(data.Email)+`</span>`, ""),
			cond(data.Phone != "", `<span>📞 `+escape(data.Phone)+`</span>`, ""),
			cond(data.Location != "", `<span>📍 `+escape(data.Location)+`</span>`, ""),
			cond(data.Portfolio != "", `<span>🔗 `+escape(data.Portfolio)+`</span>`, ""),
			escape(summary),
			renderExpBlockTimeline(experiences, escape),
			renderEduBlockTimeline(educations, escape),
			renderSkillsBlockCategorized(skills, categorizedSkills, hasCategorized, escape, "tl"),
		)

	case "columns":
		return fmt.Sprintf(`<div class="resume-output resume-columns" style="width:210mm;height:297mm;display:flex;background:#fff;box-shadow:0 0 30px rgba(0,0,0,0.3);border-radius:2px;overflow:hidden;">
			<div style="width:50%%;padding:1.5rem;border-right:1px solid #dfe6e9;">
				<div style="margin-bottom:1rem;">
					<h1 style="font-size:1.5rem;font-weight:700;color:#2d3436;margin:0;">%s</h1>
					<div style="font-size:0.8rem;color:#636e72;margin-top:0.1rem;">%s</div>
					<div style="font-size:0.65rem;color:#636e72;margin-top:0.4rem;line-height:1.6;">%s%s%s%s</div>
				</div>
				<div style="margin-bottom:1rem;"><div style="font-size:0.7rem;font-weight:700;color:#2d3436;text-transform:uppercase;letter-spacing:1px;border-bottom:1px solid #dfe6e9;padding-bottom:0.2rem;margin-bottom:0.4rem;">Summary</div><p style="font-size:0.65rem;color:#333;line-height:1.35;">%s</p></div>
				%s
			</div>
			<div style="width:50%%;padding:1.5rem;">
				%s
				%s
			</div>
		</div>`,
			escape(data.Name), escape(data.Role),
			cond(data.Email != "", `<div>✉ `+escape(data.Email)+`</div>`, ""),
			cond(data.Phone != "", `<div>📞 `+escape(data.Phone)+`</div>`, ""),
			cond(data.Location != "", `<div>📍 `+escape(data.Location)+`</div>`, ""),
			cond(data.Portfolio != "", `<div>🔗 `+escape(data.Portfolio)+`</div>`, ""),
			escape(summary),
			renderSkillsBlockCategorized(skills, categorizedSkills, hasCategorized, escape, "cl"),
			renderExpBlock("Experience", experiences, escape, "cp"),
			renderEduBlock("Education", educations, escape, "cp"),
		)
	default: // compact
		return fmt.Sprintf(`<div class="resume-output resume-compact" style="width:210mm;height:297mm;background:#fff;color:#222;font-family:'Segoe UI',Arial,sans-serif;padding:0.4rem 1.2rem;box-shadow:0 0 30px rgba(0,0,0,0.3);border-radius:2px;">
			<div style="border-bottom:2px solid #00cc9e;padding-bottom:0.5rem;margin-bottom:0.8rem;">
				<h1 style="font-size:1.2rem;font-weight:700;margin:0;color:#0a0a0e;">%s</h1>
				<div style="font-size:0.8rem;color:#555;margin-top:0.15rem;">%s</div>
				<div style="font-size:0.6rem;color:#777;margin-top:0.2rem;display:flex;flex-wrap:wrap;gap:0.3rem;">%s%s%s%s</div>
			</div>
			<div style="margin-bottom:0.8rem;"><div style="font-size:0.62rem;font-weight:600;color:#00cc9e;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.15rem;">Summary</div><p style="font-size:0.72rem;line-height:1.5;color:#333;margin:0;">%s</p></div>
			%s%s%s
		</div>`,
			escape(data.Name), escape(data.Role),
			cond(data.Email != "", `<span>✉ `+escape(data.Email)+`</span>`, ""),
			cond(data.Phone != "", `<span>📞 `+escape(data.Phone)+`</span>`, ""),
			cond(data.Location != "", `<span>📍 `+escape(data.Location)+`</span>`, ""),
			cond(data.Portfolio != "", `<span>🔗 `+escape(data.Portfolio)+`</span>`, ""),
			escape(summary),
			renderExpBlock("Experience", experiences, escape, "cp"),
			renderEduBlock("Education", educations, escape, "cp"),
			renderSkillsBlockCategorized(skills, categorizedSkills, hasCategorized, escape, "cp"),
		)
	}
}

func renderExpBlock(title string, items []exp, escape func(string) string, style string) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	if style == "rt" {
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1rem;"><div style="font-size:0.7rem;font-weight:bold;color:#00cc9e;text-transform:uppercase;letter-spacing:1px;border-bottom:1px solid #e0e0e0;padding-bottom:0.15rem;margin-bottom:0.25rem;">// %s</div>`, title))
		for _, e := range items {
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.25rem;"><div style="font-size:0.72rem;font-weight:600;">%s @ %s</div><div style="font-size:0.65rem;color:#555;">%s — %s</div><div style="font-size:0.65rem;color:#333;margin-top:0.1rem;line-height:1.3;">%s</div></div>`,
				escape(e.Role), escape(e.Company), escape(e.From), escape(e.To), escape(e.Description)))
		}
		b.WriteString(`</div>`)
	} else if style == "rm" {
		b.WriteString(fmt.Sprintf(`<div style="margin-top:0.4rem;"><div style="font-size:0.68rem;font-weight:700;text-transform:uppercase;letter-spacing:1px;color:#111;margin-bottom:0.2rem;">%s</div>`, title))
		for _, e := range items {
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.2rem;"><div style="font-size:0.72rem;font-weight:600;">%s — %s</div><div style="font-size:0.62rem;color:#555;">%s — %s</div><div style="font-size:0.62rem;color:#333;margin-top:0.05rem;">%s</div></div>`,
				escape(e.Role), escape(e.Company), escape(e.From), escape(e.To), escape(e.Description)))
		}
		b.WriteString(`</div>`)
	} else if style == "mod" {
		b.WriteString(`<div>`)
		for _, e := range items {
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.25rem;"><div style="font-size:0.72rem;font-weight:600;">%s</div><div style="font-size:0.62rem;color:#555;">%s · %s — %s</div><p style="font-size:0.62rem;color:#444;margin-top:0.05rem;line-height:1.3;">%s</p></div>`,
				escape(e.Role), escape(e.Company), escape(e.From), escape(e.To), escape(e.Description)))
		}
		b.WriteString(`</div>`)
	} else { // compact
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.4rem;"><div style="font-size:0.62rem;font-weight:600;color:#00cc9e;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.2rem;">%s</div>`, title))
		for _, e := range items {
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.2rem;"><div style="font-size:0.7rem;font-weight:600;">%s — <span style="font-weight:400;color:#555;">%s</span></div><div style="font-size:0.6rem;color:#777;">%s — %s</div><div style="font-size:0.62rem;color:#444;margin-top:0.05rem;line-height:1.3;">%s</div></div>`,
				escape(e.Role), escape(e.Company), escape(e.From), escape(e.To), escape(e.Description)))
		}
		b.WriteString(`</div>`)
	}
	return b.String()
}

func renderEduBlock(title string, items []edu, escape func(string) string, style string) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	if style == "rt" {
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1rem;"><div style="font-size:0.7rem;font-weight:bold;color:#00cc9e;text-transform:uppercase;letter-spacing:1px;border-bottom:1px solid #e0e0e0;padding-bottom:0.15rem;margin-bottom:0.25rem;">// %s</div>`, title))
		for _, e := range items {
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.2rem;"><div style="font-size:0.72rem;font-weight:600;">%s</div><div style="font-size:0.65rem;color:#555;">%s</div><div style="font-size:0.6rem;color:#777;">%s — %s</div></div>`,
				escape(e.Degree), escape(e.School), escape(e.From), escape(e.To)))
		}
		b.WriteString(`</div>`)
	} else if style == "rm" {
		b.WriteString(fmt.Sprintf(`<div style="margin-top:0.4rem;"><div style="font-size:0.68rem;font-weight:700;text-transform:uppercase;letter-spacing:1px;color:#111;margin-bottom:0.2rem;">%s</div>`, title))
		for _, e := range items {
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.15rem;"><div style="font-size:0.7rem;font-weight:600;">%s</div><div style="font-size:0.62rem;color:#555;">%s — %s — %s</div></div>`,
				escape(e.Degree), escape(e.School), escape(e.From), escape(e.To)))
		}
		b.WriteString(`</div>`)
	} else { // compact
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.4rem;"><div style="font-size:0.62rem;font-weight:600;color:#00cc9e;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.15rem;">%s</div>`, title))
		for _, e := range items {
			b.WriteString(fmt.Sprintf(`<div style="font-size:0.68rem;margin-bottom:0.1rem;"><span style="font-weight:600;">%s</span> — %s <span style="color:#777;">(%s — %s)</span></div>`,
				escape(e.Degree), escape(e.School), escape(e.From), escape(e.To)))
		}
		b.WriteString(`</div>`)
	}
	return b.String()
}

func renderSkillsBlock(skills []string, escape func(string) string, style string) string {
	if len(skills) == 0 {
		return ""
	}
	var tags []string
	for _, s := range skills {
		tags = append(tags, fmt.Sprintf(`<span style="font-size:0.65rem;padding:0.1rem 0.5rem;border:1px solid #ddd;border-radius:2px;color:#333;">%s</span>`, escape(s)))
	}
	joined := strings.Join(tags, "")

	if style == "rt" {
		return fmt.Sprintf(`<div style="margin-bottom:0.5rem;"><div style="font-size:0.7rem;font-weight:bold;color:#00cc9e;text-transform:uppercase;letter-spacing:1px;border-bottom:1px solid #e0e0e0;padding-bottom:0.2rem;margin-bottom:0.4rem;">// skills</div><div style="display:flex;flex-wrap:wrap;gap:0.3rem;">%s</div></div>`, joined)
	}
	return fmt.Sprintf(`<div style="margin-top:0.4rem;"><div style="font-size:0.68rem;font-weight:700;text-transform:uppercase;letter-spacing:1px;color:#111;margin-bottom:0.15rem;">Skills</div><div style="display:flex;flex-wrap:wrap;gap:0.2rem;margin-top:0.15rem;">%s</div></div>`, joined)
}

func renderSkillsBlockCategorized(skills []string, categorizedSkills map[string][]string, hasCategorized bool, escape func(string) string, style string) string {
	if hasCategorized {
		return renderSkillsByCategory(categorizedSkills, escape, style)
	}
	return renderSkillsBlock(skills, escape, style)
}

func renderSidebarSkillsCategorized(skills []string, categorizedSkills map[string][]string, hasCategorized bool, escape func(string) string) string {
	if hasCategorized {
		return renderSkillsByCategory(categorizedSkills, escape, "mod")
	}
	return renderSidebarSkills(skills, escape)
}

func renderSidebarSkills(skills []string, escape func(string) string) string {
	if len(skills) == 0 {
		return ""
	}
	var items []string
	for _, s := range skills {
		items = append(items, fmt.Sprintf(`<div style="font-size:0.65rem;color:#ccc;margin-bottom:0.15rem;">▸ %s</div>`, escape(s)))
	}
	return fmt.Sprintf(`<div><div style="font-size:0.7rem;font-weight:600;text-transform:uppercase;letter-spacing:1px;color:#00ffc8;margin-top:1rem;margin-bottom:0.3rem;border-bottom:1px solid rgba(255,255,255,0.1);padding-bottom:0.2rem;">Skills</div>%s</div>`, strings.Join(items, ""))
}

func renderSidebarEdu(items []edu, escape func(string) string) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<div><div style="font-size:0.7rem;font-weight:600;text-transform:uppercase;letter-spacing:1px;color:#00ffc8;margin-top:1rem;margin-bottom:0.3rem;border-bottom:1px solid rgba(255,255,255,0.1);padding-bottom:0.2rem;">Education</div>`)
	for _, e := range items {
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.4rem;"><div style="font-size:0.7rem;font-weight:600;color:#fff;">%s</div><div style="font-size:0.62rem;color:#aaa;">%s</div><div style="font-size:0.6rem;color:#777;">%s — %s</div></div>`,
			escape(e.Degree), escape(e.School), escape(e.From), escape(e.To)))
	}
	b.WriteString(`</div>`)
	return b.String()
}

func parseCategorizedSkills(cat SkillsCategories) map[string][]string {
	result := make(map[string][]string)
	categories := map[string]string{
		"Languages":  cat.Languages,
		"Frameworks": cat.Frameworks,
		"Tools":      cat.Tools,
		"Databases":  cat.Databases,
		"Cloud":      cat.Cloud,
	}
	for key, val := range categories {
		if val != "" {
			var items []string
			for _, s := range strings.Split(val, ",") {
				s = strings.TrimSpace(s)
				if s != "" {
					items = append(items, s)
				}
			}
			if len(items) > 0 {
				result[key] = items
			}
		}
	}
	return result
}

func renderSkillsByCategory(categorizedSkills map[string][]string, escape func(string) string, style string) string {
	if len(categorizedSkills) == 0 {
		return ""
	}
	var b strings.Builder
	// Determine category order
	categoryOrder := []string{"Languages", "Frameworks", "Tools", "Databases", "Cloud"}
	displayNames := map[string]string{
		"Languages":  "Languages",
		"Frameworks": "Frameworks & Libraries",
		"Tools":      "Tools & Platforms",
		"Databases":  "Databases",
		"Cloud":      "Cloud & DevOps",
	}

	if style == "rt" {
		b.WriteString(`<div style="margin-bottom:1rem;">`)
		b.WriteString(`<div style="font-size:0.8rem;font-weight:bold;color:#00cc9e;text-transform:uppercase;letter-spacing:1px;border-bottom:1px solid #e0e0e0;padding-bottom:0.2rem;margin-bottom:0.4rem;">// skills</div>`)
		for _, cat := range categoryOrder {
			items, ok := categorizedSkills[cat]
			if !ok || len(items) == 0 {
				continue
			}
			var tags []string
			for _, s := range items {
				tags = append(tags, fmt.Sprintf(`<span style="font-size:0.65rem;padding:0.1rem 0.5rem;border:1px solid #ddd;border-radius:2px;color:#333;">%s</span>`, escape(s)))
			}
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.3rem;"><span style="font-size:0.68rem;font-weight:600;color:#555;">%s:</span> %s</div>`, displayNames[cat], strings.Join(tags, " ")))
		}
		b.WriteString(`</div>`)
	} else if style == "rm" {
		b.WriteString(`<div style="margin-top:0.8rem;"><div style="font-size:0.75rem;font-weight:700;text-transform:uppercase;letter-spacing:1.5px;color:#111;margin-bottom:0.3rem;">Skills</div>`)
		for _, cat := range categoryOrder {
			items, ok := categorizedSkills[cat]
			if !ok || len(items) == 0 {
				continue
			}
			var tags []string
			for _, s := range items {
				tags = append(tags, fmt.Sprintf(`<span style="font-size:0.65rem;padding:0.1rem 0.5rem;border:1px solid #ddd;border-radius:2px;color:#333;">%s</span>`, escape(s)))
			}
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.2rem;"><span style="font-size:0.7rem;font-weight:600;color:#555;">%s:</span> %s</div>`, displayNames[cat], strings.Join(tags, " ")))
		}
		b.WriteString(`</div>`)
	} else if style == "mod" {
		// Sidebar style for modern
		b.WriteString(`<div><div style="font-size:0.7rem;font-weight:600;text-transform:uppercase;letter-spacing:1px;color:#00ffc8;margin-top:1rem;margin-bottom:0.3rem;border-bottom:1px solid rgba(255,255,255,0.1);padding-bottom:0.2rem;">Skills</div>`)
		for _, cat := range categoryOrder {
			items, ok := categorizedSkills[cat]
			if !ok || len(items) == 0 {
				continue
			}
			b.WriteString(fmt.Sprintf(`<div style="margin-top:0.4rem;"><div style="font-size:0.62rem;font-weight:600;color:#00ffc8;text-transform:uppercase;">%s</div>`, displayNames[cat]))
			for _, s := range items {
				b.WriteString(fmt.Sprintf(`<div style="font-size:0.65rem;color:#ccc;margin-bottom:0.15rem;">▸ %s</div>`, escape(s)))
			}
			b.WriteString(`</div>`)
		}
		b.WriteString(`</div>`)
	} else if style == "exec" {
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1rem;"><div style="font-size:0.65rem;font-weight:600;color:#c9a93e;text-transform:uppercase;letter-spacing:1px;border-bottom:1px solid #c9a93e;padding-bottom:0.15rem;margin-bottom:0.25rem;">Technical Skills</div>`))
		for _, cat := range categoryOrder {
			items, ok := categorizedSkills[cat]
			if !ok || len(items) == 0 {
				continue
			}
			var tags []string
			for _, s := range items {
				tags = append(tags, fmt.Sprintf(`<span style="font-size:0.6rem;padding:0.05rem 0.4rem;background:#f8f6f0;border:1px solid #c9a93e;border-radius:2px;color:#222;font-family:'Georgia',serif;">%s</span>`, escape(s)))
			}
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.25rem;"><span style="font-size:0.7rem;font-weight:600;color:#1a252c;font-family:'Georgia',serif;">%s:</span> %s</div>`, displayNames[cat], strings.Join(tags, " ")))
		}
		b.WriteString(`</div>`)
	} else if style == "cr" {
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1rem;"><div style="font-size:0.75rem;font-weight:700;color:#ff6b6b;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.4rem;">⚡ Skills</div>`))
		for _, cat := range categoryOrder {
			items, ok := categorizedSkills[cat]
			if !ok || len(items) == 0 {
				continue
			}
			var tags []string
			colors := []string{"#ff6b6b", "#ffd93d", "#6bcb77", "#4d96ff", "#c084fc"}
			ci := 0
			for _, s := range items {
				c := colors[ci%len(colors)]
				tags = append(tags, fmt.Sprintf(`<span style="font-size:0.6rem;padding:0.1rem 0.4rem;background:%s20;color:%s;border:1px solid %s;border-radius:8px;font-weight:500;">%s</span>`, c, c, c, escape(s)))
				ci++
			}
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.3rem;"><span style="font-size:0.7rem;font-weight:700;color:#555;">%s</span><br>%s</div>`, displayNames[cat], strings.Join(tags, " ")))
		}
		b.WriteString(`</div>`)
	} else if style == "tl" {
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1rem;"><div style="font-size:0.75rem;font-weight:700;color:#2d3436;text-transform:uppercase;letter-spacing:1.5px;border-bottom:2px solid #6c5ce7;padding-bottom:0.2rem;margin-bottom:0.5rem;">Skills</div>`))
		for _, cat := range categoryOrder {
			items, ok := categorizedSkills[cat]
			if !ok || len(items) == 0 {
				continue
			}
			var tags []string
			for _, s := range items {
				tags = append(tags, fmt.Sprintf(`<span style="font-size:0.6rem;padding:0.05rem 0.4rem;background:#e8e0f7;color:#6c5ce7;border-radius:3px;font-weight:500;">%s</span>`, escape(s)))
			}
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.2rem;"><span style="font-size:0.7rem;font-weight:600;color:#6c5ce7;">%s:</span> %s</div>`, displayNames[cat], strings.Join(tags, " ")))
		}
		b.WriteString(`</div>`)
	} else if style == "cl" {
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.8rem;"><div style="font-size:0.7rem;font-weight:700;color:#2d3436;text-transform:uppercase;letter-spacing:1px;border-bottom:1px solid #dfe6e9;padding-bottom:0.2rem;margin-bottom:0.4rem;">Technical Skills</div>`))
		for _, cat := range categoryOrder {
			items, ok := categorizedSkills[cat]
			if !ok || len(items) == 0 {
				continue
			}
			var tags []string
			for _, s := range items {
				tags = append(tags, fmt.Sprintf(`<span style="font-size:0.6rem;padding:0.05rem 0.3rem;background:#e8f0fe;color:#2d3436;border-radius:2px;">%s</span>`, escape(s)))
			}
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.2rem;"><span style="font-size:0.68rem;font-weight:600;color:#555;">%s:</span> %s</div>`, displayNames[cat], strings.Join(tags, " ")))
		}
		b.WriteString(`</div>`)
	} else { // compact, cp
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.8rem;"><div style="font-size:0.7rem;font-weight:600;color:#00cc9e;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.2rem;">Skills</div>`))
		for _, cat := range categoryOrder {
			items, ok := categorizedSkills[cat]
			if !ok || len(items) == 0 {
				continue
			}
			var tags []string
			for _, s := range items {
				tags = append(tags, fmt.Sprintf(`<span style="font-size:0.65rem;padding:0.1rem 0.5rem;border:1px solid #ddd;border-radius:2px;color:#333;">%s</span>`, escape(s)))
			}
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.15rem;"><span style="font-size:0.68rem;font-weight:600;color:#555;">%s:</span> %s</div>`, displayNames[cat], strings.Join(tags, " ")))
		}
		b.WriteString(`</div>`)
	}
	return b.String()
}



// ── New template render functions ─────────────────────────
func renderExpBlockExecutive(items []exp, escape func(string) string) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1rem;"><div style="font-size:0.65rem;font-weight:700;color:#c9a93e;text-transform:uppercase;letter-spacing:1px;border-bottom:1px solid #c9a93e;padding-bottom:0.15rem;margin-bottom:0.25rem;font-family:'Georgia',serif;">Professional Experience</div>`))
	for _, e := range items {
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.25rem;padding-bottom:0.25rem;border-bottom:1px solid #f0ede6;"><div style="font-size:0.72rem;font-weight:700;color:#0a1628;font-family:'Georgia',serif;">%s</div><div style="font-size:0.65rem;color:#555;font-style:italic;">%s</div><div style="font-size:0.6rem;color:#c9a93e;font-weight:600;">%s — %s</div><div style="font-size:0.62rem;color:#333;margin-top:0.1rem;line-height:1.35;font-family:'Georgia',serif;">%s</div></div>`,
			escape(e.Role), escape(e.Company), escape(e.From), escape(e.To), escape(e.Description)))
	}
	b.WriteString(`</div>`)
	return b.String()
}

func renderEduBlockExecutive(items []edu, escape func(string) string) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1rem;"><div style="font-size:0.65rem;font-weight:700;color:#c9a93e;text-transform:uppercase;letter-spacing:1px;border-bottom:1px solid #c9a93e;padding-bottom:0.15rem;margin-bottom:0.25rem;font-family:'Georgia',serif;">Education</div>`))
	for _, e := range items {
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.15rem;"><div style="font-size:0.7rem;font-weight:600;color:#0a1628;font-family:'Georgia',serif;">%s</div><div style="font-size:0.62rem;color:#555;">%s</div><div style="font-size:0.6rem;color:#888;">%s — %s</div></div>`,
			escape(e.Degree), escape(e.School), escape(e.From), escape(e.To)))
	}
	b.WriteString(`</div>`)
	return b.String()
}

func renderExpBlockCreative(items []exp, escape func(string) string) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1rem;"><div style="font-size:0.75rem;font-weight:700;color:#ff6b6b;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.4rem;">💼 Experience</div>`))
	for _, e := range items {
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.25rem;background:#fff5f5;border-radius:8px;padding:0.3rem 0.5rem;border-left:3px solid #ff6b6b;"><div style="font-size:0.72rem;font-weight:700;color:#222;">%s</div><div style="font-size:0.62rem;color:#e17055;font-weight:600;">@ %s</div><div style="font-size:0.6rem;color:#888;">%s — %s</div><div style="font-size:0.62rem;color:#333;margin-top:0.05rem;line-height:1.3;">%s</div></div>`,
			escape(e.Role), escape(e.Company), escape(e.From), escape(e.To), escape(e.Description)))
	}
	b.WriteString(`</div>`)
	return b.String()
}

func renderEduBlockCreative(items []edu, escape func(string) string) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1rem;"><div style="font-size:0.75rem;font-weight:700;color:#6bcb77;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.4rem;">🎓 Education</div>`))
	for _, e := range items {
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.15rem;background:#f0fff4;border-radius:8px;padding:0.25rem 0.5rem;border-left:3px solid #6bcb77;"><div style="font-size:0.7rem;font-weight:600;color:#222;">%s</div><div style="font-size:0.6rem;color:#555;">%s</div><div style="font-size:0.58rem;color:#888;">%s — %s</div></div>`,
			escape(e.Degree), escape(e.School), escape(e.From), escape(e.To)))
	}
	b.WriteString(`</div>`)
	return b.String()
}

func renderExpBlockTimeline(items []exp, escape func(string) string) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1rem;"><div style="font-size:0.75rem;font-weight:700;color:#6c5ce7;text-transform:uppercase;letter-spacing:1.5px;border-bottom:2px solid #6c5ce7;padding-bottom:0.2rem;margin-bottom:0.5rem;">Experience</div>
		<div style="position:relative;padding-left:1rem;">
		<div style="position:absolute;left:0;top:0;bottom:0;width:2px;background:#6c5ce7;"></div>`))
	for i, e := range items {
		extra := ""
		if i < len(items)-1 {
			extra = `margin-bottom:0.8rem;`
		}
		b.WriteString(fmt.Sprintf(`<div style="position:relative;padding-left:1.2rem;%s"><div style="position:absolute;left:-0.45rem;top:0.2rem;width:12px;height:12px;border-radius:50%%;background:#6c5ce7;border:2px solid #fff;"></div><div style="font-size:0.68rem;color:#6c5ce7;font-weight:600;">%s — %s</div><div style="font-size:0.8rem;font-weight:600;color:#2d3436;">%s</div><div style="font-size:0.72rem;color:#636e72;">%s</div><div style="font-size:0.7rem;color:#333;margin-top:0.1rem;line-height:1.4;">%s</div></div>`,
			extra, escape(e.From), escape(e.To), escape(e.Role), escape(e.Company), escape(e.Description)))
	}
	b.WriteString(`</div></div>`)
	return b.String()
}

func renderEduBlockTimeline(items []edu, escape func(string) string) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1rem;"><div style="font-size:0.75rem;font-weight:700;color:#6c5ce7;text-transform:uppercase;letter-spacing:1.5px;border-bottom:2px solid #6c5ce7;padding-bottom:0.2rem;margin-bottom:0.5rem;">Education</div>
		<div style="position:relative;padding-left:1rem;">
		<div style="position:absolute;left:0;top:0;bottom:0;width:2px;background:#6c5ce7;"></div>`))
	for i, e := range items {
		extra := ""
		if i < len(items)-1 {
			extra = `margin-bottom:0.8rem;`
		}
		b.WriteString(fmt.Sprintf(`<div style="position:relative;padding-left:1.2rem;%s"><div style="position:absolute;left:-0.45rem;top:0.2rem;width:12px;height:12px;border-radius:50%%;background:#6c5ce7;border:2px solid #fff;"></div><div style="font-size:0.78rem;font-weight:600;color:#2d3436;">%s</div><div style="font-size:0.68rem;color:#636e72;">%s</div><div style="font-size:0.65rem;color:#6c5ce7;">%s — %s</div></div>`,
			extra, escape(e.Degree), escape(e.School), escape(e.From), escape(e.To)))
	}
	b.WriteString(`</div></div>`)
	return b.String()
}

// ── Helpers ────────────────────────────────────────────────
func cond(ok bool, a, b string) string {
	if ok {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func toSliceInterface[T any](s []T) []interface{} {
	res := make([]interface{}, len(s))
	for i, v := range s {
		res[i] = v
	}
	return res
}

// ── Main handler ───────────────────────────────────────────
func handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, "Method not allowed", 405)
		return
	}

	var req GenerateRequest
	body, _ := io.ReadAll(r.Body)
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, "Invalid request body", 400)
		return
	}

	if req.Name == "" {
		writeError(w, "Name is required", 400)
		return
	}

	// Step 1: Create Razorpay order
	log.Printf("Generating resume for: %s (%s)", req.Name, req.Template)
	orderID, err := createRazorpayOrder()
	if err != nil {
		log.Printf("Order creation failed: %v", err)
		writeError(w, "Payment setup failed: "+err.Error(), 502)
		return
	}
	log.Printf("Order created: %s", orderID)

	// Return order info to frontend so it can open checkout
	writeJSON(w, GenerateResponse{
		Success: true,
		OrderID: orderID,
		Key:     razorpayKey,
		Amount:  1000,
		Message: "order_created",
	})
}

func handleConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, "Method not allowed", 405)
		return
	}

	var req struct {
		GenerateRequest
		OrderID   string `json:"order_id"`
		PaymentID string `json:"payment_id"`
		Signature string `json:"signature"`
	}
	body, _ := io.ReadAll(r.Body)
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, "Invalid request body", 400)
		return
	}

	if req.Name == "" || req.OrderID == "" || req.PaymentID == "" || req.Signature == "" {
		writeError(w, "Missing required fields", 400)
		return
	}

	// Verify payment signature
	if !verifyPayment(req.OrderID, req.PaymentID, req.Signature) {
		log.Printf("Payment VERIFICATION FAILED for order %s", req.OrderID)
		writeError(w, "Payment verification failed", 402)
		return
	}

	log.Printf("Payment verified: %s (payment: %s)", req.OrderID, req.PaymentID)

	// Generate with Gemini (server-side key)
	genData := GenerateRequest{
		Name: req.Name, Role: req.Role, Email: req.Email, Phone: req.Phone,
		Location: req.Location, Portfolio: req.Portfolio, Summary: req.Summary,
		Skills: req.Skills, SkillsCategories: req.SkillsCategories,
		Experience: req.Experience, Education: req.Education,
		Template: req.Template,
	}

	var aiContent map[string]interface{}
	var aiErr error
	if geminiKey != "" {
		aiContent, aiErr = generateWithGemini(genData)
		if aiErr != nil {
			log.Printf("Gemini generation failed: %v (proceeding without AI)", aiErr)
		}
	} else {
		log.Printf("No Gemini key configured — using raw form data")
	}

	// Render HTML server-side
	html := renderResumeHTML(genData, aiContent)

	writeJSON(w, GenerateResponse{
		Success:   true,
		HTML:      html,
		OrderID:   req.OrderID,
		PaymentID: req.PaymentID,
		Message:   fmt.Sprintf("resume_generated%s", cond(aiErr != nil, "_no_ai", "")),
	})
}

func main() {
	port := "3000"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}
	if k := os.Getenv("RAZORPAY_KEY"); k != "" {
		razorpayKey = k
	}
	if s := os.Getenv("RAZORPAY_SECRET"); s != "" {
		razorpaySecret = s
	}
	geminiKey = os.Getenv("GEMINI_API_KEY")

	if geminiKey == "" {
		log.Println("⚠️  GEMINI_API_KEY not set — AI generation will be skipped, using raw form data")
	}

	if o := os.Getenv("CORS_ORIGIN"); o != "" {
		corsOrigin = o
		log.Printf("✦ CORS origin restricted to: %s", corsOrigin)
	} else {
		log.Println("✦ CORS wide open (*) — set CORS_ORIGIN env var for production")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", cors(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"status":  "ok",
			"gemini":  geminiKey != "",
			"service": "resume-forge",
		})
	}))
	mux.HandleFunc("/api/generate", cors(handleGenerate))
	mux.HandleFunc("/api/confirm", cors(handleConfirm))

 	log.Printf("✦ Resume Forge backend on port %s", port)
 	log.Printf("✦ Gemini configured: %v", geminiKey != "")
 	if err := http.ListenAndServe(":"+port, mux); err != nil {
 		log.Fatal(err)
 	}
}
