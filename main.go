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

type proj struct {
	Title string
	Desc  string
	Tech  string
}

type achievement struct {
	Title       string `json:"title"`
	Date        string `json:"date"`
	Description string `json:"desc"`
}

type certification struct {
	Title  string `json:"title"`
	Issuer string `json:"issuer"`
	Date   string `json:"date"`
	Link   string `json:"link"`
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
	SkillsLanguages  string            `json:"skills_languages"`
	SkillsFrameworks string            `json:"skills_frameworks"`
	SkillsTools      string            `json:"skills_tools"`
	SkillsDatabases  string            `json:"skills_databases"`
	SkillsCloud      string            `json:"skills_cloud"`
	SkillsCategories SkillsCategories  `json:"skills_categories"`
	Experience       []ExpEntry        `json:"experience"`
	Education        []EduEntry        `json:"education"`
	Projects         []ProjEntry       `json:"projects"`
	Achievements     []achievement     `json:"achievements"`
	Certifications   []certification   `json:"certifications"`
	Template         string            `json:"template"`
}

type SkillsCategories struct {
	Languages  string `json:"skills_languages"`
	Frameworks string `json:"skills_frameworks"`
	Tools      string `json:"skills_tools"`
	Databases  string `json:"skills_databases"`
	Cloud      string `json:"skills_cloud"`
}

// normalizeSkills copies flat skill fields into SkillsCategories if the nested object is empty
func normalizeSkills(req *GenerateRequest) {
	if req.SkillsCategories.Languages == "" && req.SkillsCategories.Frameworks == "" && 
	   req.SkillsCategories.Tools == "" && req.SkillsCategories.Databases == "" && 
	   req.SkillsCategories.Cloud == "" {
		req.SkillsCategories.Languages = req.SkillsLanguages
		req.SkillsCategories.Frameworks = req.SkillsFrameworks
		req.SkillsCategories.Tools = req.SkillsTools
		req.SkillsCategories.Databases = req.SkillsDatabases
		req.SkillsCategories.Cloud = req.SkillsCloud
	}
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

type ProjEntry struct {
	Title string `json:"title"`
	Desc  string `json:"desc"`
	Tech  string `json:"tech"`
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
Projects: %s
Achievements: %s
Certifications: %s
Skills: %s

Return ONLY valid JSON with this exact structure (no markdown, no code fences):
{
  "summary": "well written professional summary...",
  "experience": [
    {"company": "...", "role": "...", "from": "...", "to": "...", "description": "expanded description with achievements..."}
  ],
  "education": [
    {"school": "...", "degree": "...", "from": "...", "to": "..."}
  ],
  "projects": [
    {"title": "...", "desc": "enhanced description...", "tech": "..."}
  ],
  "achievements": [
    {"title": "...", "date": "...", "description": "..."}
  ],
  "certifications": [
    {"title": "...", "issuer": "...", "date": "...", "link": "..."}
  ]
}

CRITICAL INSTRUCTIONS:
1. For every single experience entry in the list, you MUST generate or enhance the "description" field. Even if the input description is empty, short, or vague, you must write a comprehensive, highly professional description (3-4 bullet points or a detailed paragraph) highlighting key achievements, responsibilities, and impact typical for that role (e.g. using action verbs and metrics). Do NOT leave the description empty.
2. In the education array, do NOT swap the school name and degree under any circumstances. The "school" field must contain the name of the school or university (e.g., "Stanford University"), and the "degree" field must contain the degree name (e.g., "BS in Computer Science").
3. In the projects array, do NOT omit any projects. The "desc" field should be a concise but professional description of the project, and the "tech" field should list the tech stack (e.g., "Go, React, Docker").`,
		data.Name, data.Role, data.Email, data.Phone, data.Location, data.Portfolio,
		data.Summary, toJSON(data.Experience), toJSON(data.Education), toJSON(data.Projects), toJSON(data.Achievements), toJSON(data.Certifications), data.Skills)

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
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Gemini API unreachable: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

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
			"projects":   toSliceInterface(data.Projects),
			"achievements": toSliceInterface(data.Achievements),
			"certifications": toSliceInterface(data.Certifications),
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

	var projects []proj
	if aiContent != nil {
		if aiProj, ok := aiContent["projects"].([]interface{}); ok {
			for _, p := range aiProj {
				if pm, ok := p.(map[string]interface{}); ok {
					projects = append(projects, proj{
						Title: toString(pm["title"]),
						Desc:  toString(pm["desc"]),
						Tech:  toString(pm["tech"]),
					})
				}
			}
		}
	}
	if len(projects) == 0 {
		for _, p := range data.Projects {
			projects = append(projects, proj{
				Title: p.Title,
				Desc:  p.Desc,
				Tech:  p.Tech,
			})
		}
	}

	var achievements []achievement
	if aiContent != nil {
		if aiAch, ok := aiContent["achievements"].([]interface{}); ok {
			for _, a := range aiAch {
				if am, ok := a.(map[string]interface{}); ok {
					achievements = append(achievements, achievement{
						Title:       toString(am["title"]),
						Date:        toString(am["date"]),
						Description: toString(am["description"]),
					})
				}
			}
		}
	}
	if len(achievements) == 0 {
		for _, a := range data.Achievements {
			achievements = append(achievements, a)
		}
	}

	var certifications []certification
	if aiContent != nil {
		if aiCert, ok := aiContent["certifications"].([]interface{}); ok {
			for _, c := range aiCert {
				if cm, ok := c.(map[string]interface{}); ok {
					certifications = append(certifications, certification{
						Title:  toString(cm["title"]),
						Issuer: toString(cm["issuer"]),
						Date:   toString(cm["date"]),
						Link:   toString(cm["link"]),
					})
				}
			}
		}
	}
	if len(certifications) == 0 {
		for _, c := range data.Certifications {
			certifications = append(certifications, c)
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

	fontCSS := `<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=DM+Sans:ital,opsz,wght@0,9..40,300;0,9..40,400;0,9..40,500;0,9..40,600;0,9..40,700;1,9..40,400&family=Fira+Code:wght@400;500;600&family=Inter:wght@300;400;500;600;700;800&family=Lora:ital,wght@0,400;0,500;0,600;0,700;1,400&family=Playfair+Display:ital,wght@0,400;0,600;0,700;1,400&family=Plus+Jakarta+Sans:wght@300;400;500;600;700;800&family=Poppins:wght@300;400;500;600;700;800&family=JetBrains+Mono:wght@400;500;600&display=swap" rel="stylesheet">
<style>
  .resume-output, .resume-output * { box-sizing: border-box !important; }
</style>`

	switch data.Template {
	case "terminal":
		return fontCSS + fmt.Sprintf(`<div class="resume-output resume-terminal" style="width:210mm;height:297mm;background:#0f172a;color:#e2e8f0;font-family:'JetBrains Mono','Fira Code',monospace;padding:15mm;box-shadow:0 0 30px rgba(0,0,0,0.3);border-radius:2px;overflow:hidden;box-sizing:border-box;display:flex;flex-direction:column;justify-content:space-between;">
				<div style="flex:1;">
					<!-- Terminal Header Bar -->
					<div style="display:flex;align-items:center;gap:0.4rem;margin-bottom:1rem;border-bottom:1px solid #334155;padding-bottom:0.5rem;">
						<span style="width:10px;height:10px;border-radius:50%%;background:#ef4444;display:inline-block;"></span>
						<span style="width:10px;height:10px;border-radius:50%%;background:#f59e0b;display:inline-block;"></span>
						<span style="width:10px;height:10px;border-radius:50%%;background:#10b981;display:inline-block;"></span>
						<span style="margin-left:0.5rem;font-size:0.65rem;color:#64748b;letter-spacing:1px;">bash - resume.sh</span>
					</div>
					
					<div style="margin-bottom:1rem;">
						<h1 style="font-size:1.6rem;font-weight:700;color:#10b981;margin:0;letter-spacing:-0.5px;">%s</h1>
						<div style="color:#38bdf8;font-size:0.8rem;margin-top:0.2rem;font-weight:500;">$ type role --name="%s"</div>
						<div style="display:flex;flex-wrap:wrap;gap:1rem;margin-top:0.5rem;font-size:0.68rem;color:#94a3b8;">%s%s%s%s</div>
					</div>

					<div style="margin-top:1.2rem;">
						<div style="font-size:0.75rem;font-weight:bold;color:#10b981;margin-bottom:0.4rem;text-transform:lowercase;">// summary</div>
						<div style="font-size:0.68rem;color:#cbd5e1;line-height:1.4;background:#1e293b;padding:0.6rem 0.8rem;border-radius:4px;border-left:3px solid #38bdf8;">%s</div>
					</div>

					<div style="margin-top:1.2rem;">
						%s
					</div>
					<div style="margin-top:1.2rem;">
						%s
					</div>
					<div style="margin-top:1.2rem;">
						%s
					</div>
					<div style="margin-top:1.2rem;">
						%s
					</div>
					<div style="margin-top:1.2rem;">
						%s
					</div>
					<div style="margin-top:1.2rem;">
						%s
					</div>
				</div>
			</div>`,
			escape(data.Name), escape(data.Role),
			cond(data.Email != "", `<span>✉ `+escape(data.Email)+`</span>`, ""),
			cond(data.Phone != "", `<span>📞 `+escape(data.Phone)+`</span>`, ""),
			cond(data.Location != "", `<span>📍 `+escape(data.Location)+`</span>`, ""),
			cond(data.Portfolio != "", `<span>🔗 `+escape(data.Portfolio)+`</span>`, ""),
			escape(summary),
			renderExpBlock("experience", experiences, escape, "rt"),
			renderAchievementsBlock(achievements, escape, "rt"),
			renderEduBlock("education", educations, escape, "rt"),
			renderProjBlock(projects, escape, "rt"),
			renderCertificationsBlock(certifications, escape, "rt"),
			renderSkillsBlockCategorized(skills, categorizedSkills, hasCategorized, escape, "rt"),
		)

	case "minimal":
		return fontCSS + fmt.Sprintf(`<div class="resume-output resume-minimal" style="width:210mm;height:297mm;background:#fff;color:#1e293b;font-family:'Inter',sans-serif;padding:18mm;box-shadow:0 0 30px rgba(0,0,0,0.3);border-radius:2px;overflow:hidden;box-sizing:border-box;display:flex;flex-direction:column;justify-content:space-between;">
				<div style="flex:1;">
					<div style="text-align:center;margin-bottom:1.5rem;">
						<h1 style="font-size:2rem;font-weight:500;color:#0f172a;letter-spacing:1px;font-family:'Lora',serif;margin-bottom:0.25rem;">%s</h1>
						<div style="color:#64748b;font-size:0.85rem;text-transform:uppercase;letter-spacing:2px;font-weight:500;margin-bottom:0.6rem;">%s</div>
						<div style="font-size:0.7rem;color:#64748b;display:flex;justify-content:center;flex-wrap:wrap;gap:1rem;">%s%s%s%s</div>
					</div>
					<div style="border-bottom:1px solid #e2e8f0;margin-bottom:1.2rem;"></div>
					
					<div style="margin-bottom:1.2rem;">
						<div style="font-size:0.78rem;font-weight:700;text-transform:uppercase;letter-spacing:1.5px;color:#0f172a;margin-bottom:0.4rem;font-family:'Lora',serif;">Summary</div>
						<p style="font-size:0.7rem;line-height:1.5;color:#334155;">%s</p>
					</div>
					
					%s
					%s
					%s
					%s
					%s
					%s
				</div>
			</div>`,
			escape(data.Name), escape(data.Role),
			cond(data.Email != "", `<span>`+escape(data.Email)+`</span>`, ""),
			cond(data.Phone != "", `<span>`+escape(data.Phone)+`</span>`, ""),
			cond(data.Location != "", `<span>`+escape(data.Location)+`</span>`, ""),
			cond(data.Portfolio != "", `<span>`+escape(data.Portfolio)+`</span>`, ""),
			escape(summary),
			renderExpBlock("Experience", experiences, escape, "rm"),
			renderAchievementsBlock(achievements, escape, "rm"),
			renderEduBlock("Education", educations, escape, "rm"),
			renderProjBlock(projects, escape, "rm"),
			renderCertificationsBlock(certifications, escape, "rm"),
			renderSkillsBlockCategorized(skills, categorizedSkills, hasCategorized, escape, "rm"),
		)

	case "modern":
		return fontCSS + fmt.Sprintf(`<div class="resume-output resume-modern" style="width:210mm;height:297mm;display:flex;background:#fff;box-shadow:0 0 30px rgba(0,0,0,0.3);border-radius:2px;overflow:hidden;box-sizing:border-box;font-family:'Plus Jakarta Sans',sans-serif;">
				<div style="width:35%%;background:#0f172a;color:#f8fafc;padding:15mm 8mm 15mm 12mm;display:flex;flex-direction:column;justify-content:space-between;box-sizing:border-box;border-right:1px solid #1e293b;">
					<div>
						<h1 style="font-size:1.4rem;font-weight:800;color:#fff;line-height:1.2;margin:0 0 0.2rem 0;letter-spacing:-0.5px;">%s</h1>
						<div style="font-size:0.72rem;color:#38bdf8;font-weight:600;text-transform:uppercase;letter-spacing:1px;margin-bottom:1.2rem;">%s</div>
						<div style="font-size:0.68rem;margin-top:1.5rem;line-height:1.5;color:#94a3b8;display:flex;flex-direction:column;gap:0.4rem;">%s%s%s%s</div>
						%s
					</div>
					%s
				</div>
				<div style="width:65%%;padding:15mm 12mm 15mm 8mm;display:flex;flex-direction:column;box-sizing:border-box;justify-content:flex-start;color:#1e293b;">
					<div style="margin-bottom:1.2rem;">
						<div style="font-size:0.78rem;font-weight:700;text-transform:uppercase;letter-spacing:1px;color:#0f172a;border-bottom:2px solid #38bdf8;padding-bottom:0.25rem;margin-bottom:0.5rem;width:fit-content;">About Me</div>
						<p style="font-size:0.68rem;color:#334155;line-height:1.5;">%s</p>
					</div>
					%s
					%s
					%s
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
			renderAchievementsBlock(achievements, escape, "mod"),
			renderProjBlock(projects, escape, "mod"),
			renderCertificationsBlock(certifications, escape, "mod"),
		)

	case "executive":
		return fontCSS + fmt.Sprintf(`<div class="resume-output resume-executive" style="width:210mm;height:297mm;background:#fff;color:#1e293b;font-family:'Lora',serif;padding:18mm;box-shadow:0 0 30px rgba(0,0,0,0.3);border-radius:2px;overflow:hidden;box-sizing:border-box;display:flex;flex-direction:column;justify-content:space-between;">
				<div style="flex:1;">
					<div style="text-align:center;margin-bottom:1.5rem;">
						<h1 style="font-size:2.2rem;font-weight:700;color:#0f1e36;margin:0;letter-spacing:0.5px;font-family:'Playfair Display',serif;text-transform:uppercase;margin-bottom:0.2rem;">%s</h1>
						<div style="font-size:0.95rem;color:#b45309;margin-top:0.2rem;font-weight:600;font-style:italic;letter-spacing:1px;">%s</div>
						<div style="font-size:0.72rem;color:#475569;margin-top:0.6rem;display:flex;justify-content:center;flex-wrap:wrap;gap:1.2rem;font-family:'Lora',serif;font-style:italic;">%s%s%s%s</div>
					</div>
					<div style="border-top:1px solid #b45309;border-bottom:1px solid #b45309;padding:0.15rem 0;margin-bottom:1.5rem;">
						<div style="border-top:0.5px solid #b45309;border-bottom:0.5px solid #b45309;height:1px;"></div>
					</div>
					
					<div style="margin-bottom:1.2rem;">
						<div style="font-size:0.8rem;font-weight:700;color:#b45309;text-transform:uppercase;letter-spacing:1.5px;border-bottom:1px solid #e2e8f0;padding-bottom:0.2rem;margin-bottom:0.4rem;font-family:'Playfair Display',serif;">Professional Summary</div>
						<p style="font-size:0.72rem;color:#334155;line-height:1.6;font-family:'Lora',serif;">%s</p>
					</div>
					%s
					%s
					%s
					%s
					%s
					%s
				</div>
			</div>`,
			escape(data.Name), escape(data.Role),
			cond(data.Email != "", `<span style="color:#b45309;margin-right:0.2rem;">✉</span>`+escape(data.Email), ""),
			cond(data.Phone != "", `<span style="color:#b45309;margin-right:0.2rem;">📞</span>`+escape(data.Phone), ""),
			cond(data.Location != "", `<span style="color:#b45309;margin-right:0.2rem;">📍</span>`+escape(data.Location), ""),
			cond(data.Portfolio != "", `<span style="color:#b45309;margin-right:0.2rem;">🔗</span>`+escape(data.Portfolio), ""),
			escape(summary),
			renderExpBlockExecutive(experiences, escape),
			renderAchievementsBlock(achievements, escape, "exec"),
			renderEduBlockExecutive(educations, escape),
			renderProjBlock(projects, escape, "exec"),
			renderCertificationsBlock(certifications, escape, "exec"),
			renderSkillsBlockCategorized(skills, categorizedSkills, hasCategorized, escape, "exec"),
		)

	case "creative":
		return fontCSS + fmt.Sprintf(`<div class="resume-output resume-creative" style="width:210mm;height:297mm;background:#fff;color:#1e293b;font-family:'Poppins',sans-serif;padding:15mm;box-shadow:0 0 30px rgba(0,0,0,0.3);border-radius:2px;overflow:hidden;box-sizing:border-box;display:flex;flex-direction:column;justify-content:space-between;">
				<div style="flex:1;">
					<div style="background:linear-gradient(135deg,#6366f1 0%%,#a855f7 100%%);padding:1.4rem 1.6rem;text-align:center;border-radius:12px;margin-bottom:1.2rem;color:#fff;box-shadow:0 4px 20px rgba(99,102,241,0.15);">
						<h1 style="font-size:1.8rem;font-weight:800;color:#fff;margin:0;letter-spacing:-0.5px;">%s</h1>
						<div style="font-size:0.85rem;color:#f3e8ff;margin-top:0.2rem;font-weight:600;text-transform:uppercase;letter-spacing:1px;">%s</div>
						<div style="font-size:0.68rem;margin-top:0.5rem;display:flex;justify-content:center;flex-wrap:wrap;gap:0.8rem;color:#f3e8ff;">%s%s%s%s</div>
					</div>
					
					<div style="margin-bottom:1.2rem;background:#f5f3ff;border-radius:12px;padding:0.8rem 1rem;border-left:4px solid #8b5cf6;">
						<div style="font-size:0.75rem;font-weight:700;color:#6d28d9;text-transform:uppercase;letter-spacing:0.5px;margin-bottom:0.25rem;">✨ About Me</div>
						<p style="font-size:0.68rem;color:#4c1d95;line-height:1.45;">%s</p>
					</div>
					
					%s
					%s
					%s
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
			renderAchievementsBlock(achievements, escape, "cr"),
			renderEduBlockCreative(educations, escape),
			renderProjBlock(projects, escape, "cr"),
			renderCertificationsBlock(certifications, escape, "cr"),
			renderSkillsBlockCategorized(skills, categorizedSkills, hasCategorized, escape, "cr"),
		)

	case "timeline":
		return fontCSS + fmt.Sprintf(`<div class="resume-output resume-timeline" style="width:210mm;height:297mm;background:#fff;color:#1e293b;font-family:'Plus Jakarta Sans',sans-serif;padding:15mm;box-shadow:0 0 30px rgba(0,0,0,0.3);border-radius:2px;overflow:hidden;box-sizing:border-box;display:flex;flex-direction:column;justify-content:space-between;">
				<div style="flex:1;">
					<div style="background:#0f172a;border-radius:10px;padding:1.2rem 1.5rem;margin-bottom:1.2rem;color:#fff;display:flex;justify-content:space-between;align-items:center;">
						<div>
							<h1 style="font-size:1.6rem;font-weight:800;color:#fff;margin:0;letter-spacing:-0.5px;">%s</h1>
							<div style="font-size:0.8rem;color:#38bdf8;margin-top:0.15rem;font-weight:600;">%s</div>
						</div>
						<div style="font-size:0.68rem;color:#94a3b8;line-height:1.45;text-align:right;">%s%s%s%s</div>
					</div>
					
					<div style="margin-bottom:1.2rem;">
						<div style="font-size:0.78rem;font-weight:700;color:#4f46e5;text-transform:uppercase;letter-spacing:1px;border-bottom:2px solid #e2e8f0;padding-bottom:0.2rem;margin-bottom:0.4rem;width:fit-content;">About</div>
						<p style="font-size:0.68rem;color:#334155;line-height:1.5;">%s</p>
					</div>
					
					%s
					%s
					%s
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
			renderAchievementsBlock(achievements, escape, "tl"),
			renderEduBlockTimeline(educations, escape),
			renderProjBlock(projects, escape, "tl"),
			renderCertificationsBlock(certifications, escape, "tl"),
			renderSkillsBlockCategorized(skills, categorizedSkills, hasCategorized, escape, "tl"),
		)

	case "columns":
		return fontCSS + fmt.Sprintf(`<div class="resume-output resume-columns" style="width:210mm;height:297mm;display:flex;background:#fff;box-shadow:0 0 30px rgba(0,0,0,0.3);border-radius:2px;overflow:hidden;box-sizing:border-box;font-family:'Inter',sans-serif;">
				<div style="width:50%%;background:#f8fafc;padding:15mm;display:flex;flex-direction:column;justify-content:flex-start;box-sizing:border-box;border-right:1px solid #e2e8f0;">
					<div style="margin-bottom:1.5rem;">
						<h1 style="font-size:1.6rem;font-weight:800;color:#0f172a;line-height:1.2;margin:0 0 0.2rem 0;letter-spacing:-0.5px;">%s</h1>
						<div style="font-size:0.85rem;color:#4f46e5;font-weight:600;text-transform:uppercase;letter-spacing:1px;margin-bottom:1rem;">%s</div>
						<div style="font-size:0.68rem;color:#475569;line-height:1.6;display:flex;flex-direction:column;gap:0.25rem;">%s%s%s%s</div>
					</div>
					<div style="margin-bottom:1.5rem;">
						<div style="font-size:0.78rem;font-weight:700;color:#0f172a;text-transform:uppercase;letter-spacing:1px;border-bottom:2px solid #4f46e5;padding-bottom:0.2rem;margin-bottom:0.4rem;width:fit-content;">Summary</div>
						<p style="font-size:0.68rem;color:#334155;line-height:1.5;">%s</p>
					</div>
					%s
				</div>
				<div style="width:50%%;padding:15mm;display:flex;flex-direction:column;justify-content:flex-start;box-sizing:border-box;color:#1e293b;">
					%s
					%s
					%s
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
			renderAchievementsBlock(achievements, escape, "cp"),
			renderEduBlock("Education", educations, escape, "cp"),
			renderProjBlock(projects, escape, "cp"),
			renderCertificationsBlock(certifications, escape, "cp"),
		)

	default: // compact
		return fontCSS + fmt.Sprintf(`<div class="resume-output resume-compact" style="width:210mm;height:297mm;background:#fff;color:#1e293b;font-family:'Inter',sans-serif;padding:15mm;box-shadow:0 0 30px rgba(0,0,0,0.3);border-radius:2px;overflow:hidden;box-sizing:border-box;display:flex;flex-direction:column;justify-content:space-between;">
				<div style="flex:1;">
					<div style="border-bottom:2px solid #0d9488;padding-bottom:0.6rem;margin-bottom:1rem;display:flex;justify-content:space-between;align-items:flex-end;">
						<div>
							<h1 style="font-size:1.5rem;font-weight:800;color:#0f172a;margin:0;letter-spacing:-0.5px;">%s</h1>
							<div style="font-size:0.8rem;color:#0d9488;margin-top:0.15rem;font-weight:600;text-transform:uppercase;letter-spacing:0.5px;">%s</div>
						</div>
						<div style="font-size:0.68rem;color:#475569;line-height:1.45;text-align:right;">%s%s%s%s</div>
					</div>
					
					<div style="margin-bottom:1rem;">
						<div style="font-size:0.75rem;font-weight:700;color:#0d9488;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.3rem;">Summary</div>
						<p style="font-size:0.68rem;line-height:1.5;color:#334155;margin:0;">%s</p>
					</div>
					
					%s
					%s
					%s
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
			renderExpBlock("Experience", experiences, escape, "cp"),
			renderAchievementsBlock(achievements, escape, "cp"),
			renderEduBlock("Education", educations, escape, "cp"),
			renderProjBlock(projects, escape, "cp"),
			renderCertificationsBlock(certifications, escape, "cp"),
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
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.75rem;font-weight:bold;color:#10b981;text-transform:lowercase;border-bottom:1px solid #334155;padding-bottom:0.2rem;margin-bottom:0.5rem;">// %s</div>`, title))
		for _, e := range items {
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.8rem;"><div style="font-size:0.75rem;font-weight:600;color:#38bdf8;">%s <span style="color:#94a3b8;font-weight:400;">at</span> %s</div><div style="font-size:0.65rem;color:#64748b;margin-bottom:0.2rem;">[%s — %s]</div><div style="font-size:0.68rem;color:#cbd5e1;line-height:1.45;white-space:pre-line;">%s</div></div>`,
				escape(e.Role), escape(e.Company), escape(e.From), escape(e.To), escape(e.Description)))
		}
		b.WriteString(`</div>`)
	} else if style == "rm" {
		b.WriteString(fmt.Sprintf(`<div style="margin-top:1.2rem;"><div style="font-size:0.78rem;font-weight:700;text-transform:uppercase;letter-spacing:1.5px;color:#0f172a;margin-bottom:0.6rem;font-family:'Lora',serif;border-bottom:1px solid #f1f5f9;padding-bottom:0.25rem;">%s</div>`, title))
		for _, e := range items {
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.8rem;"><div style="display:flex;justify-content:space-between;align-items:baseline;margin-bottom:0.15rem;"><span style="font-size:0.75rem;font-weight:600;color:#0f172a;">%s <span style="font-weight:400;color:#64748b;">— %s</span></span><span style="font-size:0.65rem;color:#64748b;">%s — %s</span></div><div style="font-size:0.68rem;color:#475569;line-height:1.45;white-space:pre-line;">%s</div></div>`,
				escape(e.Role), escape(e.Company), escape(e.From), escape(e.To), escape(e.Description)))
		}
		b.WriteString(`</div>`)
	} else if style == "mod" {
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.78rem;font-weight:700;text-transform:uppercase;letter-spacing:1px;color:#0f172a;border-bottom:2px solid #38bdf8;padding-bottom:0.25rem;margin-bottom:0.5rem;width:fit-content;">%s</div>`, title))
		for _, e := range items {
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.8rem;"><div style="display:flex;justify-content:space-between;align-items:baseline;margin-bottom:0.15rem;"><span style="font-size:0.78rem;font-weight:700;color:#0f172a;">%s <span style="font-weight:500;color:#38bdf8;">@ %s</span></span><span style="font-size:0.62rem;color:#64748b;font-weight:600;">%s — %s</span></div><div style="font-size:0.68rem;color:#475569;line-height:1.45;white-space:pre-line;">%s</div></div>`,
				escape(e.Role), escape(e.Company), escape(e.From), escape(e.To), escape(e.Description)))
		}
		b.WriteString(`</div>`)
	} else {
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.75rem;font-weight:700;color:#0d9488;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.4rem;border-bottom:1px solid #f1f5f9;padding-bottom:0.2rem;">%s</div>`, title))
		for _, e := range items {
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.6rem;"><div style="display:flex;justify-content:space-between;align-items:baseline;margin-bottom:0.15rem;"><span style="font-size:0.75rem;font-weight:700;color:#0f172a;">%s <span style="font-weight:400;color:#64748b;">— %s</span></span><span style="font-size:0.62rem;color:#6b7280;font-weight:500;">%s — %s</span></div><div style="font-size:0.68rem;color:#475569;line-height:1.45;white-space:pre-line;">%s</div></div>`,
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
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.75rem;font-weight:bold;color:#10b981;text-transform:lowercase;border-bottom:1px solid #334155;padding-bottom:0.2rem;margin-bottom:0.5rem;">// %s</div>`, title))
		for _, e := range items {
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.4rem;"><div style="font-size:0.75rem;font-weight:600;color:#38bdf8;">%s</div><div style="font-size:0.68rem;color:#cbd5e1;">%s <span style="color:#64748b;">[%s — %s]</span></div></div>`,
				escape(e.Degree), escape(e.School), escape(e.From), escape(e.To)))
		}
		b.WriteString(`</div>`)
	} else if style == "rm" {
		b.WriteString(fmt.Sprintf(`<div style="margin-top:1.2rem;"><div style="font-size:0.78rem;font-weight:700;text-transform:uppercase;letter-spacing:1.5px;color:#0f172a;margin-bottom:0.5rem;font-family:'Lora',serif;border-bottom:1px solid #f1f5f9;padding-bottom:0.25rem;">%s</div>`, title))
		for _, e := range items {
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.4rem;display:flex;justify-content:space-between;align-items:baseline;"><div style="font-size:0.72rem;font-weight:600;color:#0f172a;">%s <span style="font-weight:400;color:#64748b;">at %s</span></div><div style="font-size:0.65rem;color:#64748b;margin-left:auto;">%s — %s</div></div>`,
				escape(e.Degree), escape(e.School), escape(e.From), escape(e.To)))
		}
		b.WriteString(`</div>`)
	} else {
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.75rem;font-weight:700;color:#0d9488;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.4rem;border-bottom:1px solid #f1f5f9;padding-bottom:0.2rem;">%s</div>`, title))
		for _, e := range items {
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.3rem;display:flex;justify-content:space-between;align-items:baseline;"><div style="font-size:0.72rem;font-weight:700;color:#0f172a;">%s <span style="font-weight:400;color:#64748b;">at %s</span></div><span style="font-size:0.62rem;color:#6b7280;font-weight:500;margin-left:auto;">%s — %s</span></div>`,
				escape(e.Degree), escape(e.School), escape(e.From), escape(e.To)))
		}
		b.WriteString(`</div>`)
	}
	return b.String()
}

func renderProjBlock(items []proj, escape func(string) string, style string) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	if style == "rt" { // terminal style
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.75rem;font-weight:bold;color:#10b981;text-transform:lowercase;border-bottom:1px solid #334155;padding-bottom:0.2rem;margin-bottom:0.5rem;">// projects</div>`))
		for _, p := range items {
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.5rem;"><div style="font-size:0.75rem;font-weight:600;color:#38bdf8;">%s <span style="font-size:0.65rem;color:#64748b;">[%s]</span></div><div style="font-size:0.68rem;color:#cbd5e1;line-height:1.4;">%s</div></div>`,
				escape(p.Title), escape(p.Tech), escape(p.Desc)))
		}
		b.WriteString(`</div>`)
	} else if style == "rm" { // minimal style
		b.WriteString(fmt.Sprintf(`<div style="margin-top:1.2rem;"><div style="font-size:0.78rem;font-weight:700;text-transform:uppercase;letter-spacing:1.5px;color:#0f172a;margin-bottom:0.5rem;font-family:'Lora',serif;border-bottom:1px solid #f1f5f9;padding-bottom:0.25rem;">Projects</div>`))
		for _, p := range items {
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.5rem;"><div style="display:flex;justify-content:space-between;align-items:baseline;"><div style="font-size:0.72rem;font-weight:700;color:#0f172a;">%s</div><span style="font-size:0.62rem;color:#64748b;font-weight:500;font-style:italic;">%s</span></div><div style="font-size:0.68rem;color:#334155;line-height:1.45;margin-top:0.15rem;">%s</div></div>`,
				escape(p.Title), escape(p.Tech), escape(p.Desc)))
		}
		b.WriteString(`</div>`)
	} else if style == "exec" { // executive style
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.8rem;font-weight:700;color:#b45309;text-transform:uppercase;letter-spacing:1.5px;border-bottom:1px solid #e2e8f0;padding-bottom:0.2rem;margin-bottom:0.5rem;font-family:'Playfair Display',serif;">Projects</div>`))
		for _, p := range items {
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.5rem;"><div style="display:flex;justify-content:space-between;align-items:baseline;"><div style="font-size:0.75rem;font-weight:700;color:#0f1e36;font-family:'Playfair Display',serif;">%s</div><span style="font-size:0.65rem;color:#64748b;font-family:'Lora',serif;font-style:italic;font-weight:500;">%s</span></div><div style="font-size:0.7rem;color:#334155;line-height:1.5;font-family:'Lora',serif;margin-top:0.15rem;">%s</div></div>`,
				escape(p.Title), escape(p.Tech), escape(p.Desc)))
		}
		b.WriteString(`</div>`)
	} else if style == "cr" { // creative style
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.8rem;font-weight:700;color:#a855f7;text-transform:uppercase;letter-spacing:0.5px;margin-bottom:0.5rem;">🚀 Projects</div>`))
		for _, p := range items {
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.6rem;background:#faf5ff;border-radius:8px;padding:0.5rem 0.6rem;border-left:3px solid #a855f7;"><div style="display:flex;justify-content:space-between;align-items:baseline;margin-bottom:0.2rem;"><span style="font-size:0.75rem;font-weight:700;color:#581c87;">%s</span><span style="font-size:0.62rem;color:#701a75;font-weight:600;background:#f3e8ff;padding:0.1rem 0.35rem;border-radius:4px;">%s</span></div><div style="font-size:0.65rem;color:#4a044e;line-height:1.45;">%s</div></div>`,
				escape(p.Title), escape(p.Tech), escape(p.Desc)))
		}
		b.WriteString(`</div>`)
	} else if style == "tl" { // timeline style
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.78rem;font-weight:700;color:#4f46e5;text-transform:uppercase;letter-spacing:1px;border-bottom:2px solid #e2e8f0;padding-bottom:0.2rem;margin-bottom:0.5rem;width:fit-content;">Projects</div>
				<div style="position:relative;padding-left:0.8rem;margin-top:0.4rem;">
				<div style="position:absolute;left:0;top:0;bottom:0;width:2px;background:#8b5cf6;"></div>`))
		for i, p := range items {
			extra := ""
			if i < len(items)-1 {
				extra = `margin-bottom:0.6rem;`
			}
			b.WriteString(fmt.Sprintf(`<div style="position:relative;padding-left:1rem;%s"><div style="position:absolute;left:-1.15rem;top:0.25rem;width:8px;height:8px;border-radius:50%%;background:#8b5cf6;border:2px solid #fff;box-shadow:0 0 0 2px #8b5cf6;"></div><div style="display:flex;justify-content:space-between;align-items:baseline;"><div style="font-size:0.72rem;font-weight:700;color:#0f172a;">%s <span style="font-weight:500;color:#8b5cf6;font-size:0.65rem;margin-left:0.4rem;">[%s]</span></div></div><div style="font-size:0.65rem;color:#4b5563;line-height:1.45;margin-top:0.15rem;">%s</div></div>`,
				extra, escape(p.Title), escape(p.Tech), escape(p.Desc)))
		}
		b.WriteString(`</div></div>`)
	} else { // compact, modern, columns style
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.75rem;font-weight:700;color:#0d9488;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.4rem;border-bottom:1px solid #f1f5f9;padding-bottom:0.2rem;">Projects</div>`))
		for _, p := range items {
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.4rem;"><div style="display:flex;justify-content:space-between;align-items:baseline;"><div style="font-size:0.72rem;font-weight:700;color:#0f172a;">%s <span style="font-weight:500;color:#6b7280;font-size:0.65rem;margin-left:0.4rem;">(%s)</span></div></div><div style="font-size:0.65rem;color:#4b5563;line-height:1.45;margin-top:0.1rem;">%s</div></div>`,
				escape(p.Title), escape(p.Tech), escape(p.Desc)))
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
		tags = append(tags, fmt.Sprintf(`<span style="font-size:0.62rem;padding:0.1rem 0.45rem;background:#f1f5f9;color:#334155;border-radius:3px;margin-bottom:0.15rem;display:inline-block;margin-right:0.15rem;">%s</span>`, escape(s)))
	}
	joined := strings.Join(tags, "")

	if style == "rt" {
		return fmt.Sprintf(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.75rem;font-weight:bold;color:#10b981;text-transform:lowercase;border-bottom:1px solid #334155;padding-bottom:0.2rem;margin-bottom:0.5rem;">// skills</div><div style="display:flex;flex-wrap:wrap;gap:0.2rem;">%s</div></div>`, joined)
	}
	return fmt.Sprintf(`<div style="margin-top:1.2rem;"><div style="font-size:0.75rem;font-weight:700;text-transform:uppercase;letter-spacing:1px;color:#0d9488;margin-bottom:0.4rem;border-bottom:1px solid #f1f5f9;padding-bottom:0.2rem;">Skills</div><div style="display:flex;flex-wrap:wrap;gap:0.2rem;margin-top:0.2rem;">%s</div></div>`, joined)
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
	return fmt.Sprintf(`<div><div style="font-size:0.7rem;font-weight:600;text-transform:uppercase;letter-spacing:1px;color:#38bdf8;margin-top:1rem;margin-bottom:0.3rem;border-bottom:1px solid rgba(255,255,255,0.1);padding-bottom:0.2rem;">Skills</div>%s</div>`, strings.Join(items, ""))
}

func renderSidebarEdu(items []edu, escape func(string) string) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<div><div style="font-size:0.7rem;font-weight:700;text-transform:uppercase;letter-spacing:1px;color:#38bdf8;margin-top:1.5rem;margin-bottom:0.5rem;border-bottom:1px solid #1e293b;padding-bottom:0.25rem;">Education</div>`)
	for _, e := range items {
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.6rem;"><div style="font-size:0.72rem;font-weight:600;color:#fff;">%s</div><div style="font-size:0.65rem;color:#94a3b8;">%s</div><div style="font-size:0.6rem;color:#64748b;margin-top:0.1rem;">%s — %s</div></div>`,
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
	categoryOrder := []string{"Languages", "Frameworks", "Tools", "Databases", "Cloud"}
	displayNames := map[string]string{
		"Languages":  "Languages",
		"Frameworks": "Frameworks & Libraries",
		"Tools":      "Tools & Platforms",
		"Databases":  "Databases",
		"Cloud":      "Cloud & DevOps",
	}

	if style == "rt" {
		b.WriteString(`<div style="margin-bottom:1.2rem;">`)
		b.WriteString(`<div style="font-size:0.75rem;font-weight:bold;color:#10b981;text-transform:lowercase;border-bottom:1px solid #334155;padding-bottom:0.2rem;margin-bottom:0.5rem;">// skills</div>`)
		for _, cat := range categoryOrder {
			items, ok := categorizedSkills[cat]
			if !ok || len(items) == 0 {
				continue
			}
			var tags []string
			for _, s := range items {
				tags = append(tags, fmt.Sprintf(`<span style="font-size:0.62rem;padding:0.05rem 0.35rem;background:#1e293b;border:1px solid #334155;color:#10b981;border-radius:3px;">%s</span>`, escape(s)))
			}
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.4rem;"><span style="font-size:0.68rem;font-weight:600;color:#94a3b8;margin-right:0.4rem;">%s:</span> <span style="display:inline-flex;flex-wrap:wrap;gap:0.25rem;">%s</span></div>`, displayNames[cat], strings.Join(tags, " ")))
		}
		b.WriteString(`</div>`)
	} else if style == "rm" {
		b.WriteString(`<div style="margin-top:1.2rem;"><div style="font-size:0.78rem;font-weight:700;text-transform:uppercase;letter-spacing:1.5px;color:#0f172a;margin-bottom:0.5rem;font-family:'Lora',serif;border-bottom:1px solid #f1f5f9;padding-bottom:0.25rem;">Skills</div>`)
		for _, cat := range categoryOrder {
			items, ok := categorizedSkills[cat]
			if !ok || len(items) == 0 {
				continue
			}
			var tags []string
			for _, s := range items {
				tags = append(tags, fmt.Sprintf(`<span style="font-size:0.62rem;padding:0.1rem 0.4rem;border:1px solid #e2e8f0;border-radius:2px;color:#0f172a;">%s</span>`, escape(s)))
			}
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.4rem;"><span style="font-size:0.7rem;font-weight:600;color:#475569;margin-right:0.4rem;">%s:</span> <span style="display:inline-flex;flex-wrap:wrap;gap:0.25rem;">%s</span></div>`, displayNames[cat], strings.Join(tags, " ")))
		}
		b.WriteString(`</div>`)
	} else if style == "mod" {
		b.WriteString(`<div><div style="font-size:0.7rem;font-weight:700;text-transform:uppercase;letter-spacing:1px;color:#38bdf8;margin-top:1.5rem;margin-bottom:0.5rem;border-bottom:1px solid #1e293b;padding-bottom:0.25rem;">Skills</div>`)
		for _, cat := range categoryOrder {
			items, ok := categorizedSkills[cat]
			if !ok || len(items) == 0 {
				continue
			}
			b.WriteString(fmt.Sprintf(`<div style="margin-top:0.4rem;"><div style="font-size:0.6rem;font-weight:700;color:#94a3b8;text-transform:uppercase;letter-spacing:0.5px;margin-bottom:0.2rem;">%s</div>`, displayNames[cat]))
			var tags []string
			for _, s := range items {
				tags = append(tags, fmt.Sprintf(`<span style="font-size:0.6rem;padding:0.05rem 0.35rem;background:#1e293b;color:#e2e8f0;border-radius:3px;margin-bottom:0.15rem;display:inline-block;margin-right:0.15rem;">%s</span>`, escape(s)))
			}
			b.WriteString(fmt.Sprintf(`<div style="display:flex;flex-wrap:wrap;gap:0.15rem;">%s</div></div>`, strings.Join(tags, "")))
		}
		b.WriteString(`</div>`)
	} else if style == "exec" {
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.8rem;font-weight:700;color:#b45309;text-transform:uppercase;letter-spacing:1.5px;border-bottom:1px solid #e2e8f0;padding-bottom:0.2rem;margin-bottom:0.5rem;font-family:'Playfair Display',serif;">Technical Skills</div>`))
		for _, cat := range categoryOrder {
			items, ok := categorizedSkills[cat]
			if !ok || len(items) == 0 {
				continue
			}
			var tags []string
			for _, s := range items {
				tags = append(tags, fmt.Sprintf(`<span style="font-size:0.62rem;padding:0.05rem 0.4rem;background:#fdfbf7;border:1px solid #b45309;border-radius:2px;color:#0f1e36;font-family:'Lora',serif;">%s</span>`, escape(s)))
			}
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.4rem;"><span style="font-size:0.72rem;font-weight:600;color:#0f1e36;font-family:'Lora',serif;margin-right:0.4rem;">%s:</span> <span style="display:inline-flex;flex-wrap:wrap;gap:0.25rem;">%s</span></div>`, displayNames[cat], strings.Join(tags, " ")))
		}
		b.WriteString(`</div>`)
	} else if style == "cr" {
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.8rem;font-weight:700;color:#6d28d9;text-transform:uppercase;letter-spacing:0.5px;margin-bottom:0.5rem;">⚡ Skills</div>`))
		for _, cat := range categoryOrder {
			items, ok := categorizedSkills[cat]
			if !ok || len(items) == 0 {
				continue
			}
			var tags []string
			for _, s := range items {
				tags = append(tags, fmt.Sprintf(`<span style="font-size:0.6rem;padding:0.1rem 0.45rem;background:#f5f3ff;color:#6d28d9;border:1px solid #ddd6fe;border-radius:20px;font-weight:500;margin-right:0.15rem;display:inline-block;margin-bottom:0.15rem;">%s</span>`, escape(s)))
			}
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.4rem;"><span style="font-size:0.7rem;font-weight:700;color:#1e1b4b;margin-right:0.4rem;">%s:</span> <br><span style="display:inline-flex;flex-wrap:wrap;gap:0.15rem;margin-top:0.2rem;">%s</span></div>`, displayNames[cat], strings.Join(tags, "")))
		}
		b.WriteString(`</div>`)
	} else if style == "tl" {
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.78rem;font-weight:700;color:#4f46e5;text-transform:uppercase;letter-spacing:1px;border-bottom:2px solid #e2e8f0;padding-bottom:0.2rem;margin-bottom:0.5rem;width:fit-content;">Skills</div>`))
		for _, cat := range categoryOrder {
			items, ok := categorizedSkills[cat]
			if !ok || len(items) == 0 {
				continue
			}
			var tags []string
			for _, s := range items {
				tags = append(tags, fmt.Sprintf(`<span style="font-size:0.6rem;padding:0.1rem 0.4rem;background:#faf5ff;color:#6b21a8;border:1px solid #e9d5ff;border-radius:4px;font-weight:500;">%s</span>`, escape(s)))
			}
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.4rem;"><span style="font-size:0.7rem;font-weight:600;color:#4f46e5;margin-right:0.4rem;">%s:</span> <span style="display:inline-flex;flex-wrap:wrap;gap:0.25rem;">%s</span></div>`, displayNames[cat], strings.Join(tags, " ")))
		}
		b.WriteString(`</div>`)
	} else if style == "cl" {
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.78rem;font-weight:700;color:#0f172a;text-transform:uppercase;letter-spacing:1px;border-bottom:2px solid #e2e8f0;padding-bottom:0.2rem;margin-bottom:0.5rem;width:fit-content;">Skills</div>`))
		for _, cat := range categoryOrder {
			items, ok := categorizedSkills[cat]
			if !ok || len(items) == 0 {
				continue
			}
			var tags []string
			for _, s := range items {
				tags = append(tags, fmt.Sprintf(`<span style="font-size:0.6rem;padding:0.1rem 0.4rem;background:#f0fdfa;color:#0f766e;border:1px solid #ccfbf1;border-radius:3px;font-weight:500;">%s</span>`, escape(s)))
			}
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.4rem;"><span style="font-size:0.68rem;font-weight:600;color:#0f172a;margin-right:0.4rem;">%s:</span> <span style="display:inline-flex;flex-wrap:wrap;gap:0.25rem;">%s</span></div>`, displayNames[cat], strings.Join(tags, " ")))
		}
		b.WriteString(`</div>`)
	} else { // compact, cp
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.75rem;font-weight:700;color:#0d9488;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.4rem;border-bottom:1px solid #f1f5f9;padding-bottom:0.2rem;">Skills</div>`))
		for _, cat := range categoryOrder {
			items, ok := categorizedSkills[cat]
			if !ok || len(items) == 0 {
				continue
			}
			var tags []string
			for _, s := range items {
				tags = append(tags, fmt.Sprintf(`<span style="font-size:0.62rem;padding:0.1rem 0.4rem;background:#f0fdfa;color:#0f766e;border:1px solid #ccfbf1;border-radius:3px;font-weight:500;">%s</span>`, escape(s)))
			}
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.4rem;"><span style="font-size:0.68rem;font-weight:600;color:#0d9488;margin-right:0.4rem;">%s:</span> <span style="display:inline-flex;flex-wrap:wrap;gap:0.25rem;">%s</span></div>`, displayNames[cat], strings.Join(tags, " ")))
		}
		b.WriteString(`</div>`)
	}
	return b.String()
}

// ── Achievements render function ──────────────────────────
func renderAchievementsBlock(items []achievement, escape func(string) string, style string) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	if style == "rt" {
		b.WriteString(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.75rem;font-weight:bold;color:#10b981;text-transform:lowercase;border-bottom:1px solid #334155;padding-bottom:0.2rem;margin-bottom:0.5rem;">// achievements</div>`)
		for _, a := range items {
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.5rem;"><div style="font-size:0.75rem;font-weight:600;color:#38bdf8;">%s <span style="font-size:0.65rem;color:#64748b;">[%s]</span></div><div style="font-size:0.68rem;color:#cbd5e1;line-height:1.4;">%s</div></div>`,
				escape(a.Title), escape(a.Date), escape(a.Description)))
		}
		b.WriteString(`</div>`)
	} else if style == "rm" {
		b.WriteString(`<div style="margin-top:1.2rem;"><div style="font-size:0.78rem;font-weight:700;text-transform:uppercase;letter-spacing:1.5px;color:#0f172a;margin-bottom:0.5rem;font-family:'Lora',serif;border-bottom:1px solid #f1f5f9;padding-bottom:0.25rem;">Achievements</div>`)
		for _, a := range items {
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.5rem;"><div style="display:flex;justify-content:space-between;align-items:baseline;"><div style="font-size:0.72rem;font-weight:700;color:#0f172a;">%s</div><span style="font-size:0.62rem;color:#64748b;font-weight:500;font-style:italic;">%s</span></div><div style="font-size:0.68rem;color:#334155;line-height:1.45;margin-top:0.15rem;">%s</div></div>`,
				escape(a.Title), escape(a.Date), escape(a.Description)))
		}
		b.WriteString(`</div>`)
	} else if style == "mod" {
		b.WriteString(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.78rem;font-weight:700;text-transform:uppercase;letter-spacing:1px;color:#0f172a;border-bottom:2px solid #38bdf8;padding-bottom:0.25rem;margin-bottom:0.5rem;width:fit-content;">Achievements</div>`)
		for _, a := range items {
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.5rem;"><div style="display:flex;justify-content:space-between;align-items:baseline;"><div style="font-size:0.72rem;font-weight:700;color:#0f172a;">%s</div><span style="font-size:0.62rem;color:#64748b;font-weight:500;">%s</span></div><div style="font-size:0.65rem;color:#4b5563;line-height:1.45;margin-top:0.1rem;">%s</div></div>`,
				escape(a.Title), escape(a.Date), escape(a.Description)))
		}
		b.WriteString(`</div>`)
	} else if style == "exec" {
		b.WriteString(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.8rem;font-weight:700;color:#b45309;text-transform:uppercase;letter-spacing:1.5px;border-bottom:1px solid #e2e8f0;padding-bottom:0.2rem;margin-bottom:0.5rem;font-family:'Playfair Display',serif;">Achievements</div>`)
		for _, a := range items {
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.5rem;"><div style="display:flex;justify-content:space-between;align-items:baseline;"><div style="font-size:0.75rem;font-weight:700;color:#0f1e36;font-family:'Playfair Display',serif;">%s</div><span style="font-size:0.65rem;color:#64748b;font-family:'Lora',serif;font-style:italic;font-weight:500;">%s</span></div><div style="font-size:0.7rem;color:#334155;line-height:1.5;font-family:'Lora',serif;margin-top:0.15rem;">%s</div></div>`,
				escape(a.Title), escape(a.Date), escape(a.Description)))
		}
		b.WriteString(`</div>`)
	} else if style == "cr" {
		b.WriteString(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.8rem;font-weight:700;color:#a855f7;text-transform:uppercase;letter-spacing:0.5px;margin-bottom:0.5rem;">🏆 Achievements</div>`)
		for _, a := range items {
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.6rem;background:#faf5ff;border-radius:8px;padding:0.5rem 0.6rem;border-left:3px solid #a855f7;"><div style="display:flex;justify-content:space-between;align-items:baseline;margin-bottom:0.2rem;"><span style="font-size:0.75rem;font-weight:700;color:#581c87;">%s</span><span style="font-size:0.62rem;color:#701a75;font-weight:600;background:#f3e8ff;padding:0.1rem 0.35rem;border-radius:4px;">%s</span></div><div style="font-size:0.65rem;color:#4a044e;line-height:1.45;">%s</div></div>`,
				escape(a.Title), escape(a.Date), escape(a.Description)))
		}
		b.WriteString(`</div>`)
	} else if style == "tl" {
		b.WriteString(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.78rem;font-weight:700;color:#4f46e5;text-transform:uppercase;letter-spacing:1px;border-bottom:2px solid #e2e8f0;padding-bottom:0.2rem;margin-bottom:0.5rem;width:fit-content;">Achievements</div>
			<div style="position:relative;padding-left:0.8rem;margin-top:0.4rem;">
			<div style="position:absolute;left:0;top:0;bottom:0;width:2px;background:#8b5cf6;"></div>`)
		for i, a := range items {
			extra := ""
			if i < len(items)-1 {
				extra = `margin-bottom:0.6rem;`
			}
			b.WriteString(fmt.Sprintf(`<div style="position:relative;padding-left:1rem;%s"><div style="position:absolute;left:-1.15rem;top:0.25rem;width:8px;height:8px;border-radius:50%%;background:#8b5cf6;border:2px solid #fff;box-shadow:0 0 0 2px #8b5cf6;"></div><div style="display:flex;justify-content:space-between;align-items:baseline;"><div style="font-size:0.72rem;font-weight:700;color:#0f172a;">%s <span style="font-weight:500;color:#8b5cf6;font-size:0.65rem;margin-left:0.4rem;">[%s]</span></div></div><div style="font-size:0.65rem;color:#4b5563;line-height:1.45;margin-top:0.15rem;">%s</div></div>`,
				extra, escape(a.Title), escape(a.Date), escape(a.Description)))
		}
		b.WriteString(`</div></div>`)
	} else if style == "cl" {
		b.WriteString(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.78rem;font-weight:700;color:#0f172a;text-transform:uppercase;letter-spacing:1px;border-bottom:2px solid #e2e8f0;padding-bottom:0.2rem;margin-bottom:0.5rem;width:fit-content;">Achievements</div>`)
		for _, a := range items {
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.4rem;"><div style="display:flex;justify-content:space-between;align-items:baseline;"><div style="font-size:0.72rem;font-weight:700;color:#0f172a;">%s <span style="font-weight:500;color:#6b7280;font-size:0.65rem;margin-left:0.4rem;">(%s)</span></div></div><div style="font-size:0.65rem;color:#4b5563;line-height:1.45;margin-top:0.1rem;">%s</div></div>`,
				escape(a.Title), escape(a.Date), escape(a.Description)))
		}
		b.WriteString(`</div>`)
	} else { // compact, cp
		b.WriteString(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.75rem;font-weight:700;color:#0d9488;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.4rem;border-bottom:1px solid #f1f5f9;padding-bottom:0.2rem;">Achievements</div>`)
		for _, a := range items {
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.4rem;"><div style="display:flex;justify-content:space-between;align-items:baseline;"><div style="font-size:0.72rem;font-weight:700;color:#0f172a;">%s <span style="font-weight:500;color:#6b7280;font-size:0.65rem;margin-left:0.4rem;">(%s)</span></div></div><div style="font-size:0.65rem;color:#4b5563;line-height:1.45;margin-top:0.1rem;">%s</div></div>`,
				escape(a.Title), escape(a.Date), escape(a.Description)))
		}
		b.WriteString(`</div>`)
	}
	return b.String()
}

// ── Certifications render function ────────────────────────
func renderCertificationsBlock(items []certification, escape func(string) string, style string) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	if style == "rt" {
		b.WriteString(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.75rem;font-weight:bold;color:#10b981;text-transform:lowercase;border-bottom:1px solid #334155;padding-bottom:0.2rem;margin-bottom:0.5rem;">// certifications</div>`)
		for _, c := range items {
			linkStr := ""
			if c.Link != "" {
				linkStr = fmt.Sprintf(` <a href="%s" style="color:#38bdf8;text-decoration:underline;font-size:0.62rem;">[link]</a>`, escape(c.Link))
			}
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.4rem;"><div style="font-size:0.72rem;font-weight:600;color:#38bdf8;">%s%s</div><div style="font-size:0.65rem;color:#94a3b8;">%s <span style="color:#64748b;">[%s]</span></div></div>`,
				escape(c.Title), linkStr, escape(c.Issuer), escape(c.Date)))
		}
		b.WriteString(`</div>`)
	} else if style == "rm" {
		b.WriteString(`<div style="margin-top:1.2rem;"><div style="font-size:0.78rem;font-weight:700;text-transform:uppercase;letter-spacing:1.5px;color:#0f172a;margin-bottom:0.5rem;font-family:'Lora',serif;border-bottom:1px solid #f1f5f9;padding-bottom:0.25rem;">Certifications</div>`)
		for _, c := range items {
			linkStr := ""
			if c.Link != "" {
				linkStr = fmt.Sprintf(` <a href="%s" style="color:#64748b;font-size:0.62rem;text-decoration:underline;">[link]</a>`, escape(c.Link))
			}
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.4rem;display:flex;justify-content:space-between;align-items:baseline;"><div style="font-size:0.72rem;font-weight:600;color:#0f172a;">%s%s <span style="font-weight:400;color:#64748b;">— %s</span></div><div style="font-size:0.65rem;color:#64748b;margin-left:auto;">%s</div></div>`,
				escape(c.Title), linkStr, escape(c.Issuer), escape(c.Date)))
		}
		b.WriteString(`</div>`)
	} else if style == "mod" {
		b.WriteString(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.78rem;font-weight:700;text-transform:uppercase;letter-spacing:1px;color:#0f172a;border-bottom:2px solid #38bdf8;padding-bottom:0.25rem;margin-bottom:0.5rem;width:fit-content;">Certifications</div>`)
		for _, c := range items {
			linkStr := ""
			if c.Link != "" {
				linkStr = fmt.Sprintf(` <a href="%s" style="color:#38bdf8;font-size:0.62rem;text-decoration:underline;">[link]</a>`, escape(c.Link))
			}
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.4rem;display:flex;justify-content:space-between;align-items:baseline;"><div style="font-size:0.72rem;font-weight:700;color:#0f172a;">%s%s <span style="font-weight:500;color:#38bdf8;">@ %s</span></div><span style="font-size:0.62rem;color:#64748b;font-weight:500;margin-left:auto;">%s</span></div>`,
				escape(c.Title), linkStr, escape(c.Issuer), escape(c.Date)))
		}
		b.WriteString(`</div>`)
	} else if style == "exec" {
		b.WriteString(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.8rem;font-weight:700;color:#b45309;text-transform:uppercase;letter-spacing:1.5px;border-bottom:1px solid #e2e8f0;padding-bottom:0.2rem;margin-bottom:0.5rem;font-family:'Playfair Display',serif;">Certifications</div>`)
		for _, c := range items {
			linkStr := ""
			if c.Link != "" {
				linkStr = fmt.Sprintf(` <a href="%s" style="color:#b45309;font-size:0.65rem;text-decoration:underline;">[link]</a>`, escape(c.Link))
			}
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.4rem;display:flex;justify-content:space-between;align-items:baseline;"><div style="font-size:0.75rem;font-weight:700;color:#0f1e36;font-family:'Playfair Display',serif;">%s%s <span style="font-weight:400;color:#475569;font-family:'Lora',serif;font-style:italic;">from %s</span></div><div style="font-size:0.65rem;color:#64748b;font-family:'Lora',serif;font-style:italic;margin-left:auto;">%s</div></div>`,
				escape(c.Title), linkStr, escape(c.Issuer), escape(c.Date)))
		}
		b.WriteString(`</div>`)
	} else if style == "cr" {
		b.WriteString(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.8rem;font-weight:700;color:#6d28d9;text-transform:uppercase;letter-spacing:0.5px;margin-bottom:0.5rem;">📜 Certifications</div>`)
		for _, c := range items {
			linkStr := ""
			if c.Link != "" {
				linkStr = fmt.Sprintf(` <a href="%s" style="color:#7c3aed;font-size:0.62rem;text-decoration:underline;">[link]</a>`, escape(c.Link))
			}
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.4rem;background:#f5f3ff;border-radius:8px;padding:0.4rem 0.6rem;border-left:3px solid #a855f7;"><div style="display:flex;justify-content:space-between;align-items:baseline;"><div style="font-size:0.72rem;font-weight:700;color:#1e1b4b;">%s%s <span style="font-weight:500;color:#7c3aed;">— %s</span></div><span style="font-size:0.62rem;color:#6b21a8;font-weight:500;margin-left:auto;">%s</span></div></div>`,
				escape(c.Title), linkStr, escape(c.Issuer), escape(c.Date)))
		}
		b.WriteString(`</div>`)
	} else if style == "tl" {
		b.WriteString(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.78rem;font-weight:700;color:#4f46e5;text-transform:uppercase;letter-spacing:1px;border-bottom:2px solid #e2e8f0;padding-bottom:0.2rem;margin-bottom:0.5rem;width:fit-content;">Certifications</div>
			<div style="position:relative;padding-left:0.8rem;margin-top:0.4rem;">
			<div style="position:absolute;left:0;top:0;bottom:0;width:2px;background:#8b5cf6;"></div>`)
		for i, c := range items {
			extra := ""
			if i < len(items)-1 {
				extra = `margin-bottom:0.6rem;`
			}
			linkStr := ""
			if c.Link != "" {
				linkStr = fmt.Sprintf(` <a href="%s" style="color:#8b5cf6;font-size:0.6rem;text-decoration:underline;">[link]</a>`, escape(c.Link))
			}
			b.WriteString(fmt.Sprintf(`<div style="position:relative;padding-left:1rem;%s"><div style="position:absolute;left:-1.15rem;top:0.25rem;width:8px;height:8px;border-radius:50%%;background:#8b5cf6;border:2px solid #fff;box-shadow:0 0 0 2px #8b5cf6;"></div><div style="display:flex;justify-content:space-between;align-items:baseline;"><div style="font-size:0.72rem;font-weight:700;color:#0f172a;">%s%s <span style="font-weight:500;color:#8b5cf6;">— %s</span></div><span style="font-size:0.62rem;color:#6b7280;font-weight:500;margin-left:auto;">%s</span></div></div>`,
				extra, escape(c.Title), linkStr, escape(c.Issuer), escape(c.Date)))
		}
		b.WriteString(`</div></div>`)
	} else if style == "cl" {
		b.WriteString(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.78rem;font-weight:700;color:#0f172a;text-transform:uppercase;letter-spacing:1px;border-bottom:2px solid #e2e8f0;padding-bottom:0.2rem;margin-bottom:0.5rem;width:fit-content;">Certifications</div>`)
		for _, c := range items {
			linkStr := ""
			if c.Link != "" {
				linkStr = fmt.Sprintf(` <a href="%s" style="color:#4f46e5;font-size:0.6rem;text-decoration:underline;">[link]</a>`, escape(c.Link))
			}
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.3rem;display:flex;justify-content:space-between;align-items:baseline;"><div style="font-size:0.72rem;font-weight:700;color:#0f172a;">%s%s <span style="font-weight:400;color:#64748b;">— %s</span></div><span style="font-size:0.62rem;color:#6b7280;font-weight:500;margin-left:auto;">%s</span></div>`,
				escape(c.Title), linkStr, escape(c.Issuer), escape(c.Date)))
		}
		b.WriteString(`</div>`)
	} else { // compact, cp
		b.WriteString(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.75rem;font-weight:700;color:#0d9488;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.4rem;border-bottom:1px solid #f1f5f9;padding-bottom:0.2rem;">Certifications</div>`)
		for _, c := range items {
			linkStr := ""
			if c.Link != "" {
				linkStr = fmt.Sprintf(` <a href="%s" style="color:#0d9488;font-size:0.6rem;text-decoration:underline;">[link]</a>`, escape(c.Link))
			}
			b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.3rem;display:flex;justify-content:space-between;align-items:baseline;"><div style="font-size:0.72rem;font-weight:700;color:#0f172a;">%s%s <span style="font-weight:400;color:#64748b;">— %s</span></div><span style="font-size:0.62rem;color:#6b7280;font-weight:500;margin-left:auto;">%s</span></div>`,
				escape(c.Title), linkStr, escape(c.Issuer), escape(c.Date)))
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
	b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.8rem;font-weight:700;color:#b45309;text-transform:uppercase;letter-spacing:1.5px;border-bottom:1px solid #e2e8f0;padding-bottom:0.2rem;margin-bottom:0.5rem;font-family:'Playfair Display',serif;">Professional Experience</div>`))
	for _, e := range items {
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.8rem;"><div style="display:flex;justify-content:space-between;align-items:baseline;margin-bottom:0.15rem;"><span style="font-size:0.75rem;font-weight:700;color:#0f1e36;font-family:'Playfair Display',serif;">%s <span style="font-weight:400;color:#475569;font-family:'Lora',serif;font-style:italic;">at %s</span></span><span style="font-size:0.65rem;color:#64748b;font-family:'Lora',serif;font-style:italic;">%s — %s</span></div><div style="font-size:0.68rem;color:#334155;line-height:1.5;font-family:'Lora',serif;white-space:pre-line;">%s</div></div>`,
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
	b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.8rem;font-weight:700;color:#b45309;text-transform:uppercase;letter-spacing:1.5px;border-bottom:1px solid #e2e8f0;padding-bottom:0.2rem;margin-bottom:0.5rem;font-family:'Playfair Display',serif;">Education</div>`))
	for _, e := range items {
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.4rem;display:flex;justify-content:space-between;align-items:baseline;"><div style="font-size:0.75rem;font-weight:700;color:#0f1e36;font-family:'Playfair Display',serif;">%s <span style="font-weight:400;color:#475569;font-family:'Lora',serif;font-style:italic;">from %s</span></div><div style="font-size:0.65rem;color:#64748b;font-family:'Lora',serif;font-style:italic;margin-left:auto;">%s — %s</div></div>`,
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
	b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.8rem;font-weight:700;color:#6d28d9;text-transform:uppercase;letter-spacing:0.5px;margin-bottom:0.5rem;">💼 Experience</div>`))
	for _, e := range items {
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.6rem;background:#fbfaff;border-radius:8px;padding:0.5rem 0.6rem;border-left:3px solid #8b5cf6;"><div style="display:flex;justify-content:space-between;align-items:baseline;margin-bottom:0.15rem;"><span style="font-size:0.75rem;font-weight:700;color:#1e1b4b;">%s <span style="font-weight:500;color:#7c3aed;">@ %s</span></span><span style="font-size:0.62rem;color:#6b21a8;font-weight:500;">%s — %s</span></div><div style="font-size:0.65rem;color:#4c1d95;line-height:1.45;white-space:pre-line;">%s</div></div>`,
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
	b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.8rem;font-weight:700;color:#0d9488;text-transform:uppercase;letter-spacing:0.5px;margin-bottom:0.5rem;">🎓 Education</div>`))
	for _, e := range items {
		b.WriteString(fmt.Sprintf(`<div style="margin-bottom:0.4rem;background:#f0fdfa;border-radius:8px;padding:0.4rem 0.6rem;border-left:3px solid #14b8a6;"><div style="display:flex;justify-content:space-between;align-items:baseline;"><div style="font-size:0.72rem;font-weight:700;color:#0f3731;">%s <span style="font-weight:500;color:#0d9488;">at %s</span></div><span style="font-size:0.62rem;color:#0f766e;font-weight:500;margin-left:auto;">%s — %s</span></div></div>`,
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
	b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.78rem;font-weight:700;color:#4f46e5;text-transform:uppercase;letter-spacing:1px;border-bottom:2px solid #e2e8f0;padding-bottom:0.2rem;margin-bottom:0.5rem;width:fit-content;">Experience</div>
		<div style="position:relative;padding-left:0.8rem;margin-top:0.4rem;">
		<div style="position:absolute;left:0;top:0;bottom:0;width:2px;background:#8b5cf6;"></div>`))
	for i, e := range items {
		extra := ""
		if i < len(items)-1 {
			extra = `margin-bottom:0.6rem;`
		}
		b.WriteString(fmt.Sprintf(`<div style="position:relative;padding-left:1rem;%s"><div style="position:absolute;left:-1.15rem;top:0.25rem;width:8px;height:8px;border-radius:50%%;background:#8b5cf6;border:2px solid #fff;box-shadow:0 0 0 2px #8b5cf6;"></div><div style="display:flex;justify-content:space-between;align-items:baseline;margin-bottom:0.15rem;"><span style="font-size:0.75rem;font-weight:700;color:#0f172a;">%s <span style="font-weight:500;color:#8b5cf6;">@ %s</span></span><span style="font-size:0.62rem;color:#6b7280;font-weight:500;">%s — %s</span></div><div style="font-size:0.65rem;color:#4b5563;line-height:1.45;white-space:pre-line;">%s</div></div>`,
			extra, escape(e.Role), escape(e.Company), escape(e.From), escape(e.To), escape(e.Description)))
	}
	b.WriteString(`</div></div>`)
	return b.String()
}

func renderEduBlockTimeline(items []edu, escape func(string) string) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<div style="margin-bottom:1.2rem;"><div style="font-size:0.78rem;font-weight:700;color:#4f46e5;text-transform:uppercase;letter-spacing:1px;border-bottom:2px solid #e2e8f0;padding-bottom:0.2rem;margin-bottom:0.5rem;width:fit-content;">Education</div>
		<div style="position:relative;padding-left:0.8rem;margin-top:0.4rem;">
		<div style="position:absolute;left:0;top:0;bottom:0;width:2px;background:#8b5cf6;"></div>`))
	for i, e := range items {
		extra := ""
		if i < len(items)-1 {
			extra = `margin-bottom:0.6rem;`
		}
		b.WriteString(fmt.Sprintf(`<div style="position:relative;padding-left:1rem;%s"><div style="position:absolute;left:-1.15rem;top:0.25rem;width:8px;height:8px;border-radius:50%%;background:#8b5cf6;border:2px solid #fff;box-shadow:0 0 0 2px #8b5cf6;"></div><div style="display:flex;justify-content:space-between;align-items:baseline;"><div style="font-size:0.72rem;font-weight:700;color:#0f172a;">%s <span style="font-weight:500;color:#8b5cf6;">at %s</span></div><span style="font-size:0.62rem;color:#6b7280;font-weight:500;margin-left:auto;">%s — %s</span></div></div>`,
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
	normalizeSkills(&req)

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
	normalizeSkills(&req.GenerateRequest)

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
		Projects: req.Projects, Achievements: req.Achievements,
		Certifications: req.Certifications,
		Template:       req.Template,
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
