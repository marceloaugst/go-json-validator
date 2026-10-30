package validator

import (
	"fmt"
	"strconv"
	"strings"
)

// RequiredOptions ajusta o comportamento de [CheckRequired].
type RequiredOptions struct {
	// RejectNull faz com que um campo presente com valor null também seja
	// considerado violação. Por padrão, null conta como "presente".
	RejectNull bool
}

// ViolationKind classifica uma violação de campo obrigatório.
type ViolationKind string

const (
	// Missing indica que o campo (ou algum ancestral) não existe.
	Missing ViolationKind = "missing"
	// Null indica que o campo existe mas é null (somente com RejectNull).
	Null ViolationKind = "null"
	// WrongType indica que o caminho atravessa um valor do tipo errado,
	// por exemplo pedir "a.b" quando "a" é uma string.
	WrongType ViolationKind = "wrong_type"
)

// Violation é um campo obrigatório que não foi satisfeito.
type Violation struct {
	Path string        `json:"path"` // caminho concreto, ex.: $.itens[2].id
	Kind ViolationKind `json:"kind"`
}

// String formata a violação em uma linha legível.
func (v Violation) String() string {
	switch v.Kind {
	case Null:
		return fmt.Sprintf("%s: campo obrigatório é null", v.Path)
	case WrongType:
		return fmt.Sprintf("%s: tipo incompatível com o caminho exigido", v.Path)
	default:
		return fmt.Sprintf("%s: campo obrigatório ausente", v.Path)
	}
}

// segment é um passo de um caminho: uma chave, um índice ou um curinga.
type segment struct {
	key   string
	index int
	kind  segKind
}

type segKind int

const (
	segKey segKind = iota
	segIndex
	segAll // []: todos os elementos do array
)

// parsePath converte um caminho como "usuario.enderecos[].cep" em segmentos.
//
// Sintaxe aceita:
//
//	a.b.c    chaves aninhadas (prefixo "$." opcional)
//	a[0]     elemento de índice 0 do array a
//	a[]      todos os elementos do array a
//
// Chaves que contêm '.', '[' ou ']' não são suportadas.
func parsePath(path string) ([]segment, error) {
	p := strings.TrimPrefix(strings.TrimPrefix(path, "$"), ".")
	if p == "" {
		return nil, fmt.Errorf("caminho vazio")
	}

	var segs []segment
	for i := 0; i < len(p); {
		switch p[i] {
		case '.':
			i++
		case '[':
			end := strings.IndexByte(p[i:], ']')
			if end < 0 {
				return nil, fmt.Errorf("caminho %q: falta ']'", path)
			}
			inner := p[i+1 : i+end]
			if inner == "" {
				segs = append(segs, segment{kind: segAll})
			} else {
				n, err := strconv.Atoi(inner)
				if err != nil || n < 0 {
					return nil, fmt.Errorf("caminho %q: índice inválido %q", path, inner)
				}
				segs = append(segs, segment{kind: segIndex, index: n})
			}
			i += end + 1
		default:
			end := strings.IndexAny(p[i:], ".[")
			if end < 0 {
				end = len(p) - i
			}
			segs = append(segs, segment{kind: segKey, key: p[i : i+end]})
			i += end
		}
	}
	if len(segs) == 0 {
		return nil, fmt.Errorf("caminho %q inválido", path)
	}
	return segs, nil
}

// CheckRequired verifica se cada caminho em required existe em doc (um valor
// retornado por [Parse]). Devolve uma violação por campo não satisfeito; um
// resultado vazio significa que todos os campos estão presentes.
//
// Um caminho com [] é verificado em todos os elementos do array. Se o array
// estiver vazio, não há o que verificar e nenhuma violação é gerada.
func CheckRequired(doc any, required []string, opts RequiredOptions) ([]Violation, error) {
	var out []Violation
	for _, r := range required {
		segs, err := parsePath(r)
		if err != nil {
			return nil, err
		}
		walk(doc, segs, "$", opts, &out)
	}
	return out, nil
}

func walk(node any, segs []segment, path string, opts RequiredOptions, out *[]Violation) {
	if len(segs) == 0 {
		if node == nil && opts.RejectNull {
			*out = append(*out, Violation{Path: path, Kind: Null})
		}
		return
	}

	s, rest := segs[0], segs[1:]
	switch s.kind {
	case segKey:
		obj, ok := node.(map[string]any)
		if !ok {
			*out = append(*out, Violation{Path: path, Kind: WrongType})
			return
		}
		child := joinKey(path, s.key)
		v, found := obj[s.key]
		if !found {
			*out = append(*out, Violation{Path: child, Kind: Missing})
			return
		}
		walk(v, rest, child, opts, out)
	case segIndex:
		arr, ok := node.([]any)
		if !ok {
			*out = append(*out, Violation{Path: path, Kind: WrongType})
			return
		}
		child := fmt.Sprintf("%s[%d]", path, s.index)
		if s.index >= len(arr) {
			*out = append(*out, Violation{Path: child, Kind: Missing})
			return
		}
		walk(arr[s.index], rest, child, opts, out)
	case segAll:
		arr, ok := node.([]any)
		if !ok {
			*out = append(*out, Violation{Path: path, Kind: WrongType})
			return
		}
		for i, e := range arr {
			walk(e, rest, fmt.Sprintf("%s[%d]", path, i), opts, out)
		}
	}
}
