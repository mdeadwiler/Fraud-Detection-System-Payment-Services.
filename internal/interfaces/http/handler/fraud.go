package handler

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	fraudapp "fraud-detecction-system/internal/application/fraud"
	"fraud-detecction-system/internal/domain/fraud"
)

// FraudHandler handles fraud-related HTTP requests
type FraudHandler struct {
	detectFraudUseCase *fraudapp.DetectFraudUseCase
	fraudService       *fraud.Service
}

// NewFraudHandler creates a new fraud handler
func NewFraudHandler(detectFraudUseCase *fraudapp.DetectFraudUseCase, fraudService *fraud.Service) *FraudHandler {
	return &FraudHandler{
		detectFraudUseCase: detectFraudUseCase,
		fraudService:       fraudService,
	}
}

// AnalyzeTransaction handles POST /api/v1/fraud/analyze
func (h *FraudHandler) AnalyzeTransaction(w http.ResponseWriter, r *http.Request) {
	var req fraudapp.AnalyzeTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	input, err := req.ToInput()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.detectFraudUseCase.Execute(r.Context(), *input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Fraud analysis failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// BatchAnalyze handles POST /api/v1/fraud/analyze/batch
func (h *FraudHandler) BatchAnalyze(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Transactions []fraudapp.AnalyzeTransactionRequest `json:"transactions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	if len(req.Transactions) == 0 {
		writeError(w, http.StatusBadRequest, "No transactions provided")
		return
	}

	if len(req.Transactions) > 100 {
		writeError(w, http.StatusBadRequest, "Maximum 100 transactions per batch")
		return
	}

	inputs := make([]fraudapp.DetectFraudInput, 0, len(req.Transactions))
	for _, txReq := range req.Transactions {
		input, err := txReq.ToInput()
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid transaction: "+err.Error())
			return
		}
		inputs = append(inputs, *input)
	}

	result, err := h.detectFraudUseCase.ExecuteBatch(r.Context(), fraudapp.BatchAnalyzeInput{
		Transactions: inputs,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Batch analysis failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetDecision handles GET /api/v1/fraud/decisions/{id}
func (h *FraudHandler) GetDecision(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		writeError(w, http.StatusBadRequest, "Decision ID is required")
		return
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid decision ID")
		return
	}

	decision, err := h.fraudService.GetDecision(r.Context(), id)
	if err != nil {
		if err == fraud.ErrDecisionNotFound {
			writeError(w, http.StatusNotFound, "Decision not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to get decision: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, decision)
}

// GetDecisionByTransaction handles GET /api/v1/fraud/transactions/{id}/decision
func (h *FraudHandler) GetDecisionByTransaction(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		writeError(w, http.StatusBadRequest, "Transaction ID is required")
		return
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid transaction ID")
		return
	}

	decision, err := h.fraudService.GetDecisionByTransaction(r.Context(), id)
	if err != nil {
		if err == fraud.ErrDecisionNotFound {
			writeError(w, http.StatusNotFound, "Decision not found for transaction")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to get decision: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, decision)
}

// GetUserRiskProfile handles GET /api/v1/fraud/users/{id}/risk
func (h *FraudHandler) GetUserRiskProfile(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		writeError(w, http.StatusBadRequest, "User ID is required")
		return
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	profile, err := h.fraudService.GetUserRiskProfile(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get risk profile: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

// ListCases handles GET /api/v1/fraud/cases
func (h *FraudHandler) ListCases(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "open"
	}

	cases, err := h.fraudService.ListCasesByStatus(r.Context(), fraud.CaseStatus(status), 50, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list cases: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"cases": cases,
		"count": len(cases),
	})
}

// GetCase handles GET /api/v1/fraud/cases/{id}
func (h *FraudHandler) GetCase(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		writeError(w, http.StatusBadRequest, "Case ID is required")
		return
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid case ID")
		return
	}

	fraudCase, err := h.fraudService.GetCase(r.Context(), id)
	if err != nil {
		if err == fraud.ErrCaseNotFound {
			writeError(w, http.StatusNotFound, "Case not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to get case: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, fraudCase)
}

// UpdateCaseRequest represents the request to update a case
type UpdateCaseRequest struct {
	Action        string `json:"action"` // assign, add_note, resolve, close, escalate
	AssigneeID    string `json:"assignee_id,omitempty"`
	Note          string `json:"note,omitempty"`
	Resolution    string `json:"resolution,omitempty"`
	EscalateReason string `json:"escalate_reason,omitempty"`
}

// UpdateCase handles PUT /api/v1/fraud/cases/{id}
func (h *FraudHandler) UpdateCase(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		writeError(w, http.StatusBadRequest, "Case ID is required")
		return
	}

	caseID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid case ID")
		return
	}

	var req UpdateCaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Get user ID from context (would come from auth middleware)
	userID := uuid.New() // Placeholder - should come from auth

	switch req.Action {
	case "assign":
		assigneeID, err := uuid.Parse(req.AssigneeID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid assignee ID")
			return
		}
		if err := h.fraudService.AssignCase(r.Context(), caseID, assigneeID); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to assign case: "+err.Error())
			return
		}

	case "add_note":
		if req.Note == "" {
			writeError(w, http.StatusBadRequest, "Note content is required")
			return
		}
		if err := h.fraudService.AddCaseNote(r.Context(), caseID, userID, req.Note); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to add note: "+err.Error())
			return
		}

	case "resolve":
		if req.Resolution == "" {
			writeError(w, http.StatusBadRequest, "Resolution is required")
			return
		}
		if err := h.fraudService.ResolveCase(r.Context(), caseID, userID, req.Resolution); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to resolve case: "+err.Error())
			return
		}

	case "close":
		if err := h.fraudService.CloseCase(r.Context(), caseID); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to close case: "+err.Error())
			return
		}

	case "escalate":
		if err := h.fraudService.EscalateCase(r.Context(), caseID, req.EscalateReason); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to escalate case: "+err.Error())
			return
		}

	default:
		writeError(w, http.StatusBadRequest, "Invalid action: "+req.Action)
		return
	}

	// Return updated case
	fraudCase, err := h.fraudService.GetCase(r.Context(), caseID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get updated case: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, fraudCase)
}

// ListRules handles GET /api/v1/fraud/rules
func (h *FraudHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.fraudService.ListActiveRules(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list rules: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"rules": rules,
		"count": len(rules),
	})
}

// CreateRule handles POST /api/v1/fraud/rules
func (h *FraudHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Type        string                 `json:"type"`
		Severity    string                 `json:"severity"`
		Action      string                 `json:"action"`
		Config      map[string]interface{} `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Get user ID from context (would come from auth middleware)
	userID := uuid.New() // Placeholder

	rule := fraud.NewRule(
		req.Name,
		req.Description,
		fraud.RuleType(req.Type),
		fraud.RuleSeverity(req.Severity),
		fraud.RuleAction(req.Action),
		userID,
	)
	rule.Config = req.Config

	if err := h.fraudService.CreateRule(r.Context(), rule); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create rule: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, rule)
}

// GetRule handles GET /api/v1/fraud/rules/{id}
func (h *FraudHandler) GetRule(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		writeError(w, http.StatusBadRequest, "Rule ID is required")
		return
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid rule ID")
		return
	}

	rule, err := h.fraudService.GetRule(r.Context(), id)
	if err != nil {
		if err == fraud.ErrRuleNotFound {
			writeError(w, http.StatusNotFound, "Rule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to get rule: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, rule)
}

// Helper functions
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

