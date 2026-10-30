package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// run executa a CLI com stdin simulado e devolve código, stdout e stderr.
func run(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Run(args, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestValidate(t *testing.T) {
	ok := writeTemp(t, "ok.json", `{"a":1}`)
	bad := writeTemp(t, "bad.json", `{"a":}`)

	if code, out, _ := run(t, "", "validate", ok); code != exitOK || !strings.Contains(out, "JSON válido") {
		t.Errorf("arquivo válido: code=%d out=%q", code, out)
	}
	if code, out, _ := run(t, "", "validate", ok, bad); code != exitInvalid || !strings.Contains(out, "linha 1") {
		t.Errorf("arquivo inválido: code=%d out=%q", code, out)
	}
	if code, _, _ := run(t, `[1]`, "validate"); code != exitOK {
		t.Errorf("stdin válido: code=%d", code)
	}
	if code, _, _ := run(t, "", "validate", "nao-existe.json"); code != exitUsage {
		t.Errorf("arquivo inexistente: code=%d", code)
	}
}

func TestCompare(t *testing.T) {
	a := writeTemp(t, "a.json", `{"x":1,"y":[1,2]}`)
	b := writeTemp(t, "b.json", `{"x":2,"y":[2,1]}`)

	if code, out, _ := run(t, "", "compare", a, b); code != exitInvalid || !strings.Contains(out, "$.x") {
		t.Errorf("code=%d out=%q", code, out)
	}
	if code, _, _ := run(t, "", "compare", "--structure", "--ignore-order", a, b); code != exitOK {
		t.Errorf("estrutura igual deveria sair com 0, code=%d", code)
	}
	if code, out, _ := run(t, "", "compare", "--json", a, b); code != exitInvalid || !strings.Contains(out, `"equal": false`) {
		t.Errorf("--json: code=%d out=%q", code, out)
	}
	if code, _, _ := run(t, "", "compare", a); code != exitUsage {
		t.Errorf("um só arquivo deveria ser erro de uso, code=%d", code)
	}
}

func TestRequired(t *testing.T) {
	doc := writeTemp(t, "doc.json", `{"nome":"Ana","email":null}`)
	cfg := writeTemp(t, "cfg.json", `{"required":["nome","telefone"]}`)

	if code, out, _ := run(t, "", "required", "--fields", "nome,email", doc); code != exitOK {
		t.Errorf("code=%d out=%q", code, out)
	}
	if code, out, _ := run(t, "", "required", "--fields", "email", "--reject-null", doc); code != exitInvalid || !strings.Contains(out, "null") {
		t.Errorf("code=%d out=%q", code, out)
	}
	if code, out, _ := run(t, "", "required", "--config", cfg, doc); code != exitInvalid || !strings.Contains(out, "$.telefone") {
		t.Errorf("code=%d out=%q", code, out)
	}
	if code, _, _ := run(t, "", "required", doc); code != exitUsage {
		t.Errorf("sem campos deveria ser erro de uso, code=%d", code)
	}
}

func TestFormat(t *testing.T) {
	code, out, _ := run(t, `{"b":1,"a":2}`, "format", "--minify", "--sort-keys")
	if code != exitOK || out != "{\"a\":2,\"b\":1}\n" {
		t.Errorf("code=%d out=%q", code, out)
	}

	p := writeTemp(t, "f.json", `{"a":1}`)
	if code, _, _ := run(t, "", "format", "-w", "--tabs", p); code != exitOK {
		t.Fatalf("-w falhou: %d", code)
	}
	got, _ := os.ReadFile(p)
	if string(got) != "{\n\t\"a\": 1\n}\n" {
		t.Errorf("arquivo gravado = %q", got)
	}

	if code, _, _ := run(t, `{`, "format"); code != exitInvalid {
		t.Errorf("JSON inválido deveria sair com 1, code=%d", code)
	}
}

func TestUnknownCommandAndHelp(t *testing.T) {
	if code, _, _ := run(t, "", "xyz"); code != exitUsage {
		t.Errorf("comando desconhecido: code=%d", code)
	}
	if code, out, _ := run(t, "", "help"); code != exitOK || !strings.Contains(out, "Comandos") {
		t.Errorf("help: code=%d", code)
	}
	if code, _, _ := run(t, ""); code != exitUsage {
		t.Errorf("sem args: code=%d", code)
	}
}
