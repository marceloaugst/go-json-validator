package validator

import (
	"bytes"
	"encoding/json"
	"strings"
)

// FormatOptions ajusta o comportamento de [Format].
type FormatOptions struct {
	// Indent é a string usada em cada nível de indentação (ex.: "  " ou "\t").
	// Ignorado quando Minify é true.
	Indent string
	// Minify remove todo espaço em branco desnecessário.
	Minify bool
	// SortKeys ordena as chaves dos objetos alfabeticamente. Sem essa opção a
	// ordem original das chaves é preservada.
	SortKeys bool
}

// Format valida data e a reescreve segundo opts. O resultado não termina com
// quebra de linha. Documentos inválidos retornam um *[SyntaxError] e nada é
// formatado.
func Format(data []byte, opts FormatOptions) ([]byte, error) {
	v, err := Parse(data)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if opts.SortKeys {
		// Reencodar via map faz o encoding/json ordenar as chaves.
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false) // preserva <, > e & como estão
		if !opts.Minify {
			enc.SetIndent("", opts.Indent)
		}
		if err := enc.Encode(v); err != nil {
			return nil, err
		}
		return bytes.TrimRight(buf.Bytes(), "\n"), nil
	}

	// Sem ordenar, trabalha direto nos bytes originais: preserva a ordem das
	// chaves e a escrita exata dos números (ex.: 1.0 continua 1.0).
	if opts.Minify {
		if err := json.Compact(&buf, data); err != nil {
			return nil, err
		}
	} else if err := json.Indent(&buf, data, "", opts.Indent); err != nil {
		return nil, err
	}
	return []byte(strings.TrimSpace(buf.String())), nil
}
