package dataengine

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestUpsertLockKeyIsValidPostgresText(t *testing.T) {
	key := upsertLockKey("Account", "ERP_Id__c", "ABC-1")
	if strings.ContainsRune(key, 0) {
		t.Fatal("advisory lock key must not contain NUL; hashtextextended takes text")
	}
	if !utf8.ValidString(key) {
		t.Fatal("lock key must be valid UTF-8")
	}
	if !strings.Contains(key, "\x1f") {
		t.Fatal("expected unit-separator delimiters")
	}
}
