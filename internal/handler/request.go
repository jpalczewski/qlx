package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

// BindRequest populates req from form values first, then overrides with JSON body
// if Content-Type is application/json. Uses `form` struct tags for form field mapping,
// falling back to `json` tags.
func BindRequest(r *http.Request, req any) error {
	_ = r.ParseForm()

	v := reflect.ValueOf(req).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		formTag := field.Tag.Get("form")
		if formTag == "" {
			formTag = field.Tag.Get("json")
		}
		if formTag == "" || formTag == "-" {
			continue
		}
		formTag = strings.Split(formTag, ",")[0]

		if val := r.FormValue(formTag); val != "" {
			fv := v.Field(i)
			switch fv.Kind() {
			case reflect.String:
				fv.SetString(val)
			case reflect.Int, reflect.Int64:
				if n, err := strconv.Atoi(val); err == nil {
					fv.SetInt(int64(n))
				}
			}
		}
	}

	if isJSONBody(r) {
		if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}
	}

	return nil
}

// isJSONBody checks if the request body is JSON.
func isJSONBody(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Content-Type"), "application/json")
}

// --- Shared request types ---

// CreateContainerRequest is the input for container creation.
type CreateContainerRequest struct {
	ParentID    string `json:"parent_id" form:"parent_id"`
	Name        string `json:"name" form:"name"`
	Description string `json:"description" form:"description"`
	Color       string `json:"color" form:"color"`
	Icon        string `json:"icon" form:"icon"`
}

// UpdateContainerRequest is the input for container updates.
type UpdateContainerRequest struct {
	Name        string `json:"name" form:"name"`
	Description string `json:"description" form:"description"`
	Color       string `json:"color" form:"color"`
	Icon        string `json:"icon" form:"icon"`
}

// CreateItemRequest is the input for item creation.
type CreateItemRequest struct {
	ContainerID string `json:"container_id" form:"container_id"`
	Name        string `json:"name" form:"name"`
	Description string `json:"description" form:"description"`
	Quantity    int    `json:"quantity" form:"quantity"`
	Color       string `json:"color" form:"color"`
	Icon        string `json:"icon" form:"icon"`
}

// UpdateItemRequest is the input for item updates.
type UpdateItemRequest struct {
	Name        string `json:"name" form:"name"`
	Description string `json:"description" form:"description"`
	Quantity    int    `json:"quantity" form:"quantity"`
	Color       string `json:"color" form:"color"`
	Icon        string `json:"icon" form:"icon"`
}

// MoveRequest is the input for move operations.
type MoveRequest struct {
	ParentID    string `json:"parent_id" form:"parent_id"`
	ContainerID string `json:"container_id" form:"container_id"`
}

// UpsertTagRequest is the input for tag create/update.
type UpsertTagRequest struct {
	Name     string `json:"name" form:"name"`
	ParentID string `json:"parent_id" form:"parent_id"`
	Color    string `json:"color" form:"color"`
	Icon     string `json:"icon" form:"icon"`
}

// AddPrinterRequest is the input for adding a printer.
type AddPrinterRequest struct {
	Name      string `json:"name" form:"name"`
	Encoder   string `json:"encoder" form:"encoder"`
	Model     string `json:"model" form:"model"`
	Transport string `json:"transport" form:"transport"`
	Address   string `json:"address" form:"address"`
}

// PrintRequest is the input for print operations.
type PrintRequest struct {
	PrinterID string `json:"printer_id"`
	Template  string `json:"template"`
	PrintDate bool   `json:"print_date"`
}

// ContainerPrintRequest is the input for container label printing.
type ContainerPrintRequest struct {
	PrinterID    string   `json:"printer_id"`
	Templates    []string `json:"templates"`
	PrintDate    bool     `json:"print_date"`
	ShowChildren bool     `json:"show_children"`
}

// TagAssignRequest is the input for assigning/removing tags.
type TagAssignRequest struct {
	TagID string `json:"tag_id" form:"tag_id"`
}
