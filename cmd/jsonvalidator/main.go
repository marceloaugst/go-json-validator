// Comando jsonvalidator: valida, compara, verifica campos obrigatórios e
// formata arquivos JSON. Execute `jsonvalidator help` para ver o uso.
package main

import (
	"os"

	"github.com/marceloaugst/go-json-validator/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
