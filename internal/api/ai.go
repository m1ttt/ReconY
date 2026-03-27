package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type askAIRequest struct {
	Query string `json:"query"`
}

type aiServiceRequest struct {
	Query         string `json:"query"`
	OpenAIAPIKey  string `json:"openai_api_key,omitempty"`
	OpenAIBaseURL string `json:"openai_base_url,omitempty"`
	OpenAIModel   string `json:"openai_model,omitempty"`
	TavilyAPIKey  string `json:"tavily_api_key,omitempty"`
}

type aiServiceResponse struct {
	Result string `json:"result"`
}

func (s *Server) askAI(w http.ResponseWriter, r *http.Request) {
	var req askAIRequest
	if err := decodeJSON(r, &req); err != nil || req.Query == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}

	aiURL := s.Config.APIKeys.AIServiceURL
	if aiURL == "" {
		aiURL = "http://localhost:8000"
	}
	endpoint := fmt.Sprintf("%s/research", aiURL)

	payload := aiServiceRequest{
		Query:         req.Query,
		OpenAIAPIKey:  s.Config.APIKeys.OpenAIKey,
		OpenAIBaseURL: s.Config.APIKeys.OpenAIBaseURL,
		OpenAIModel:   s.Config.APIKeys.OpenAIModel,
		TavilyAPIKey:  s.Config.APIKeys.TavilyKey,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal AI request")
		return
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	httpReq, err := http.NewRequestWithContext(r.Context(), "POST", endpoint, bytes.NewBuffer(bodyBytes))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create AI request")
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("failed to reach AI service: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		writeError(w, http.StatusBadGateway, fmt.Sprintf("AI service returned error: %s", string(respBody)))
		return
	}

	var aiResp aiServiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&aiResp); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse AI service response")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"result": aiResp.Result})
}
