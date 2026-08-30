package dataengine

import "testing"

func TestTypedExprMatchesProjectionCasts(t *testing.T) {
	cases := []struct {
		fieldType string
		wantSub   string
	}{
		{"number", "NULLIF((r.data ->> 'Amount'), '')::numeric"},
		{"currency", "NULLIF((r.data ->> 'Amount'), '')::numeric"},
		{"integer", "NULLIF((r.data ->> 'Amount'), '')::bigint"},
		{"boolean", "NULLIF((r.data ->> 'Amount'), '')::boolean"},
		{"date", "NULLIF((r.data ->> 'Amount'), '')::date"},
		{"datetime", "NULLIF((r.data ->> 'Amount'), '')::timestamptz"},
		{"text", "(r.data ->> 'Amount')"},
		{"email", "(r.data ->> 'Amount')"},
	}
	for _, tc := range cases {
		got := typedExpr("r", "Amount", tc.fieldType)
		if got != tc.wantSub {
			t.Errorf("typedExpr(%q)=%q want %q", tc.fieldType, got, tc.wantSub)
		}
		cast := castTypeForFieldType(tc.fieldType)
		switch cast {
		case "numeric", "bigint", "boolean", "timestamptz", "date":
			if got != "NULLIF((r.data ->> 'Amount'), '')::"+cast {
				t.Errorf("projection cast %q not mirrored in typedExpr: %q", cast, got)
			}
		}
	}
}
