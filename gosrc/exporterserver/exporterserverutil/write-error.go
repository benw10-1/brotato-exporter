package exporterserverutil

import (
	"log"
	"net/http"
)

// WriteError
func WriteError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}

	log.Printf("exporterserverutil.WriteError: error serving req - %v", err)

	if re, ok := err.(*ResponseError); ok {
		http.Error(w, re.Message(), re.StatusCode())
		return
	}

	http.Error(w, "Error serving request", http.StatusInternalServerError)
}
