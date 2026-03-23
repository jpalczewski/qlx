package webutil

import (
	"errors"
	"net/http"

	"github.com/erxyi/qlx/internal/shared/validate"
	"github.com/erxyi/qlx/internal/store"
)

var statusMap = map[error]int{
	store.ErrContainerNotFound:     http.StatusNotFound,
	store.ErrItemNotFound:          http.StatusNotFound,
	store.ErrTagNotFound:           http.StatusNotFound,
	store.ErrPrinterNotFound:       http.StatusNotFound,
	store.ErrContainerHasChildren:  http.StatusConflict,
	store.ErrContainerHasItems:     http.StatusConflict,
	store.ErrTagHasChildren:        http.StatusConflict,
	store.ErrCycleDetected:         http.StatusBadRequest,
	store.ErrInvalidParent:         http.StatusBadRequest,
	store.ErrInvalidContainer:      http.StatusBadRequest,
	validate.ErrNameRequired:       http.StatusBadRequest,
	validate.ErrNameTooLong:        http.StatusBadRequest,
	validate.ErrDescriptionTooLong: http.StatusBadRequest,
	validate.ErrInvalidCharacters:  http.StatusBadRequest,
}

// StoreHTTPStatus maps a store error to an HTTP status code.
func StoreHTTPStatus(err error) int {
	for sentinel, code := range statusMap {
		if errors.Is(err, sentinel) {
			return code
		}
	}
	return http.StatusBadRequest
}

// WriteStoreErrorJSON writes a JSON error response with the mapped status code.
func WriteStoreErrorJSON(w http.ResponseWriter, err error) {
	JSON(w, StoreHTTPStatus(err), map[string]string{"error": err.Error()})
}
