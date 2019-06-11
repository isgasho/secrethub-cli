package secrethub

import (
	"errors"
	"testing"

	"github.com/secrethub/secrethub-cli/internals/cli/validation"
	"github.com/secrethub/secrethub-cli/internals/secrethub/tpl"
	generictpl "github.com/secrethub/secrethub-cli/internals/tpl"

	"github.com/secrethub/secrethub-go/internals/assert"
)

func TestParseEnv(t *testing.T) {
	cases := map[string]struct {
		raw      string
		expected map[string]string
		err      error
	}{
		"success": {
			raw: "foo=bar\nbaz={{path/to/secret}}",
			expected: map[string]string{
				"foo": "bar",
				"baz": "{{path/to/secret}}",
			},
		},
		"= sign in value": {
			raw: "foo=foo=bar",
			expected: map[string]string{
				"foo": "foo=bar",
			},
		},
		"inject not closed": {
			raw: "foo={{path/to/secret",
			err: ErrTemplate(1, generictpl.ErrTagNotClosed("}}")),
		},
		"invalid key": {
			raw: "FOO\000=bar",
			err: ErrTemplate(1, validation.ErrInvalidEnvarName("FOO\000")),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			actual, err := parseEnv(tc.raw)

			expected := map[string]tpl.VarTemplate{}
			for k, v := range tc.expected {
				template, err := tpl.NewV2Parser().Parse(v)
				assert.OK(t, err)
				expected[k] = template
			}

			assert.Equal(t, actual, expected)
			assert.Equal(t, err, tc.err)
		})
	}
}

func TestParseYML(t *testing.T) {
	cases := map[string]struct {
		raw      string
		expected map[string]string
		err      error
	}{
		"success": {
			raw: "foo: bar\nbaz: ${path/to/secret}",
			expected: map[string]string{
				"foo": "bar",
				"baz": "${path/to/secret}",
			},
		},
		"= in value": {
			raw: "foo: foo=bar\nbar: baz",
			expected: map[string]string{
				"foo": "foo=bar",
				"bar": "baz",
			},
		},
		"inject not closed": {
			raw: "foo: ${path/to/secret",
			err: generictpl.ErrTagNotClosed("}"),
		},
		"nested yml": {
			raw: "ROOT:\n\tSUB\n\t\tNAME: val1",
			err: errors.New("yaml: line 2: found character that cannot start any token"),
		},
		"invalid key yml": {
			raw: "FOO=: bar",
			err: validation.ErrInvalidEnvarName("FOO="),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			actual, err := parseYML(tc.raw)

			expected := map[string]tpl.SecretTemplate{}
			for k, v := range tc.expected {
				template, err := tpl.NewV1Parser().Parse(v)
				assert.OK(t, err)
				secretTemplate, err := template.InjectVars(map[string]string{})
				assert.OK(t, err)
				expected[k] = secretTemplate
			}

			assert.Equal(t, actual, ymlTemplate{vars: expected})
			assert.Equal(t, err, tc.err)
		})
	}
}

func TestNewEnv(t *testing.T) {
	cases := map[string]struct {
		raw          string
		replacements map[string]string
		templateVars map[string]string
		expected     map[string]string
		err          error
	}{
		"success": {
			raw: "foo=bar\nbaz={{path/to/secret}}",
			replacements: map[string]string{
				"path/to/secret": "val",
			},
			expected: map[string]string{
				"foo": "bar",
				"baz": "val",
			},
		},
		"success with vars": {
			raw: "foo=bar\nbaz={{${app}/db/pass}}",
			replacements: map[string]string{
				"company/application/db/pass": "secret",
			},
			templateVars: map[string]string{
				"app": "company/application",
			},
			expected: map[string]string{
				"foo": "bar",
				"baz": "secret",
			},
		},
		"success yml": {
			raw: "foo: bar\nbaz: ${path/to/secret}",
			replacements: map[string]string{
				"path/to/secret": "val",
			},
			expected: map[string]string{
				"foo": "bar",
				"baz": "val",
			},
		},
		"yml error": {
			raw: "foo: ${path/to/secret",
			err: ErrTemplate(1, errors.New("template is not formatted as key=value pairs")),
		},
		"env error": {
			raw: "foo={{path/to/secret",
			err: ErrTemplate(1, generictpl.ErrTagNotClosed("}}")),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			env, err := NewEnv(tc.raw, tc.templateVars)
			if err != nil {
				assert.Equal(t, err, tc.err)
			} else {
				actual, err := env.Env(tc.replacements)
				assert.Equal(t, err, tc.err)

				assert.Equal(t, actual, tc.expected)
			}
		})
	}
}
