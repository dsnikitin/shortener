// Package exit предоставляет анализатор для запрета использования os.Exit в функции main.
//
// Анализатор предназначен для использования в multichecker и помогает
// предотвратить преждевременное завершение программы в функции main,
// что важно для приложений, требующих корректной очистки ресурсов.
package exit

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "exitcheck",
	Doc:  "disallows the use of os.Exit in the main function",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	if pass.Pkg.Name() != "main" {
		return nil, nil
	}

	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			if fn, ok := node.(*ast.FuncDecl); ok {
				if fn.Name.Name != "main" {
					return true
				}

				for _, stm := range fn.Body.List {
					stmExpr, ok := stm.(*ast.ExprStmt)
					if !ok {
						continue
					}
					callExpr, ok := stmExpr.X.(*ast.CallExpr)
					if !ok {
						continue
					}

					selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
					if !ok {
						continue
					}

					if v, ok := selExpr.X.(*ast.Ident); ok {
						if v.Name == "os" && selExpr.Sel.Name == "Exit" {
							pass.Reportf(selExpr.Pos(), "os.Exit is not allowed in the main function")
						}
					}
				}

				return false
			}

			return true
		})
	}

	return nil, nil
}
