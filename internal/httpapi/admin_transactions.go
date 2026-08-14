package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	adminsvc "github.com/DeliciousBuding/fund-dashboard/internal/service/admin"
	"github.com/go-chi/chi/v5"
)

type importTransactionsRequest struct {
	Transactions []adminsvc.ImportTransaction `json:"transactions"`
}

func registerAdminTransactionRoutes(r chi.Router, dbService adminsvc.Service) {
	r.Post("/import-transactions", func(w http.ResponseWriter, req *http.Request) {
		req.Body = http.MaxBytesReader(w, req.Body, 2<<20)
		var body importTransactionsRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		result, err := dbService.ImportTransactions(req.Context(), body.Transactions)
		writeAdminTransactionResult(w, req, result, err)
	})

	r.Put("/transactions/{seq}", func(w http.ResponseWriter, req *http.Request) {
		seq, err := strconv.Atoi(chi.URLParam(req, "seq"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "seq required")
			return
		}
		req.Body = http.MaxBytesReader(w, req.Body, 1<<20)
		var body adminsvc.UpdateTransaction
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		result, err := dbService.UpdateTransaction(req.Context(), seq, body)
		writeAdminTransactionResult(w, req, result, err)
	})

	r.Delete("/transactions/{seq}", func(w http.ResponseWriter, req *http.Request) {
		seq, err := strconv.Atoi(chi.URLParam(req, "seq"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "seq required")
			return
		}
		result, err := dbService.DeleteTransaction(req.Context(), seq)
		writeAdminTransactionResult(w, req, result, err)
	})
}

func writeAdminTransactionResult(w http.ResponseWriter, r *http.Request, result any, err error) {
	if err == nil {
		WriteJSON(w, http.StatusOK, result)
		return
	}
	if errors.Is(err, adminsvc.ErrInvalidInput) {
		// Validation messages are intentional short domain text.
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, adminsvc.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeSafeError(w, r, http.StatusInternalServerError, err)
}
