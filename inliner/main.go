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

	if !funcDeclCallsCogo(funcDecl) {
		return false
	}

	for i, stmt := range funcDecl.Body.List {

		// Find functions calls in the style of 'cogo.ABC123()'
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

		// Now that we found a call to cogo decide what to do
		if pkgFuncCallExpr.Sel.Name == "Begin" {

			beginStmt := &ast.SwitchStmt{
				Tag: ast.NewIdent("state"),
				Body: &ast.BlockStmt{
					List: []ast.Stmt{
						&ast.CaseClause{
							List: nil,
							Body: []ast.Stmt{
								&ast.ExprStmt{
									X: &ast.CallExpr{
										Fun: &ast.Ident{
											Name: "Wow",
										},
										Args: nil,
									},
								},
							},
						},
					},
				},
			}

			funcDecl.Body.List[i] = beginStmt

		} else if pkgFuncCallExpr.Sel.Name == "Yield" {

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

func funcDeclCallsCogo(fd *ast.FuncDecl) bool {

	if fd.Body == nil || len(fd.Body.List) == 0 {
		return false
	}

	for _, stmt := range fd.Body.List {

		// Find functions calls in the style of 'cogo.ABC123()'
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
		return ok && pkgIdent.Name == "cogo"
	}

	return false
}

func filter[T any](arr []T, where func(x T) bool) []T {

	out := []T{}
	for i := 0; i < len(arr); i++ {

		if !where(arr[i]) {
			continue
		}

		out = append(out, arr[i])
	}

	return out
}
