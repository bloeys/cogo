package main

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
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

	p := processor{
		fset: pkg.Fset,
	}
	for i, synFile := range pkg.Syntax {
		pkg.Syntax[i] = astutil.Apply(synFile, p.processDeclNode, nil).(*ast.File)
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

type processor struct {
	fset *token.FileSet
}

func (p *processor) processDeclNode(c *astutil.Cursor) bool {

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

	beginBodyListIndex := -1
	lastCaseEndBodyListIndex := -1
	switchStmt := &ast.SwitchStmt{
		Tag: ast.NewIdent("c.state"),
		Body: &ast.BlockStmt{
			List: []ast.Stmt{},
		},
	}

	for i, stmt := range funcDecl.Body.List {

		var cogoFuncCallExpr *ast.SelectorExpr

		// ifStmt, ifStmtOk := stmt.(*ast.IfStmt)
		// if ifStmtOk {
		// 	handleNestedCogo(ifStmt.Body)
		// 	continue
		// }

		// Find functions calls in the style of 'cogo.ABC123()'
		exprStmt, exprStmtOk := stmt.(*ast.ExprStmt)
		if !exprStmtOk {
			continue
		}

		callExpr, exprStmtOk := exprStmt.X.(*ast.CallExpr)
		if !exprStmtOk {
			continue
		}

		cogoFuncCallExpr, exprStmtOk = callExpr.Fun.(*ast.SelectorExpr)
		if !exprStmtOk {
			continue
		}

		if !funcCallHasPkgName(cogoFuncCallExpr, "cogo") {
			continue
		}

		cogoFuncCallLineNum := p.fset.File(cogoFuncCallExpr.Pos()).Line(cogoFuncCallExpr.Pos())
		fmt.Printf("Found: '%+v' at line %d\n", cogoFuncCallExpr, cogoFuncCallLineNum)

		// Now that we found a call to cogo decide what to do
		if cogoFuncCallExpr.Sel.Name == "Begin" {

			beginBodyListIndex = i
			lastCaseEndBodyListIndex = i
			continue
		} else if cogoFuncCallExpr.Sel.Name == "Yield" || cogoFuncCallExpr.Sel.Name == "End" {

			// Add everything from the last begin/yield until this yield into a case

			stmtsSinceLastCogo := funcDecl.Body.List[lastCaseEndBodyListIndex+1 : i]
			switchStmt.Body.List = append(switchStmt.Body.List, getCaseWithStmts(
				stmtsSinceLastCogo,
				[]ast.Expr{ast.NewIdent(fmt.Sprint(cogoFuncCallLineNum))},
			))

			lastCaseEndBodyListIndex = i
		}
	}

	funcDecl.Body.List = funcDecl.Body.List[:beginBodyListIndex]
	funcDecl.Body.List = append(funcDecl.Body.List, switchStmt)
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

		return funcCallHasPkgName(pkgFuncCallExpr, "cogo")
	}

	return false
}

func funcCallHasPkgName(selExpr *ast.SelectorExpr, pkgName string) bool {
	pkgIdent, ok := selExpr.X.(*ast.Ident)
	return ok && pkgIdent.Name == pkgName
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

func getCaseWithStmts(stmts []ast.Stmt, conditions []ast.Expr) *ast.CaseClause {
	return &ast.CaseClause{
		List: conditions,
		Body: stmts,
	}
}
