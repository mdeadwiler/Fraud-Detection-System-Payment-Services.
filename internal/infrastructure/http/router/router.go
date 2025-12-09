package router

import (
	"net/http"

	"fraud-detecction-system/internal/interfaces/http/handler"
)

// Router holds all HTTP handlers
type Router struct {
	mux           *http.ServeMux
	fraudHandler  *handler.FraudHandler
	healthHandler *handler.HealthHandler
}

// NewRouter creates a new router with all routes configured
func NewRouter(
	fraudHandler *handler.FraudHandler,
	healthHandler *handler.HealthHandler,
) *Router {
	r := &Router{
		mux:           http.NewServeMux(),
		fraudHandler:  fraudHandler,
		healthHandler: healthHandler,
	}
	r.setupRoutes()
	return r
}

func (r *Router) setupRoutes() {
	// Health endpoints
	r.mux.HandleFunc("GET /health", r.healthHandler.Health)
	r.mux.HandleFunc("GET /ready", r.healthHandler.Ready)
	r.mux.HandleFunc("GET /live", r.healthHandler.Live)

	// Fraud analysis endpoints
	r.mux.HandleFunc("POST /api/v1/fraud/analyze", r.fraudHandler.AnalyzeTransaction)
	r.mux.HandleFunc("POST /api/v1/fraud/analyze/batch", r.fraudHandler.BatchAnalyze)

	// Fraud decisions
	r.mux.HandleFunc("GET /api/v1/fraud/decisions/{id}", r.fraudHandler.GetDecision)
	r.mux.HandleFunc("GET /api/v1/fraud/transactions/{id}/decision", r.fraudHandler.GetDecisionByTransaction)

	// User risk profiles
	r.mux.HandleFunc("GET /api/v1/fraud/users/{id}/risk", r.fraudHandler.GetUserRiskProfile)

	// Fraud cases
	r.mux.HandleFunc("GET /api/v1/fraud/cases", r.fraudHandler.ListCases)
	r.mux.HandleFunc("GET /api/v1/fraud/cases/{id}", r.fraudHandler.GetCase)
	r.mux.HandleFunc("PUT /api/v1/fraud/cases/{id}", r.fraudHandler.UpdateCase)

	// Fraud rules
	r.mux.HandleFunc("GET /api/v1/fraud/rules", r.fraudHandler.ListRules)
	r.mux.HandleFunc("POST /api/v1/fraud/rules", r.fraudHandler.CreateRule)
	r.mux.HandleFunc("GET /api/v1/fraud/rules/{id}", r.fraudHandler.GetRule)
}

// ServeHTTP implements http.Handler
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if req.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	r.mux.ServeHTTP(w, req)
}

// Handler returns the http.Handler
func (r *Router) Handler() http.Handler {
	return r
}

