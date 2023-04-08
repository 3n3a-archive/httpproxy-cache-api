package utils

import (
	"encoding/json"
	"net/http"
)

type H map[string]interface{}

// JSON marshals the value as JSON and writes it to the response writer.
// with status code (compat with .JSON)
//
// JSON(w, value)
// JSON(w, value, statusCode)
func JSON(w http.ResponseWriter, value interface{}, optional ...any) error {
	if value == nil {
		return nil
	}

	statusCode := 200

	if len(optional) >= 1 {
		statusCode = optional[0].(int)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode) // needs to be called after setting headers !!

	enc := json.NewEncoder(w)
	if err := enc.Encode(value); err != nil {
		return err
	}

	return nil
}