package validator

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateSyntax(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		line    int
		col     int
	}{
		{"objeto válido", `{"a": 1}`, false, 0, 0},
		{"array válido", `[1, 2, 3]`, false, 0, 0},
		{"escalar válido", `"texto"`, false, 0, 0},
		{"vazio", ``, true, 1, 1},
		{"só espaços", "  \n ", true, 2, 2},
		{"vírgula sobrando", "{\n  \"a\": 1,\n}", true, 3, 1},
		{"aspas simples", `{'a': 1}`, true, 1, 2},
		{"falta fechar", `{"a": [1, 2`, true, 1, 12},
		{"dois valores", `{} {}`, true, 1, 4},
		{"lixo no final", `{} x`, true, 1, 4},
		{"acentos contam como 1 coluna", "{\"ç\": x}", true, 1, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSyntax([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Fatalf("erro = %v, quer erro = %v", err, tt.wantErr)
			}
			if err == nil {
				return
			}
			var se *SyntaxError
			if !errors.As(err, &se) {
				t.Fatalf("esperava *SyntaxError, veio %T", err)
			}
			if se.Line != tt.line || se.Column != tt.col {
				t.Errorf("posição = %d:%d, quer %d:%d (%s)", se.Line, se.Column, tt.line, tt.col, se.Msg)
			}
		})
	}
}

func TestSyntaxErrorMessageIsTranslated(t *testing.T) {
	err := ValidateSyntax([]byte(`{"a" 1}`))
	if err == nil || !strings.Contains(err.Error(), "depois do nome da chave") {
		t.Errorf("mensagem não traduzida: %v", err)
	}
}

func mustParse(t *testing.T, s string) any {
	t.Helper()
	v, err := Parse([]byte(s))
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	return v
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		opts CompareOptions
		want []string // "kind path"
	}{
		{"iguais", `{"a":1,"b":[1,2]}`, `{"b":[1,2],"a":1}`, CompareOptions{}, nil},
		{"número 1 == 1.0", `{"a":1}`, `{"a":1.0}`, CompareOptions{}, nil},
		{"valor mudou", `{"a":1}`, `{"a":2}`, CompareOptions{}, []string{"changed $.a"}},
		{"chave adicionada e removida", `{"a":1}`, `{"b":1}`, CompareOptions{},
			[]string{"removed $.a", "added $.b"}},
		{"tipo mudou", `{"a":"1"}`, `{"a":1}`, CompareOptions{}, []string{"type_changed $.a"}},
		{"aninhado e array", `{"u":{"e":["x","y"]}}`, `{"u":{"e":["x","z","w"]}}`, CompareOptions{},
			[]string{"changed $.u.e[1]", "added $.u.e[2]"}},
		{"ordem do array importa", `[1,2]`, `[2,1]`, CompareOptions{},
			[]string{"changed $[0]", "changed $[1]"}},
		{"ignora ordem do array", `[1,2,{"a":1}]`, `[{"a":1},2,1]`, CompareOptions{IgnoreArrayOrder: true}, nil},
		{"ignora ordem mas mantém índice original", `{"i":[{"id":1,"q":2},{"id":2,"q":5}]}`, `{"i":[{"id":2,"q":5},{"id":1,"q":"2"}]}`,
			CompareOptions{IgnoreArrayOrder: true}, []string{"type_changed $.i[0].q"}},
		{"ignora ordem: removido e adicionado", `[3,1,2]`, `[2,1]`, CompareOptions{IgnoreArrayOrder: true},
			[]string{"removed $[0]"}},
		{"só estrutura ignora valores", `{"a":1,"b":"x"}`, `{"a":99,"b":"y"}`, CompareOptions{StructureOnly: true}, nil},
		{"só estrutura vê tipos", `{"a":1}`, `{"a":"1"}`, CompareOptions{StructureOnly: true}, []string{"type_changed $.a"}},
		{"chave com ponto", `{"a.b":1}`, `{"a.b":2}`, CompareOptions{}, []string{`changed $["a.b"]`}},
		{"raiz de tipos diferentes", `[]`, `{}`, CompareOptions{}, []string{"type_changed $"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diffs := Compare(mustParse(t, tt.a), mustParse(t, tt.b), tt.opts)
			var got []string
			for _, d := range diffs {
				got = append(got, string(d.Kind)+" "+d.Path)
			}
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("diferenças = %v, quer %v", got, tt.want)
			}
		})
	}
}

func TestCheckRequired(t *testing.T) {
	doc := mustParse(t, `{
		"nome": "Ana",
		"email": null,
		"endereco": {"cidade": "SP"},
		"itens": [{"id": 1}, {"id": 2, "preco": 3}, {"preco": 4}],
		"vazio": []
	}`)
	tests := []struct {
		name     string
		required []string
		opts     RequiredOptions
		want     []string
	}{
		{"todos presentes", []string{"nome", "endereco.cidade", "$.itens[0].id"}, RequiredOptions{}, nil},
		{"ausente", []string{"telefone"}, RequiredOptions{}, []string{"missing $.telefone"}},
		{"ausente aninhado", []string{"endereco.cep"}, RequiredOptions{}, []string{"missing $.endereco.cep"}},
		{"null aceito por padrão", []string{"email"}, RequiredOptions{}, nil},
		{"null rejeitado", []string{"email"}, RequiredOptions{RejectNull: true}, []string{"null $.email"}},
		{"curinga acha o elemento faltante", []string{"itens[].id"}, RequiredOptions{}, []string{"missing $.itens[2].id"}},
		{"curinga em vários campos", []string{"itens[].preco"}, RequiredOptions{},
			[]string{"missing $.itens[0].preco"}},
		{"índice fora do array", []string{"itens[9]"}, RequiredOptions{}, []string{"missing $.itens[9]"}},
		{"array vazio com curinga", []string{"vazio[].x"}, RequiredOptions{}, nil},
		{"tipo errado", []string{"nome.primeiro"}, RequiredOptions{}, []string{"wrong_type $.nome"}},
		{"ancestral ausente", []string{"x.y.z"}, RequiredOptions{}, []string{"missing $.x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vs, err := CheckRequired(doc, tt.required, tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, v := range vs {
				got = append(got, string(v.Kind)+" "+v.Path)
			}
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("violações = %v, quer %v", got, tt.want)
			}
		})
	}
}

func TestParsePathErrors(t *testing.T) {
	for _, p := range []string{"", "$", "a[", "a[x]", "a[-1]"} {
		if _, err := parsePath(p); err == nil {
			t.Errorf("parsePath(%q) deveria falhar", p)
		}
	}
}

func TestFormat(t *testing.T) {
	in := `{"b":1.0,"a":[1,{"z":"<&>","y":null}]}`
	tests := []struct {
		name string
		opts FormatOptions
		want string
	}{
		{"minify", FormatOptions{Minify: true}, `{"b":1.0,"a":[1,{"z":"<&>","y":null}]}`},
		{"indent preserva ordem e números", FormatOptions{Indent: "  "},
			"{\n  \"b\": 1.0,\n  \"a\": [\n    1,\n    {\n      \"z\": \"<&>\",\n      \"y\": null\n    }\n  ]\n}"},
		{"ordenar + minify", FormatOptions{SortKeys: true, Minify: true}, `{"a":[1,{"y":null,"z":"<&>"}],"b":1.0}`},
		{"ordenar + indent tab", FormatOptions{SortKeys: true, Indent: "\t"},
			"{\n\t\"a\": [\n\t\t1,\n\t\t{\n\t\t\t\"y\": null,\n\t\t\t\"z\": \"<&>\"\n\t\t}\n\t],\n\t\"b\": 1.0\n}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Format([]byte(in), tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestFormatRejectsInvalid(t *testing.T) {
	if _, err := Format([]byte(`{"a":`), FormatOptions{}); err == nil {
		t.Error("esperava erro para JSON inválido")
	}
}

func TestFormatIsIdempotent(t *testing.T) {
	in := []byte(`{"x":[1,2,{"y":true}],"a":"b"}`)
	opts := FormatOptions{Indent: "  ", SortKeys: true}
	once, _ := Format(in, opts)
	twice, _ := Format(once, opts)
	if string(once) != string(twice) {
		t.Errorf("Format não é idempotente:\n%s\n---\n%s", once, twice)
	}
}
