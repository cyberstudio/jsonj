package jsonj

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcess(t *testing.T) {
	tests := []struct {
		name  string
		rules []Rule
		input string
		want  string
	}{
		{
			name: "insert",
			rules: []Rule{
				NewInsertRule("mark", "key", generateMeta),
			},
			input: `{"mark": "value"}`,
			want:  `{"key": "value", "meta": {"length": 5}}`,
		},
		{
			name: "replace",
			rules: []Rule{
				NewReplaceRule("mark", "key", generateMeta),
			},
			input: `{"mark": "value"}`,
			want:  `{"key": {"meta": {"length": 5}}}`,
		},
		{
			name: "delete",
			rules: []Rule{
				NewDeleteRule("mark"),
			},
			input: `{"mark": "value"}`,
			want:  `{}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := ProcessParams{
				Passes: []Pass{{
					RuleSet: NewRuleSet(tt.rules...),
					Repeats: 1,
				}},
			}

			t.Run("single mark", func(t *testing.T) {
				got, err := Process(context.Background(), []byte(tt.input), params)
				require.NoError(t, err)
				assert.JSONEq(t, tt.want, string(got))
			})

			t.Run("mark at the start", func(t *testing.T) {
				input := suffixedObject(t, tt.input) // {"mark":..., suffix}
				got, err := Process(context.Background(), []byte(input), params)
				require.NoError(t, err)
				assert.JSONEq(t, suffixedObject(t, tt.want), string(got))
			})

			t.Run("mark at the end", func(t *testing.T) {
				input := prefixedObject(t, tt.input) // {prefix, "mark":...}
				got, err := Process(context.Background(), []byte(input), params)
				require.NoError(t, err)
				assert.JSONEq(t, prefixedObject(t, tt.want), string(got))
			})
		})
	}
}

func suffixedObject(t *testing.T, val string) string {
	t.Helper()
	if val[0] != '{' || val[len(val)-1] != '}' {
		t.Fatal("{...} object expected")
	}
	const suffix = `,"type": "File"}`
	if val == "{}" { // '{}'
		return "{" + strings.TrimPrefix(suffix, ",")
	}
	return val[0:len(val)-1] + suffix
}

func prefixedObject(t *testing.T, val string) string {
	t.Helper()
	if val[0] != '{' || val[len(val)-1] != '}' {
		t.Fatal("{...} object expected")
	}
	const prefix = `{"type": "File",`
	if len(val) == 2 { // '{}'
		return strings.TrimSuffix(prefix, ",") + "}"
	}
	return prefix + val[1:]
}

// generateMeta is example of batch values handling
func generateMeta(_ context.Context, iterator FragmentIterator, _ interface{}) ([]interface{}, error) {
	type Output struct {
		Meta struct {
			Length int `json:"length"`
		} `json:"meta"`
	}

	var entities []interface{}
	for iterator.Next() {
		var (
			output Output
			value  string
		)
		if err := iterator.BindParams(&value); err != nil {
			panic(err)
		}
		output.Meta.Length = len(value)
		entities = append(entities, output)
	}
	return entities, nil
}

func Test_findJSONFragmentEnd(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want int
	}{
		{
			name: "string with escaped symbols",
			data: []byte("\n\r\t  \"white spaces\\\" \\n\\b\\f\\r\\t [{}] \"    ,"),
			want: len("\n\r\t  \"white spaces\\\" \\n\\b\\f\\r\\t [{}] \""),
		},
		{
			name: "array",
			data: []byte(` [ 1.2, 2.3, "string" ] `),
			want: len(` [ 1.2, 2.3, "string" ]`),
		},
		{
			name: "nested arrays",
			data: []byte(` [ 1.2, 2.3, "string", { "key": [ 1, "str", null, {} ] } ] `),
			want: len(` [ 1.2, 2.3, "string", { "key": [ 1, "str", null, {} ] } ]`),
		},
		{
			name: "brackets inside string",
			data: []byte(` [ "string]", "[string]" ]`),
			want: len(` [ "string]", "[string]" ]`),
		},
		{
			name: "object",
			data: []byte(` { "key1": 1.2 }, "next" : {}`),
			want: len(` { "key1": 1.2 }`),
		},
		{
			name: "nested objects",
			data: []byte(` { "key1": 1.2, "key2": { "key": [ 1, "str", null, {} ] } }, {"next"}`),
			want: len(` { "key1": 1.2, "key2": { "key": [ 1, "str", null, {} ] } }`),
		},
		{
			name: "brackets inside object",
			data: []byte(` { "key": "}value{}" }`),
			want: len(` { "key": "}value{}" }`),
		},
		{
			name: "integer number",
			data: []byte(` -1234567890, "next": {}`),
			want: len(` -1234567890`),
		},
		{
			name: "float number",
			data: []byte(` 1234.567890, "next": {}`),
			want: len(` 1234.567890`),
		},
		{
			name: "exponent number",
			data: []byte(` -9e10, "next": {}`),
			want: len(` -9e10`),
		},
		{
			name: "null",
			data: []byte(` null, "next": {}`),
			want: len(` null`),
		},
		{
			name: "true",
			data: []byte(` true, "next": {}`),
			want: len(` true`),
		},
		{
			name: "false",
			data: []byte(` false, "next": {}`),
			want: len(` false`),
		},
		{
			name: "object with spaces",
			data: []byte(` [ { "object_uuid" : "37f5e2bf-979b-47b4-baca-7362fd1e4a4d" } ]`),
			want: len(` [ { "object_uuid" : "37f5e2bf-979b-47b4-baca-7362fd1e4a4d" } ]`),
		},
		{
			name: "object w/out spaces",
			data: []byte(`[{"object_uuid":"37f5e2bf-979b-47b4-baca-7362fd1e4a4d"}]`),
			want: len(`[{"object_uuid":"37f5e2bf-979b-47b4-baca-7362fd1e4a4d"}]`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findJSONFragmentEnd(tt.data)
			assert.Equal(t, tt.want, got)
		})
	}
}
