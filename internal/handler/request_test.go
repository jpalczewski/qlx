package handler

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBindRequest_FormValues(t *testing.T) {
	var req struct {
		Name  string `json:"name" form:"name"`
		Color string `json:"color" form:"color"`
	}
	r := httptest.NewRequest("POST", "/",
		strings.NewReader("name=Test&color=red"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if err := BindRequest(r, &req); err != nil {
		t.Fatal(err)
	}
	if req.Name != "Test" {
		t.Fatalf("expected Test, got %s", req.Name)
	}
	if req.Color != "red" {
		t.Fatalf("expected red, got %s", req.Color)
	}
}

func TestBindRequest_JSONBody(t *testing.T) {
	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	r := httptest.NewRequest("POST", "/",
		strings.NewReader(`{"name":"Test","color":"blue"}`))
	r.Header.Set("Content-Type", "application/json")

	if err := BindRequest(r, &req); err != nil {
		t.Fatal(err)
	}
	if req.Name != "Test" {
		t.Fatalf("expected Test, got %s", req.Name)
	}
}

func TestBindRequest_JSONOverridesForm(t *testing.T) {
	var req struct {
		Name string `json:"name" form:"name"`
	}
	r := httptest.NewRequest("POST", "/?name=FromForm",
		strings.NewReader(`{"name":"FromJSON"}`))
	r.Header.Set("Content-Type", "application/json")

	if err := BindRequest(r, &req); err != nil {
		t.Fatal(err)
	}
	if req.Name != "FromJSON" {
		t.Fatalf("expected FromJSON, got %s", req.Name)
	}
}

func TestBindRequest_InvalidJSON(t *testing.T) {
	var req struct {
		Name string `json:"name"`
	}
	r := httptest.NewRequest("POST", "/",
		strings.NewReader(`{invalid`))
	r.Header.Set("Content-Type", "application/json")

	if err := BindRequest(r, &req); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestBindRequest_IntField(t *testing.T) {
	var req struct {
		Quantity int `json:"quantity" form:"quantity"`
	}
	r := httptest.NewRequest("POST", "/",
		strings.NewReader("quantity=5"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if err := BindRequest(r, &req); err != nil {
		t.Fatal(err)
	}
	if req.Quantity != 5 {
		t.Fatalf("expected 5, got %d", req.Quantity)
	}
}

func TestBindRequest_IntFieldInvalidValue(t *testing.T) {
	var req struct {
		Quantity int `json:"quantity" form:"quantity"`
	}
	r := httptest.NewRequest("POST", "/",
		strings.NewReader("quantity=abc"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if err := BindRequest(r, &req); err != nil {
		t.Fatal(err)
	}
	if req.Quantity != 0 {
		t.Fatalf("expected 0 for invalid int, got %d", req.Quantity)
	}
}
