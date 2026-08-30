package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAgentConversationCRUD(t *testing.T) {
	_, pool, srv := setupPlaybookTest(t)
	const convTitle = "Run coach thread"
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM agent_conversation_messages WHERE conversation_id IN (SELECT id FROM agent_conversations WHERE title=$1)`, convTitle)
		_, _ = pool.Exec(context.Background(), `DELETE FROM agent_conversations WHERE title=$1`, convTitle)
	})

	createBody, _ := json.Marshal(map[string]any{
		"mode":            "run",
		"title":           convTitle,
		"playbookApiName": "RunCoach",
	})
	reqCreate := httptest.NewRequest(http.MethodPost, "/client/v1/agents/conversations", bytes.NewReader(createBody))
	reqCreate.Header.Set("Authorization", "Bearer admin")
	reqCreate.Header.Set("Content-Type", "application/json")
	recCreate := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", recCreate.Code, recCreate.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(recCreate.Body.Bytes(), &created)
	convID, _ := created["id"].(string)
	if convID == "" {
		t.Fatal("missing conversation id")
	}

	msgBody, _ := json.Marshal(map[string]any{
		"messages": []map[string]any{
			{"role": "human", "body": "Summarize these accounts", "parts": []map[string]any{
				{"type": "contextExcerpts", "excerpts": []map[string]any{{"label": "5 accounts", "text": "a|b"}}},
			}},
			{"role": "agent", "body": "Here is a summary."},
		},
	})
	reqMsg := httptest.NewRequest(http.MethodPost, "/client/v1/agents/conversations/"+convID+"/messages", bytes.NewReader(msgBody))
	reqMsg.Header.Set("Authorization", "Bearer admin")
	reqMsg.Header.Set("Content-Type", "application/json")
	recMsg := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recMsg, reqMsg)
	if recMsg.Code != http.StatusOK {
		t.Fatalf("append: %d %s", recMsg.Code, recMsg.Body.String())
	}

	reqGet := httptest.NewRequest(http.MethodGet, "/client/v1/agents/conversations/"+convID, nil)
	reqGet.Header.Set("Authorization", "Bearer admin")
	recGet := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("get: %d %s", recGet.Code, recGet.Body.String())
	}
	var loaded map[string]any
	_ = json.Unmarshal(recGet.Body.Bytes(), &loaded)
	messages, _ := loaded["messages"].([]any)
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %v", loaded["messages"])
	}
}

func TestPrincipalPreferenceCRUD(t *testing.T) {
	_, pool, srv := setupPlaybookTest(t)
	const kind = "ide.settings"
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM principal_preferences WHERE kind=$1`, kind)
	})

	doc := map[string]any{"theme": "dark", "composerExpanded": true}
	body, _ := json.Marshal(doc)
	reqPut := httptest.NewRequest(http.MethodPut, "/client/v1/preferences/"+kind, bytes.NewReader(body))
	reqPut.Header.Set("Authorization", "Bearer admin")
	reqPut.Header.Set("Content-Type", "application/json")
	recPut := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recPut, reqPut)
	if recPut.Code != http.StatusOK {
		t.Fatalf("put: %d %s", recPut.Code, recPut.Body.String())
	}

	reqGet := httptest.NewRequest(http.MethodGet, "/client/v1/preferences/"+kind, nil)
	reqGet.Header.Set("Authorization", "Bearer admin")
	recGet := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("get: %d %s", recGet.Code, recGet.Body.String())
	}

	reqDel := httptest.NewRequest(http.MethodDelete, "/client/v1/preferences/"+kind, nil)
	reqDel.Header.Set("Authorization", "Bearer admin")
	recDel := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recDel, reqDel)
	if recDel.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", recDel.Code, recDel.Body.String())
	}
}
