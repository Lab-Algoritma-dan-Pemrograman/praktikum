package grading

import (
	"bytes"
	"encoding/json"
	"io"
	"unicode/utf8"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
)

// DecodeResult provides a fail-closed decoder for the suggestion DTO. It rejects
// partial JSON, omitted/null fields, unknown or duplicate keys, fractional scores
// and oversized wire payloads. It performs no I/O and selects no AI provider.
// Provenance fields still must be set by a trusted adapter, not by a model.
func DecodeResult(req entity.GradeRequest, payload []byte) (entity.GradeResult, error) {
	fallback := manualFallback("unverified", false, "invalid_result")
	if err := ValidateRequest(req); err != nil {
		return manualFallback("unverified", false, "invalid_request"), err
	}
	if len(payload) > MaxResultBytes || !utf8.Valid(payload) {
		return fallback, ErrInvalidResult
	}
	fields, err := strictObject(payload, "status", "suggestions", "provider_version", "simulated", "error_code")
	if err != nil {
		return fallback, ErrInvalidResult
	}
	var suggestions []json.RawMessage
	if err := json.Unmarshal(fields["suggestions"], &suggestions); err != nil || len(suggestions) > MaxQuestions {
		return fallback, ErrInvalidResult
	}
	for _, suggestion := range suggestions {
		if _, err := strictObject(suggestion, "question_id", "score", "feedback"); err != nil {
			return fallback, ErrInvalidResult
		}
	}
	var result entity.GradeResult
	if err := json.Unmarshal(payload, &result); err != nil {
		return fallback, ErrInvalidResult
	}
	return AcceptResult(req, result)
}

func strictObject(payload []byte, required ...string) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, ErrInvalidResult
	}
	fields := make(map[string]json.RawMessage, len(required))
	allowed := make(map[string]bool, len(required))
	for _, field := range required {
		allowed[field] = true
	}
	for decoder.More() {
		token, err := decoder.Token()
		name, ok := token.(string)
		if err != nil || !ok || !allowed[name] || fields[name] != nil {
			return nil, ErrInvalidResult
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, ErrInvalidResult
		}
		fields[name] = value
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') || len(fields) != len(required) {
		return nil, ErrInvalidResult
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, ErrInvalidResult
	}
	return fields, nil
}
