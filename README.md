# go-json-validator

Ferramenta de linha de comando **e** biblioteca em Go para trabalhar com arquivos JSON. Ela faz quatro coisas:

| Comando    | O que faz                                                              |
|------------|------------------------------------------------------------------------|
| `validate` | Valida a sintaxe e aponta **linha e coluna** do erro                   |
| `compare`  | Compara dois JSONs (estrutura ou valores) e lista as diferenças por caminho |
| `required` | Verifica se campos obrigatórios existem, inclusive dentro de arrays    |
| `format`   | Indenta, minifica e/ou ordena as chaves                                |

Sem dependências externas: só a biblioteca padrão do Go.

---

## Instalação

Requer Go 1.22 ou superior.

```bash
git clone https://github.com/marceloaugst/go-json-validator.git
cd go-json-validator
make build            # gera bin/jsonvalidator
# ou, sem make:
go build -o bin/jsonvalidator ./cmd/jsonvalidator
```

## Uso

Todos os comandos aceitam `-` no lugar do arquivo para ler da entrada padrão, então funcionam em pipes (`curl ... | jsonvalidator validate`). As opções devem vir **antes** dos nomes de arquivo.

### `validate` — validar sintaxe

```console
$ jsonvalidator validate examples/valido.json examples/invalido.json
✓ examples/valido.json: JSON válido
✗ examples/invalido.json: linha 5, coluna 1: caractere inválido '}' onde se esperava o nome de uma chave entre aspas
```

O arquivo de exemplo tem uma vírgula sobrando na linha 4 — um erro clássico. As mensagens do `encoding/json` do Go (em inglês, com posição em bytes) são traduzidas e convertidas para linha/coluna.

### `compare` — comparar estruturas

```console
$ jsonvalidator compare examples/valido.json examples/pedido_v2.json
~ $.email: "ana@example.com" → "ana.souza@example.com"
~ $.endereco.cidade: "São Paulo" → "Campinas"
+ $.endereco.pais: "BR" (somente no segundo)
- $.idade: 29 (somente no primeiro)
! $.pedidos[1].total: tipo number → string

5 diferença(s) encontrada(s)
```

Legenda: `+` adicionado, `-` removido, `~` valor alterado, `!` tipo alterado.

Opções:

- `--structure` compara apenas chaves e tipos, ignorando valores. Responde "esses dois documentos têm o mesmo formato?":

  ```console
  $ jsonvalidator compare --structure examples/valido.json examples/pedido_v2.json
  + $.endereco.pais: "BR" (somente no segundo)
  - $.idade: 29 (somente no primeiro)
  ! $.pedidos[1].total: tipo number → string
  ```
- `--ignore-order` trata arrays como conjuntos (`[1,2]` == `[2,1]`).
  Os caminhos reportados usam o índice **original** do elemento no arquivo (no primeiro documento quando o elemento existe nos dois; no documento onde ele existe, quando é adicionado ou removido).
- `--json` imprime o resultado em JSON, para consumo por outros programas.

A ordem das chaves em objetos nunca importa, e `1`, `1.0` e `1e0` são considerados o mesmo número.

### `required` — campos obrigatórios

```console
$ jsonvalidator required --fields "nome,email,telefone,pedidos[].id" examples/valido.json
✗ $.telefone: campo obrigatório ausente

1 problema(s) encontrado(s)
```

Os campos podem vir de `--fields` (separados por vírgula), de um arquivo com `--config`, ou dos dois. O arquivo pode ser um array de strings ou `{"required": [...]}` (veja [`examples/obrigatorios.json`](examples/obrigatorios.json)).

Sintaxe dos caminhos:

| Caminho              | Significado                                              |
|----------------------|----------------------------------------------------------|
| `nome`               | chave `nome` na raiz                                     |
| `endereco.cep`       | chave aninhada (o prefixo `$.` é opcional)               |
| `pedidos[0].id`      | `id` do primeiro elemento de `pedidos`                   |
| `pedidos[].id`       | `id` em **todos** os elementos de `pedidos`              |

Cada elemento que falhar é reportado com seu caminho exato (ex.: `$.pedidos[2].id`). Por padrão, um campo com valor `null` conta como presente; use `--reject-null` para tratá-lo como violação. Um array vazio com `[]` não gera violações, pois não há elementos a verificar.

### `format` — formatar

```console
$ echo '{"b":1,"a":{"z":true,"y":null}}' | jsonvalidator format --sort-keys --indent 4
{
    "a": {
        "y": null,
        "z": true
    },
    "b": 1
}
```

| Opção         | Efeito                                                     |
|---------------|------------------------------------------------------------|
| `--indent N`  | N espaços por nível (padrão 2)                             |
| `--tabs`      | indenta com tabs                                           |
| `--minify`    | remove todo espaço desnecessário                           |
| `--sort-keys` | ordena as chaves alfabeticamente                           |
| `-w`          | grava no próprio arquivo                                   |
| `-o arquivo`  | grava em outro arquivo                                     |

Sem `--sort-keys`, a ordem original das chaves e a escrita dos números (`1.0` continua `1.0`) são preservadas. Arquivos inválidos nunca são reescritos.

### Códigos de saída

Seguem a convenção de `diff` e `grep`, o que permite usar a ferramenta em scripts e CI:

| Código | Significado                                                       |
|--------|-------------------------------------------------------------------|
| `0`    | tudo certo                                                        |
| `1`    | JSON analisado com problemas (sintaxe, diferenças, campos ausentes) |
| `2`    | uso incorreto ou erro de leitura/escrita                          |

```bash
# exemplo em um pipeline de CI
jsonvalidator validate config/*.json && jsonvalidator required --config schema/obrigatorios.json config/app.json
```

---

## Usando como biblioteca

O pacote [`validator`](validator) não faz I/O e pode ser importado por outros programas:

```go
import "github.com/marceloaugst/go-json-validator/validator"

doc, err := validator.Parse(data) // valida a sintaxe e decodifica
if err != nil {
	var se *validator.SyntaxError
	if errors.As(err, &se) {
		fmt.Printf("erro na linha %d, coluna %d: %s\n", se.Line, se.Column, se.Msg)
	}
	return err
}

violations, _ := validator.CheckRequired(doc, []string{"nome", "pedidos[].id"}, validator.RequiredOptions{})
diffs := validator.Compare(docA, docB, validator.CompareOptions{IgnoreArrayOrder: true})
pretty, _ := validator.Format(data, validator.FormatOptions{Indent: "  ", SortKeys: true})
```

A documentação completa está nos comentários do código e pode ser lida com `go doc`:

```bash
go doc ./validator
go doc ./validator.Compare
```

---

## Como funciona

### Estrutura do projeto

```
.
├── cmd/jsonvalidator/    # main.go: apenas chama cli.Run
├── internal/cli/         # subcomandos, flags, formatação da saída, códigos de saída
├── validator/            # lógica de negócio, reutilizável e sem I/O
│   ├── syntax.go         #   Parse / ValidateSyntax, SyntaxError com linha e coluna
│   ├── compare.go        #   Compare, Difference, CompareOptions
│   ├── required.go       #   CheckRequired, parser de caminhos
│   └── format.go         #   Format
└── examples/             # arquivos para testar os comandos
```

A separação em duas camadas é intencional: `validator` conhece JSON mas não sabe nada de terminal; `cli` conhece terminal mas delega toda a lógica. Por isso `cli.Run` recebe `stdin`, `stdout` e `stderr` como parâmetros — os testes rodam comandos completos sem iniciar processos.

### Decisões de design

- **Validação com posição.** O `encoding/json` informa o erro como deslocamento em bytes. `syntax.go` converte isso para linha e coluna (contando caracteres, não bytes, para que acentos não distorçam a coluna) e traduz as mensagens mais comuns.
- **Um único valor por documento.** Depois de decodificar o primeiro valor, qualquer coisa que não seja espaço em branco é erro (`{} {}` e `{} x` são rejeitados). O `json.Unmarshal` faria o mesmo; o `Decoder` sozinho, não.
- **`json.Number` em vez de `float64`.** Decodificar números como `float64` perderia precisão em inteiros grandes (ex.: IDs de 64 bits) e reescreveria `1.0` como `1`. Com `json.Number` o texto original é mantido, e na comparação `1`, `1.0` e `1e0` são tratados como iguais.
- **Comparação determinística.** As chaves são visitadas em ordem alfabética, então a saída é estável entre execuções (importante para diff em CI e para testes).
- **`--ignore-order` por forma canônica.** Cada elemento de um array vira uma string canônica (chaves ordenadas) e os arrays são ordenados por ela antes de comparar. Assim, `[{"a":1},2]` e `[2,{"a":1}]` são equivalentes.
- **`format` sem `--sort-keys` trabalha nos bytes originais** (`json.Indent` / `json.Compact`), preservando ordem e escrita dos números. Com `--sort-keys`, o documento é recodificado e `SetEscapeHTML(false)` evita transformar `<`, `>` e `&` em `<`.

### Limitações conhecidas

- Chaves duplicadas (`{"a":1,"a":2}`) são aceitas — o JSON permite, e o último valor vence.
- Caminhos de `required` não suportam chaves que contenham `.`, `[` ou `]`.
- Não implementa JSON Schema; o `required` cobre apenas a presença de campos, não seus tipos.
- Comentários e vírgulas finais (JSONC/JSON5) são tratados como erro de sintaxe, como manda o padrão.

---

## Desenvolvimento

```bash
make test    # go test -race ./...
make cover   # cobertura
make lint    # gofmt + go vet
make build   # compila em bin/
```

Os testes da biblioteca usam *table-driven tests* cobrindo casos de borda (entrada vazia, conteúdo extra, acentos, arrays vazios, tipos incompatíveis). Os testes da CLI executam comandos completos e verificam saída e código de saída.

## Ideias para evoluir

- Suporte a JSON Schema (tipos, `pattern`, `enum`)
- Detecção de chaves duplicadas
- Validar diretórios recursivamente (`validate ./config`)
- Saída colorida e formato SARIF/JUnit para CI

## Licença

Defina a licença de sua preferência (ex.: MIT) antes de publicar.
