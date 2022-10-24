package main

import (
	"fmt"
	"go/ast"
	"go/format"
	"os"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
)

func printDebugInfo() {

	fmt.Printf("Running inliner on '%s'\n", os.Getenv("GOFILE"))

	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Printf("  cwd = %s\n", cwd)
	fmt.Printf("  os.Args = %#v\n\n", os.Args)
}

func main() {

	printDebugInfo()

	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	// Parse package
	pkgs, err := packages.Load(&packages.Config{
		Dir:   cwd,
		Mode:  packages.NeedName | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax,
		Tests: false,
	})
	if err != nil {
		panic(err)
	}

	if len(pkgs) != 1 {
		panic(fmt.Sprintf("expected to find one package but found %d", len(pkgs)))
	}

	processPkg(pkgs[0])
}

func processPkg(pkg *packages.Package) {

	for i, synFile := range pkg.Syntax {
		pkg.Syntax[i] = astutil.Apply(synFile, processDeclNode, nil).(*ast.File)
		// for _, dec := range synFile.Decls {
		// ast.Inspect(dec, processDeclNode)
		// }
	}

	// f, err := os.Create("main_gen.go")
	// if err != nil {
	// 	panic(err.Error())
	// }

	err := format.Node(os.Stdout, pkg.Fset, pkg.Syntax[0])
	if err != nil {
		panic(err.Error())
	}
}

func processDeclNode(c *astutil.Cursor) bool {

	n := c.Node()
	if n == nil {
		return false
	}

	funcDecl, ok := n.(*ast.FuncDecl)
	if !ok || funcDecl.Body == nil {
		return true
	}

	if funcDecl.Name.Name != "test" {
		return false
	}

	for _, stmt := range funcDecl.Body.List {

		exprStmt, ok := stmt.(*ast.ExprStmt)
		if !ok {
			continue
		}

		callExpr, ok := exprStmt.X.(*ast.CallExpr)
		if !ok {
			continue
		}

		pkgFuncCallExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}

		pkgIdent, ok := pkgFuncCallExpr.X.(*ast.Ident)
		if !ok || pkgIdent.Name != "cogo" {
			continue
		}
		fmt.Printf("Found: %+v\n", pkgFuncCallExpr)

		if pkgFuncCallExpr.Sel.Name == "Yield" {

			exprStmt.X = &ast.CallExpr{
				Fun: &ast.Ident{
					Name: "Wow",
				},
				Args: nil,
			}
		}
	}

	return true
}
