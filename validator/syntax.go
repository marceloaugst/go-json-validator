package validator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"unicode/utf8"
)

// SyntaxError descreve um problema de sintaxe com a posição onde ele ocorreu.
type SyntaxError struct {
	Line   int    // linha (começa em 1)
	Column int    // coluna em caracteres (começa em 1)
	Offset int    // deslocamento em bytes (começa em 0)
	Msg    string // mensagem em português
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("linha %d, coluna %d: %s", e.Line, e.Column, e.Msg)
}

// Parse valida a sintaxe de data e devolve o valor decodificado.
//
// O documento deve conter exatamente um valor JSON (objeto, array, string,
// número, booleano ou null). Em caso de erro, o retorno é um *[SyntaxError].
func Parse(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, toSyntaxError(data, err)
	}

	// Depois do primeiro valor só pode haver espaço em branco.
	start := skipSpace(data, int(dec.InputOffset()))
	if start < len(data) {
		var extra any
		if err := dec.Decode(&extra); err != nil && !errors.Is(err, io.EOF) {
			return nil, toSyntaxError(data, err)
		}
		return nil, newSyntaxError(data, start, "conteúdo extra após o fim do valor JSON")
	}
	return v, nil
}

// ValidateSyntax informa se data é um documento JSON válido.
// Quando não é, o erro retornado é um *[SyntaxError].
func ValidateSyntax(data []byte) error {
	_, err := Parse(data)
	return err
}

func skipSpace(data []byte, i int) int {
	for i < len(data) && (data[i] == ' ' || data[i] == '\t' || data[i] == '\n' || data[i] == '\r') {
		i++
	}
	return i
}

// newSyntaxError converte um deslocamento em bytes para linha e coluna.
func newSyntaxError(data []byte, offset int, msg string) *SyntaxError {
	offset = min(max(offset, 0), len(data))
	line, col := 1, 1
	for _, r := range string(data[:offset]) {
		if r == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return &SyntaxError{Line: line, Column: col, Offset: offset, Msg: msg}
}

func toSyntaxError(data []byte, err error) error {
	var se *json.SyntaxError
	switch {
	case len(bytes.TrimSpace(data)) == 0:
		return newSyntaxError(data, len(data), "entrada vazia: esperava um valor JSON")
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return newSyntaxError(data, len(data), "fim inesperado do documento (falta fechar chave, colchete ou aspas?)")
	case errors.As(err, &se):
		// Offset é a quantidade de bytes lidos; o caractere problemático é o último.
		off := int(se.Offset) - 1
		// Se o offset cai no meio de um caractere multibyte, volta ao início dele.
		for off > 0 && off < len(data) && !utf8.RuneStart(data[off]) {
			off--
		}
		return newSyntaxError(data, off, translate(se.Error()))
	default:
		return newSyntaxError(data, 0, err.Error())
	}
}

var invalidChar = regexp.MustCompile(`^invalid character (.+) (looking for beginning of value|after object key:value pair|after object key|after array element|in string literal|in string escape code|in numeric literal|in exponent of numeric literal|after decimal point in numeric literal|in \\u hexadecimal character escape|looking for beginning of object key string)$`)

var literalErr = regexp.MustCompile(`^invalid character (.+) in literal (true|false|null) \(expecting (.+)\)$`)

var contexts = map[string]string{
	"looking for beginning of value":             "onde se esperava o início de um valor",
	"looking for beginning of object key string": "onde se esperava o nome de uma chave entre aspas",
	"after object key":                           "depois do nome da chave (esperava ':')",
	"after object key:value pair":                "depois do par chave:valor (esperava ',' ou '}')",
	"after array element":                        "depois de um item do array (esperava ',' ou ']')",
	"in string literal":                          "dentro de uma string (caractere de controle precisa de escape)",
	"in string escape code":                      "em uma sequência de escape inválida",
	"in numeric literal":                         "em um número",
	"in exponent of numeric literal":             "no expoente de um número",
	"after decimal point in numeric literal":     "depois da vírgula decimal de um número",
	`in \u hexadecimal character escape`:         "em uma sequência \\u inválida",
}

// translate traduz as mensagens mais comuns de encoding/json.
// Mensagens desconhecidas são devolvidas como vieram.
func translate(msg string) string {
	if m := invalidChar.FindStringSubmatch(msg); m != nil {
		return "caractere inválido " + m[1] + " " + contexts[m[2]]
	}
	if m := literalErr.FindStringSubmatch(msg); m != nil {
		return fmt.Sprintf("caractere inválido %s no literal %s (esperava %s)", m[1], m[2], m[3])
	}
	return msg
}
