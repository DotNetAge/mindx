package httpclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	client := NewClient("https://api.example.com", 30*time.Second)
	assert.NotNil(t, client)
	assert.Equal(t, "https://api.example.com", client.baseURL)

	// Test with zero timeout (should use default)
	client2 := NewClient("https://api.example.com", 0)
	assert.NotNil(t, client2)
}

func TestNewClientWithTransport(t *testing.T) {
	transport := &http.Transport{
		MaxIdleConns: 100,
	}
	client := NewClientWithTransport("https://api.example.com", 30*time.Second, transport)
	assert.NotNil(t, client)
	assert.Equal(t, "https://api.example.com", client.baseURL)
}

func TestClient_Post(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/test", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer token123", r.Header.Get("Authorization"))

		var payload map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)
		assert.Equal(t, "value1", payload["key1"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := NewClient(server.URL, 10*time.Second)

	payload := map[string]string{"key1": "value1"}
	headers := map[string]string{"Authorization": "Bearer token123"}

	resp, err := client.Post(context.Background(), "/test", payload, headers)
	require.NoError(t, err)
	assert.Contains(t, string(resp), "ok")
}

func TestClient_Post_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "bad request"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, 10*time.Second)

	payload := map[string]string{"key": "value"}
	resp, err := client.Post(context.Background(), "/test", payload, nil)

	// Should return error for 4xx status
	assert.Error(t, err)
	assert.Contains(t, string(resp), "bad request")
}

func TestClient_Post_InvalidPayload(t *testing.T) {
	client := NewClient("https://api.example.com", 10*time.Second)

	// Test with invalid payload (channel that can't be marshaled)
	invalidPayload := make(chan int)
	_, err := client.Post(context.Background(), "/test", invalidPayload, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal payload")
}

func TestClient_PostForm(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/test", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, "value1", r.FormValue("key1"))
		assert.Equal(t, "value2", r.FormValue("key2"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := NewClient(server.URL, 10*time.Second)

	formData := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	resp, err := client.PostForm(context.Background(), "/test", formData, nil)
	require.NoError(t, err)
	assert.Contains(t, string(resp), "ok")
}

func TestClient_Get(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/test", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "Bearer token123", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := NewClient(server.URL, 10*time.Second)

	headers := map[string]string{"Authorization": "Bearer token123"}

	resp, err := client.Get(context.Background(), "/test", headers)
	require.NoError(t, err)
	assert.Contains(t, string(resp), "ok")
}

func TestClient_Get_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "not found"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, 10*time.Second)

	resp, err := client.Get(context.Background(), "/test", nil)

	// Should return error for 4xx status
	assert.Error(t, err)
	assert.Contains(t, string(resp), "not found")
}

func TestClient_Do(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/test", r.URL.Path)
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "Bearer token123", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
	}))
	defer server.Close()

	client := NewClient(server.URL, 10*time.Second)

	body := []byte(`{"key": "value"}`)
	headers := map[string]string{"Authorization": "Bearer token123"}

	resp, err := client.Do(context.Background(), "PUT", "/test", body, headers)
	require.NoError(t, err)
	assert.Contains(t, string(resp), "updated")
}

func TestClient_Do_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal error"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, 10*time.Second)

	resp, err := client.Do(context.Background(), "GET", "/test", nil, nil)
	assert.Error(t, err)
	assert.Contains(t, string(resp), "internal error")
}

func TestClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, 10*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := client.Get(ctx, "/test", nil)
	assert.Error(t, err)
}

func TestClient_SetHTTPClient(t *testing.T) {
	client := NewClient("https://api.example.com", 10*time.Second)

	customClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	client.SetHTTPClient(customClient)
	assert.Equal(t, customClient, client.httpClient)
}

func TestClient_CloseIdleConnections(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, 10*time.Second)

	// Make a request first
	_, err := client.Get(context.Background(), "/test", nil)
	require.NoError(t, err)

	// Close idle connections (should not panic)
	client.CloseIdleConnections()
}
