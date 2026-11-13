package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"regexp"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var safeFlagName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type EvaluationResponse struct {
	FlagName string `json:"flag_name"`
	UserID   string `json:"user_id"`
	Result   bool   `json:"result"`
}

func (a *App) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (a *App) evaluationHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 1. Parsear os query parameters
	userID := r.URL.Query().Get("user_id")
	flagName := r.URL.Query().Get("flag_name")

	if userID == "" || flagName == "" {
		http.Error(w, `{"error": "user_id e flag_name são obrigatórios"}`, http.StatusBadRequest)
		return
	}
	if !safeFlagName.MatchString(flagName) {
		http.Error(w, `{"error": "flag_name contém caracteres inválidos"}`, http.StatusBadRequest)
		return
	}

	// Span SERVER criado pelo otelhttp (main.go); enriquecido com dados de negócio.
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("feature_flag.key", flagName),
		attribute.String("enduser.id", userID),
	)

	// 2. Obter a decisão (lógica de cache/serviço está em evaluator.go)
	result, err := a.getDecision(ctx, userID, flagName)
	if err != nil {
		// Se o erro for "não encontrado", retornamos 'false' (comportamento seguro)
		if _, ok := err.(*NotFoundError); ok {
			result = false
		} else {
			// Outros erros (serviços offline, etc)
			span.RecordError(err)
			span.SetStatus(codes.Error, "falha ao avaliar a flag")
			log.Printf("Erro ao avaliar flag '%s' (trace_id=%s): %v", flagName, span.SpanContext().TraceID(), err)
			http.Error(w, `{"error": "Erro interno ao avaliar a flag"}`, http.StatusBadGateway)
			return
		}
	}

	span.SetAttributes(attribute.Bool("feature_flag.result", result))

	// 3. Enviar evento para SQS (assincronamente)
	// Isso não bloqueia a resposta para o cliente. WithoutCancel mantém o trace
	// (o analytics-service continua o mesmo trace) sem herdar o cancelamento
	// do ctx, que termina quando a resposta é enviada.
	go a.sendEvaluationEvent(context.WithoutCancel(ctx), userID, flagName, result)

	// 4. Retornar a resposta
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(EvaluationResponse{
		FlagName: flagName,
		UserID:   userID,
		Result:   result,
	})
}
