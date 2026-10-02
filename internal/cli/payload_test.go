package cli

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestParseValueRequiresCompleteJSON(t *testing.T) {
	for _, test := range []struct {
		value string
		want  any
	}{
		{value: "123 Main Street", want: "123 Main Street"},
		{value: "00123", want: "00123"},
		{value: "true story", want: "true story"},
		{value: "null island", want: "null island"},
		{value: "1 2", want: "1 2"},
		{value: `{"id":1} trailing`, want: `{"id":1} trailing`},
		{value: "  text  ", want: "text"},
		{value: "2026-06-03T09:00:00Z", want: "2026-06-03T09:00:00Z"},
		{value: "123", want: json.Number("123")},
		{value: "9007199254740993", want: json.Number("9007199254740993")},
		{value: " true ", want: true},
		{value: "null"},
		{value: `"00123"`, want: "00123"},
		{value: `[1,false]`, want: []any{json.Number("1"), false}},
		{value: `{"id":1}`, want: map[string]any{"id": json.Number("1")}},
	} {
		t.Run(test.value, func(t *testing.T) {
			if got := parseValue(test.value); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("parseValue() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestAuthTokenRejectsInvalidArgumentsWithoutRevealingToken(t *testing.T) {
	t.Setenv("TRIPSY_AUTH_BACKEND", "file")
	for _, args := range [][]string{
		{"delete"}, {"sett"}, {"--unexpected"}, {"set", "first", "second"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			command := append([]string{"--token", "test-secret", "--config-dir", t.TempDir(), "auth", "token"}, args...)
			if code := Run(command, nil, &stdout, &stderr); code != 2 {
				t.Fatalf("code = %d, want usage error", code)
			}
			if stdout.Len() != 0 || strings.Contains(stderr.String(), "test-secret") {
				t.Fatal("invalid token arguments must not reveal the configured token")
			}
		})
	}
}
