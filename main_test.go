package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"io"
	"os"
	"testing"
	"time"
)

type gqlRequest struct {
	Query     string      `json:"query"`
	Variables interface{} `json:"variables,omitempty"`
}

type gqlError struct {
	Message string `json:"message"`
}

type gqlResponse struct {
	Data   map[string]json.RawMessage `json:"data"`
	Errors []gqlError                 `json:"errors"`
}

type Transaction struct {
	ID                   string `json:"id"`
	UserID               string `json:"userid"`
	CoinID               string `json:"coinid"`
	DataID               string `json:"dataid"`
	CoinUsed             float64 `json:"coinused"`
	TransactionTimestamp string `json:"transactionTimestamp"`
	ExpiryDate           string `json:"expiryDate"`
	PlatformName         string `json:"platformName"`
}

// internal/e2e/graphql_smoke_test.go
func gqlPost(t *testing.T, url string, query string, vars interface{}) gqlResponse {
	t.Helper()

	body, _ := json.Marshal(gqlRequest{
		Query:     query,
		Variables: vars,
	})

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")     // force JSON response
	req.Header.Set("X-User-ID", "smoke-user")        // optional: hit user limiter path

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http post: %v", err)
	}
	defer res.Body.Close()

	// Read body once so we can both debug and decode.
	raw, _ := io.ReadAll(res.Body)

	if res.StatusCode != http.StatusOK {
		snippet := string(raw)
		if len(snippet) > 400 {
			snippet = snippet[:400] + "…"
		}
		t.Fatalf("non-200 status %d. body:\n%s", res.StatusCode, snippet)
	}

	var out gqlResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		snippet := string(raw)
		if len(snippet) > 400 {
			snippet = snippet[:400] + "…"
		}
		t.Fatalf("decode error: %v\nbody:\n%s", err, snippet)
	}

	if len(out.Errors) > 0 {
		t.Fatalf("graphql errors: %+v", out.Errors)
	}
	return out
}

func TestGraphQL_Add_Get_List(t *testing.T) {
	url := os.Getenv("GRAPHQL_URL")
	if url == "" {
		url = "http://localhost:8080/graphql"
	}

	// --- 1) addTransaction ---
	addMutation := `
mutation Add($input: AddTransactionInput!) {
  addTransaction(input: $input) {
    id
    userid
    coinid
    dataid
    coinused
    transactionTimestamp
    expiryDate
    platformName
  }
}`

	userID := "c1f2e3d4-5678-90ab-cdef-1234567890ab" // test UUID; replace if you enforce auth.uid()
	now := time.Now().UTC()
	vars := map[string]interface{}{
		"input": map[string]interface{}{
			"coinid":               "BTC",
			"userid":               userID,
			"dataid":               fmt.Sprintf("order-%d", now.UnixNano()),
			"coinused":             0.75,
			"transactionTimestamp": now.Format(time.RFC3339),
			"expiryDate":           now.Add(24 * time.Hour).Format(time.RFC3339),
			"platformName":         "e2e-test",
		},
	}

	addResp := gqlPost(t, url, addMutation, vars)
	var added Transaction
	if err := json.Unmarshal(addResp.Data["addTransaction"], &added); err != nil {
		t.Fatalf("unmarshal addTransaction: %v", err)
	}
	if added.ID == "" {
		t.Fatalf("expected non-empty id")
	}
	if added.UserID != userID || added.CoinID != "BTC" || added.PlatformName != "e2e-test" {
		t.Fatalf("unexpected addTransaction result: %+v", added)
	}

	// --- 2) getTransactionByID ---
	getByIDQuery := `
query One($id: String!) {
  getTransactionByID(id: $id) {
    id
    userid
    coinid
    dataid
    coinused
    transactionTimestamp
    expiryDate
    platformName
  }
}`
	getResp := gqlPost(t, url, getByIDQuery, map[string]interface{}{"id": added.ID})
	var got Transaction
	if err := json.Unmarshal(getResp.Data["getTransactionByID"], &got); err != nil {
		t.Fatalf("unmarshal getTransactionByID: %v", err)
	}
	if got.ID != added.ID {
		t.Fatalf("mismatched id: added=%s got=%s", added.ID, got.ID)
	}

	// --- 3) getTransactions (filter by userid) ---
	listQuery := `
query Many($f: TransactionFilter) {
  getTransactions(filter: $f) {
    id
    userid
    coinid
    dataid
    coinused
    transactionTimestamp
    expiryDate
    platformName
  }
}`
	listVars := map[string]interface{}{
		"f": map[string]interface{}{
			"userid": userID,
			"limit":  10,
		},
	}
	listResp := gqlPost(t, url, listQuery, listVars)
	var items []Transaction
	if err := json.Unmarshal(listResp.Data["getTransactions"], &items); err != nil {
		t.Fatalf("unmarshal getTransactions: %v", err)
	}
	if len(items) == 0 {
		t.Fatalf("expected at least one transaction for user %s", userID)
	}
	// Optionally ensure the added id is present in the list
	found := false
	for _, it := range items {
		if it.ID == added.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("added transaction id %s not found in list", added.ID)
	}
}
