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

// Request body ceilings for the transaction mutations. Shared by the admin
// (Bearer) and SPA (session) surfaces so the two cannot drift apart.
const (
	importTransactionsBodyLimit = 2 << 20
	updateTransactionBodyLimit  = 1 << 20
)

// transactionMutationPaths names the write paths one surface exposes. The
// admin surface mounts relative paths inside chi's /api/admin subrouter; the
// SPA surface mounts absolute paths on the browser-write group. Only the paths
// differ - limits, decode errors, service calls and error mapping are one
// implementation.
type transactionMutationPaths struct {
	importPath string
	seqPath    string // carries the {seq} URL param; used for both update and delete
}

func registerAdminTransactionRoutes(r chi.Router, dbService adminsvc.Service) {
	registerTransactionMutationRoutes(r, dbService, transactionMutationPaths{
		importPath: "/import-transactions",
		seqPath:    "/transactions/{seq}",
	})
}

func registerTransactionMutationRoutes(r chi.Router, service adminsvc.Service, paths transactionMutationPaths) {
	r.Post(paths.importPath, func(w http.ResponseWriter, req *http.Request) {
		req.Body = http.MaxBytesReader(w, req.Body, importTransactionsBodyLimit)
		var body importTransactionsRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		result, err := service.ImportTransactions(req.Context(), body.Transactions)
		writeAdminTransactionResult(w, req, result, err)
	})

	r.Put(paths.seqPath, func(w http.ResponseWriter, req *http.Request) {
		seq, err := strconv.Atoi(chi.URLParam(req, "seq"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "seq required")
			return
		}
		req.Body = http.MaxBytesReader(w, req.Body, updateTransactionBodyLimit)
		var body adminsvc.UpdateTransaction
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		result, err := service.UpdateTransaction(req.Context(), seq, body)
		writeAdminTransactionResult(w, req, result, err)
	})

	r.Delete(paths.seqPath, func(w http.ResponseWriter, req *http.Request) {
		seq, err := strconv.Atoi(chi.URLParam(req, "seq"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "seq required")
			return
		}
		result, err := service.DeleteTransaction(req.Context(), seq)
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
