package llm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"time-retention/internal/models"
)

// GeminiResponse defines the JSON structure of the Gemini API response
type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

// ComputeCustomerHash generates a SHA256 hash of the customer's data to check for modifications.
func ComputeCustomerHash(details *models.CustomerDetails) string {
	var usageStrs []string
	for _, u := range details.UsageHistory {
		usageStrs = append(usageStrs, fmt.Sprintf("%s:%.2f:%.2f", u.MonthStr, u.DownloadGB, u.UploadGB))
	}
	raw := fmt.Sprintf("%s|%s|%.2f|%d|%s|%s",
		details.FullName,
		details.PlanName,
		details.MonthlyFee,
		details.TenureMonths,
		details.ContractEndDate,
		strings.Join(usageStrs, ","),
	)
	h := sha256.New()
	h.Write([]byte(raw))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// GeneratePitch creates a personalized pitch for a customer.
// It will call Google Gemini if GEMINI_API_KEY is available, or fall back to an offline template-based pitch generator.
func GeneratePitch(ctx context.Context, details *models.CustomerDetails) (string, string, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")

	// Calculate averages
	var totalDownload, totalUpload float64
	count := float64(len(details.UsageHistory))
	if count == 0 {
		count = 1 // Prevent division by zero
	}
	for _, u := range details.UsageHistory {
		totalDownload += u.DownloadGB
		totalUpload += u.UploadGB
	}
	avgDownload := totalDownload / count
	avgUpload := totalUpload / count

	prompt := fmt.Sprintf(`Write a highly personalized, friendly recontract sales pitch for the following customer. Mention their plan name, monthly price, and tenure. Reference their average monthly download usage (%.2f GB) and upload usage (%.2f GB) to explain why they should upgrade or maintain their plan. Keep it concise (less than 120 words).

Customer Details:
- Name: %s
- Current Plan: %s
- Monthly Fee: $%.2f
- Tenure: %d months
- Contract Expiration: %s

Be professional and compelling. Highlight the benefits of recontracting now.`,
		avgDownload, avgUpload, details.FullName, details.PlanName, details.MonthlyFee, details.TenureMonths, details.ContractEndDate)

	if apiKey == "" {
		// Use Mock generator
		pitch := generateMockPitch(details, avgDownload, avgUpload)
		return pitch, "mock-gemini-1.5-flash", nil
	}

	// Call Google Gemini API
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=%s", apiKey)
	
	reqBody, err := json.Marshal(map[string]interface{}{
		"contents": []interface{}{
			map[string]interface{}{
				"parts": []interface{}{
					map[string]string{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"maxOutputTokens": 200,
			"temperature":     0.7,
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", "", fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("gemini api call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		log.Printf("Gemini API error (HTTP %d): %s. Falling back to mock generator.", resp.StatusCode, string(respBytes))
		return generateMockPitch(details, avgDownload, avgUpload), "mock-gemini-1.5-flash", nil
	}

	var geminiResp GeminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return "", "", fmt.Errorf("failed to decode gemini response: %w", err)
	}

	if len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
		pitchText := strings.TrimSpace(geminiResp.Candidates[0].Content.Parts[0].Text)
		return pitchText, "gemini-1.5-flash", nil
	}

	return "", "", fmt.Errorf("empty response received from Gemini API")
}

func generateMockPitch(details *models.CustomerDetails, avgDownload, avgUpload float64) string {
	// A high-quality mock response generator that returns realistic, personalized pitches.
	var pitchBuilder strings.Builder

	pitchBuilder.WriteString(fmt.Sprintf("Dear %s,\n\n", details.FullName))
	pitchBuilder.WriteString(fmt.Sprintf("Thank you for being with us for %d months on our %s plan! ", details.TenureMonths, details.PlanName))
	
	pitchBuilder.WriteString(fmt.Sprintf("As your contract ends on %s, we want to help you stay connected at the best value. ", details.ContractEndDate))
	
	if avgDownload > 500 {
		pitchBuilder.WriteString(fmt.Sprintf("We noticed you're a power user, averaging %.2f GB of downloads monthly. ", avgDownload))
		pitchBuilder.WriteString(fmt.Sprintf("To support your high-bandwidth activities, we recommend renewing on our 2Gbps plan to unlock double the speed! "))
	} else if details.PlanName == "100Mbps" {
		pitchBuilder.WriteString(fmt.Sprintf("With your average monthly download of %.2f GB, you are utilizing your connection well. ", avgDownload))
		pitchBuilder.WriteString(fmt.Sprintf("We recommend upgrading to our 500Mbps plan to enjoy smoother streaming and faster file transfers. "))
	} else {
		pitchBuilder.WriteString(fmt.Sprintf("You've been average downloading %.2f GB and uploading %.2f GB monthly on your current $%s plan. ", avgDownload, avgUpload, fmt.Sprintf("%.2f", details.MonthlyFee)))
		pitchBuilder.WriteString("Recontracting today lets you lock in this rate or choose to upgrade for an even faster experience. ")
	}

	pitchBuilder.WriteString("\n\nLet's get this set up for you today!")
	return pitchBuilder.String()
}
