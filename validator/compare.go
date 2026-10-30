package validator

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// DiffKind classifica uma diferença encontrada por [Compare].
type DiffKind string

const (
	// Added indica um valor que existe apenas no segundo documento.
	Added DiffKind = "added"
	// Removed indica um valor que existe apenas no primeiro documento.
	Removed DiffKind = "removed"
	// Changed indica o mesmo caminho e o mesmo tipo, mas com valores diferentes.
	Changed DiffKind = "changed"
	// TypeChanged indica o mesmo caminho com tipos diferentes (ex.: string vs número).
	TypeChanged DiffKind = "type_changed"
)

// Difference é uma diferença entre dois documentos.
type Difference struct {
	Path string   `json:"path"` // ex.: $.usuario.emails[0]
	Kind DiffKind `json:"kind"`
	Old  any      `json:"old,omitempty"` // valor no primeiro documento (se existir)
	New  any      `json:"new,omitempty"` // valor no segundo documento (se existir)
}

// String formata a diferença em uma linha legível.
func (d Difference) String() string {
	switch d.Kind {
	case Added:
		return fmt.Sprintf("+ %s: %s (somente no segundo)", d.Path, render(d.New))
	case Removed:
		return fmt.Sprintf("- %s: %s (somente no primeiro)", d.Path, render(d.Old))
	case TypeChanged:
		return fmt.Sprintf("! %s: tipo %s → %s", d.Path, typeName(d.Old), typeName(d.New))
	default:
		return fmt.Sprintf("~ %s: %s → %s", d.Path, render(d.Old), render(d.New))
	}
}

// CompareOptions ajusta o comportamento de [Compare].
type CompareOptions struct {
	// StructureOnly compara apenas chaves e tipos, ignorando os valores
	// escalares. Útil para checar se dois documentos têm o mesmo "formato".
	StructureOnly bool
	// IgnoreArrayOrder trata arrays como conjuntos: [1,2] e [2,1] são iguais.
	IgnoreArrayOrder bool
}

// Compare compara dois documentos já decodificados (veja [Parse]) e devolve as
// diferenças em ordem determinística. Um resultado vazio significa que os
// documentos são equivalentes segundo as opções.
func Compare(a, b any, opts CompareOptions) []Difference {
	var diffs []Difference
	compare("$", a, b, opts, &diffs)
	return diffs
}

func compare(path string, a, b any, opts CompareOptions, out *[]Difference) {
	ta, tb := typeName(a), typeName(b)
	if ta != tb {
		*out = append(*out, Difference{Path: path, Kind: TypeChanged, Old: a, New: b})
		return
	}

	switch av := a.(type) {
	case map[string]any:
		bv := b.(map[string]any)
		for _, k := range sortedKeys(av, bv) {
			child := joinKey(path, k)
			x, inA := av[k]
			y, inB := bv[k]
			switch {
			case inA && !inB:
				*out = append(*out, Difference{Path: child, Kind: Removed, Old: x})
			case !inA && inB:
				*out = append(*out, Difference{Path: child, Kind: Added, New: y})
			default:
				compare(child, x, y, opts, out)
			}
		}
	case []any:
		bv := b.([]any)
		// idxA[i] e idxB[i] guardam a posição original do elemento i, para que os
		// caminhos apontem para o arquivo como ele está, mesmo com --ignore-order.
		idxA, idxB := identity(len(av)), identity(len(bv))
		if opts.IgnoreArrayOrder {
			av, idxA = sortArray(av, opts)
			bv, idxB = sortArray(bv, opts)
		}
		for i := 0; i < max(len(av), len(bv)); i++ {
			switch {
			case i >= len(bv):
				*out = append(*out, Difference{Path: indexPath(path, idxA[i]), Kind: Removed, Old: av[i]})
			case i >= len(av):
				*out = append(*out, Difference{Path: indexPath(path, idxB[i]), Kind: Added, New: bv[i]})
			default:
				// Par de elementos: o caminho usa o índice no primeiro documento.
				compare(indexPath(path, idxA[i]), av[i], bv[i], opts, out)
			}
		}
	default:
		if !opts.StructureOnly && !scalarEqual(a, b) {
			*out = append(*out, Difference{Path: path, Kind: Changed, Old: a, New: b})
		}
	}
}

// sortArray devolve uma cópia de in ordenada por uma chave canônica, de forma
// que a ordem original deixe de importar, junto com o índice original de cada
// elemento da cópia.
func sortArray(in []any, opts CompareOptions) ([]any, []int) {
	keys := make([]string, len(in))
	for i, e := range in {
		keys[i] = canonical(e, opts.StructureOnly)
	}
	idx := identity(len(in))
	sort.SliceStable(idx, func(i, j int) bool { return keys[idx[i]] < keys[idx[j]] })
	sorted := make([]any, len(in))
	for i, orig := range idx {
		sorted[i] = in[orig]
	}
	return sorted, idx
}

func identity(n int) []int {
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	return idx
}

func indexPath(path string, i int) string { return fmt.Sprintf("%s[%d]", path, i) }

// canonical gera uma representação textual estável de v, usada para ordenar.
func canonical(v any, structureOnly bool) string {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var sb strings.Builder
		sb.WriteString("{")
		for _, k := range keys {
			sb.WriteString(strconv.Quote(k) + ":" + canonical(t[k], structureOnly) + ",")
		}
		sb.WriteString("}")
		return sb.String()
	case []any:
		parts := make([]string, len(t))
		for i, e := range t {
			parts[i] = canonical(e, structureOnly)
		}
		if structureOnly {
			sort.Strings(parts)
		}
		return "[" + strings.Join(parts, ",") + "]"
	default:
		if structureOnly {
			return typeName(v)
		}
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func scalarEqual(a, b any) bool {
	na, okA := a.(json.Number)
	nb, okB := b.(json.Number)
	if okA && okB {
		if na == nb {
			return true
		}
		fa, errA := strconv.ParseFloat(string(na), 64)
		fb, errB := strconv.ParseFloat(string(nb), 64)
		return errA == nil && errB == nil && fa == fb // 1 == 1.0 == 1e0
	}
	return a == b
}

func typeName(v any) string {
	switch v.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case json.Number, float64:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%T", v)
	}
}

func sortedKeys(a, b map[string]any) []string {
	set := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		set[k] = struct{}{}
	}
	for k := range b {
		set[k] = struct{}{}
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// joinKey acrescenta uma chave ao caminho, usando colchetes quando a chave tem
// caracteres que tornariam o caminho ambíguo.
func joinKey(path, key string) string {
	if key != "" && strings.IndexFunc(key, func(r rune) bool {
		return !(r == '_' || r == '-' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z')
	}) < 0 {
		return path + "." + key
	}
	return path + "[" + strconv.Quote(key) + "]"
}

// render mostra um valor em uma linha, truncando valores muito longos.
func render(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	s := string(b)
	if r := []rune(s); len(r) > 60 {
		return string(r[:57]) + "..."
	}
	return s
}
