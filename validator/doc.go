// Package validator reúne as quatro operações principais do go-json-validator:
//
//   - Validação de sintaxe ([ValidateSyntax], [Parse]), com linha e coluna do erro;
//   - Comparação de estruturas ([Compare]), com diferenças indicadas por caminho;
//   - Verificação de campos obrigatórios ([CheckRequired]);
//   - Formatação ([Format]): indentar, minificar e ordenar chaves.
//
// O pacote depende apenas da biblioteca padrão e não faz I/O: todas as funções
// recebem bytes ou valores já decodificados, o que facilita o uso em outros
// programas e os testes.
//
// Os documentos são decodificados em tipos genéricos (map[string]any, []any,
// string, json.Number, bool e nil). Números usam [encoding/json.Number] para não
// perder precisão em inteiros grandes.
package validator
