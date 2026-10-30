// Package cli implementa a interface de linha de comando do jsonvalidator.
//
// Toda a lógica fica em [Run], que recebe os fluxos de entrada e saída como
// parâmetros. Assim os comandos podem ser testados sem executar um processo.
package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/marceloaugst/go-json-validator/validator"
)

// Códigos de saída, seguindo a convenção de ferramentas como diff e grep.
const (
	exitOK      = 0 // tudo certo
	exitInvalid = 1 // o JSON foi analisado e tem problemas (sintaxe, diferenças, campos)
	exitUsage   = 2 // uso incorreto ou erro de I/O
)

// Version é a versão do programa. Pode ser sobrescrita no build com
// -ldflags "-X github.com/marceloaugst/go-json-validator/internal/cli.Version=1.2.3".
var Version = "dev"

const usage = `jsonvalidator - validador de arquivos JSON

Uso:
  jsonvalidator <comando> [opções] [argumentos]

Comandos:
  validate   Valida a sintaxe de um ou mais arquivos
  compare    Compara a estrutura (ou os valores) de dois arquivos
  required   Verifica se campos obrigatórios estão presentes
  format     Indenta, minifica ou ordena as chaves de um arquivo
  version    Mostra a versão
  help       Mostra esta ajuda

Use "jsonvalidator <comando> -h" para ver as opções de cada comando.
Use "-" no lugar de um arquivo para ler da entrada padrão.
As opções devem vir antes dos arquivos.

Códigos de saída: 0 = ok, 1 = problemas encontrados, 2 = erro de uso ou de leitura.
`

// Run executa o comando descrito por args (sem o nome do programa) e devolve o
// código de saída do processo.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
	a := &app{stdin: stdin, stdout: stdout, stderr: stderr}
	switch cmd, rest := args[0], args[1:]; cmd {
	case "validate":
		return a.validate(rest)
	case "compare":
		return a.compare(rest)
	case "required":
		return a.required(rest)
	case "format":
		return a.format(rest)
	case "version", "-v", "--version":
		fmt.Fprintln(stdout, "jsonvalidator", Version)
		return exitOK
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return exitOK
	default:
		fmt.Fprintf(stderr, "comando desconhecido: %q\n\n%s", cmd, usage)
		return exitUsage
	}
}

type app struct {
	stdin          io.Reader
	stdout, stderr io.Writer
}

// newFlagSet cria um FlagSet que escreve a ajuda em stderr e não encerra o processo.
func (a *app) newFlagSet(name, synopsis string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	fs.Usage = func() {
		fmt.Fprintf(a.stderr, "Uso: jsonvalidator %s %s\n\nOpções:\n", name, synopsis)
		fs.PrintDefaults()
	}
	return fs
}

// parseFlags devolve (código, true) quando a execução deve parar.
func parseFlags(fs *flag.FlagSet, args []string) (int, bool) {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK, true
		}
		return exitUsage, true
	}
	return 0, false
}

// read lê um arquivo, ou a entrada padrão quando name é "-".
func (a *app) read(name string) ([]byte, error) {
	if name == "-" {
		return io.ReadAll(a.stdin)
	}
	return os.ReadFile(name)
}

// load lê e valida a sintaxe de um arquivo. Em caso de erro já imprime a
// mensagem e devolve o código de saída adequado.
func (a *app) load(name string) (any, int) {
	data, err := a.read(name)
	if err != nil {
		fmt.Fprintf(a.stderr, "erro: %v\n", err)
		return nil, exitUsage
	}
	v, err := validator.Parse(data)
	if err != nil {
		fmt.Fprintf(a.stderr, "✗ %s: %v\n", name, err)
		return nil, exitInvalid
	}
	return v, exitOK
}

func (a *app) validate(args []string) int {
	fs := a.newFlagSet("validate", "[arquivo...]")
	if code, stop := parseFlags(fs, args); stop {
		return code
	}
	files := fs.Args()
	if len(files) == 0 {
		files = []string{"-"}
	}

	code := exitOK
	for _, name := range files {
		data, err := a.read(name)
		if err != nil {
			fmt.Fprintf(a.stderr, "erro: %v\n", err)
			code = exitUsage
			continue
		}
		if err := validator.ValidateSyntax(data); err != nil {
			fmt.Fprintf(a.stdout, "✗ %s: %v\n", name, err)
			if code == exitOK {
				code = exitInvalid
			}
			continue
		}
		fmt.Fprintf(a.stdout, "✓ %s: JSON válido\n", name)
	}
	return code
}

func (a *app) compare(args []string) int {
	fs := a.newFlagSet("compare", "[opções] <arquivo1> <arquivo2>")
	structure := fs.Bool("structure", false, "compara só chaves e tipos, ignorando valores")
	ignoreOrder := fs.Bool("ignore-order", false, "ignora a ordem dos elementos dos arrays")
	asJSON := fs.Bool("json", false, "imprime o resultado como JSON")
	if code, stop := parseFlags(fs, args); stop {
		return code
	}
	if fs.NArg() != 2 {
		fmt.Fprintln(a.stderr, "erro: informe exatamente dois arquivos")
		fs.Usage()
		return exitUsage
	}

	first, code := a.load(fs.Arg(0))
	if code != exitOK {
		return code
	}
	second, code := a.load(fs.Arg(1))
	if code != exitOK {
		return code
	}

	diffs := validator.Compare(first, second, validator.CompareOptions{
		StructureOnly:    *structure,
		IgnoreArrayOrder: *ignoreOrder,
	})

	if *asJSON {
		if diffs == nil {
			diffs = []validator.Difference{}
		}
		a.printJSON(map[string]any{"equal": len(diffs) == 0, "differences": diffs})
	} else if len(diffs) == 0 {
		mode := "valores"
		if *structure {
			mode = "estrutura"
		}
		fmt.Fprintf(a.stdout, "✓ Documentos equivalentes (%s)\n", mode)
	} else {
		for _, d := range diffs {
			fmt.Fprintln(a.stdout, d)
		}
		fmt.Fprintf(a.stdout, "\n%d diferença(s) encontrada(s)\n", len(diffs))
	}
	if len(diffs) > 0 {
		return exitInvalid
	}
	return exitOK
}

func (a *app) required(args []string) int {
	fs := a.newFlagSet("required", "[opções] <arquivo>")
	fields := fs.String("fields", "", "caminhos obrigatórios separados por vírgula (ex.: nome,endereco.cep,itens[].id)")
	config := fs.String("config", "", "arquivo JSON com a lista de caminhos (array de strings ou {\"required\": [...]})")
	rejectNull := fs.Bool("reject-null", false, "considera campos com valor null como ausentes")
	asJSON := fs.Bool("json", false, "imprime o resultado como JSON")
	if code, stop := parseFlags(fs, args); stop {
		return code
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(a.stderr, "erro: informe exatamente um arquivo")
		fs.Usage()
		return exitUsage
	}

	paths, err := a.requiredPaths(*fields, *config)
	if err != nil {
		fmt.Fprintf(a.stderr, "erro: %v\n", err)
		return exitUsage
	}

	doc, code := a.load(fs.Arg(0))
	if code != exitOK {
		return code
	}
	violations, err := validator.CheckRequired(doc, paths, validator.RequiredOptions{RejectNull: *rejectNull})
	if err != nil {
		fmt.Fprintf(a.stderr, "erro: %v\n", err)
		return exitUsage
	}

	if *asJSON {
		if violations == nil {
			violations = []validator.Violation{}
		}
		a.printJSON(map[string]any{"ok": len(violations) == 0, "violations": violations})
	} else if len(violations) == 0 {
		fmt.Fprintf(a.stdout, "✓ %s: todos os %d campo(s) obrigatório(s) presentes\n", fs.Arg(0), len(paths))
	} else {
		for _, v := range violations {
			fmt.Fprintf(a.stdout, "✗ %s\n", v)
		}
		fmt.Fprintf(a.stdout, "\n%d problema(s) encontrado(s)\n", len(violations))
	}
	if len(violations) > 0 {
		return exitInvalid
	}
	return exitOK
}

// requiredPaths junta os caminhos de --fields e de --config.
func (a *app) requiredPaths(fields, config string) ([]string, error) {
	var paths []string
	for _, f := range strings.Split(fields, ",") {
		if f = strings.TrimSpace(f); f != "" {
			paths = append(paths, f)
		}
	}
	if config != "" {
		data, err := a.read(config)
		if err != nil {
			return nil, err
		}
		fromFile, err := parseRequiredConfig(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", config, err)
		}
		paths = append(paths, fromFile...)
	}
	if len(paths) == 0 {
		return nil, errors.New("informe os campos com --fields ou --config")
	}
	return paths, nil
}

// parseRequiredConfig aceita ["a","b"] ou {"required": ["a","b"]}.
func parseRequiredConfig(data []byte) ([]string, error) {
	var list []string
	if err := json.Unmarshal(data, &list); err == nil {
		return list, nil
	}
	var obj struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(data, &obj); err != nil || obj.Required == nil {
		return nil, errors.New(`formato inválido: use um array de strings ou {"required": [...]}`)
	}
	return obj.Required, nil
}

func (a *app) format(args []string) int {
	fs := a.newFlagSet("format", "[opções] [arquivo]")
	indent := fs.Int("indent", 2, "espaços por nível de indentação")
	tabs := fs.Bool("tabs", false, "indenta com tabs")
	minify := fs.Bool("minify", false, "remove espaços e quebras de linha")
	sortKeys := fs.Bool("sort-keys", false, "ordena as chaves dos objetos")
	write := fs.Bool("w", false, "grava o resultado no próprio arquivo")
	output := fs.String("o", "", "grava o resultado neste arquivo")
	if code, stop := parseFlags(fs, args); stop {
		return code
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(a.stderr, "erro: informe no máximo um arquivo")
		fs.Usage()
		return exitUsage
	}
	name := "-"
	if fs.NArg() == 1 {
		name = fs.Arg(0)
	}
	if *write && (name == "-" || *output != "") {
		fmt.Fprintln(a.stderr, "erro: -w exige um arquivo e não pode ser combinado com -o")
		return exitUsage
	}
	if *indent < 0 {
		fmt.Fprintln(a.stderr, "erro: --indent não pode ser negativo")
		return exitUsage
	}

	data, err := a.read(name)
	if err != nil {
		fmt.Fprintf(a.stderr, "erro: %v\n", err)
		return exitUsage
	}

	opts := validator.FormatOptions{Minify: *minify, SortKeys: *sortKeys, Indent: strings.Repeat(" ", *indent)}
	if *tabs {
		opts.Indent = "\t"
	}
	out, err := validator.Format(data, opts)
	if err != nil {
		fmt.Fprintf(a.stderr, "✗ %s: %v\n", name, err)
		return exitInvalid
	}
	out = append(out, '\n')

	dest := *output
	if *write {
		dest = name
	}
	if dest == "" {
		a.stdout.Write(out)
		return exitOK
	}
	if err := os.WriteFile(dest, out, 0o644); err != nil {
		fmt.Fprintf(a.stderr, "erro: %v\n", err)
		return exitUsage
	}
	fmt.Fprintf(a.stderr, "✓ %s formatado\n", dest)
	return exitOK
}

func (a *app) printJSON(v any) {
	enc := json.NewEncoder(a.stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	enc.Encode(v)
}
