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

const cogoSwitchLbl = "cogoSwitchLbl"

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

	coroutineParamName := getCoroutineParamNameFromFuncDecl(funcDecl)
	if coroutineParamName == "" || !funcDeclCallsCoroutineBegin(funcDecl, coroutineParamName) {
		return false
	}

	beginBodyListIndex := -1
	lastCaseEndBodyListIndex := -1
	switchStmt := &ast.SwitchStmt{
		Tag: ast.NewIdent(coroutineParamName + ".State"),
		Body: &ast.BlockStmt{
			List: []ast.Stmt{},
		},
	}

	for i, stmt := range funcDecl.Body.List {

		var cogoFuncSelExpr *ast.SelectorExpr

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

		cogoFuncSelExpr, exprStmtOk = callExpr.Fun.(*ast.SelectorExpr)
		if !exprStmtOk {
			continue
		}

		if !funcCallHasLhsName(cogoFuncSelExpr, coroutineParamName) {
			continue
		}

		// cogoFuncCallLineNum := p.fset.File(cogoFuncSelExpr.Pos()).Line(cogoFuncSelExpr.Pos())

		// Now that we found a call to cogo decide what to do
		if cogoFuncSelExpr.Sel.Name == "Begin" {

			beginBodyListIndex = i
			lastCaseEndBodyListIndex = i
			continue
		} else if cogoFuncSelExpr.Sel.Name == "Yield" {

			// Add everything from the last begin/yield until this yield into a case
			stmtsSinceLastCogo := funcDecl.Body.List[lastCaseEndBodyListIndex+1 : i]

			caseStmts := make([]ast.Stmt, 0, len(stmtsSinceLastCogo)+2)
			caseStmts = append(caseStmts, stmtsSinceLastCogo...)
			caseStmts = append(caseStmts,
				&ast.IncDecStmt{
					Tok: token.INC,
					X:   ast.NewIdent(coroutineParamName + ".State"),
				},
				&ast.ReturnStmt{
					Results: callExpr.Args,
				},
			)

			switchStmt.Body.List = append(switchStmt.Body.List,
				getCaseWithStmts(
					[]ast.Expr{ast.NewIdent(fmt.Sprint(len(switchStmt.Body.List)))},
					caseStmts,
				),
			)

			lastCaseEndBodyListIndex = i

		}
	}

	// Add everything after the last yield in a separate case
	stmtsToEndOfFunc := funcDecl.Body.List[lastCaseEndBodyListIndex+1:]

	caseStmts := make([]ast.Stmt, 0, len(stmtsToEndOfFunc)+1)
	caseStmts = append(caseStmts,
		&ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent(coroutineParamName + ".State")},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{ast.NewIdent("-1")},
		},
	)
	caseStmts = append(caseStmts, stmtsToEndOfFunc...)

	switchStmt.Body.List = append(switchStmt.Body.List,
		getCaseWithStmts(
			[]ast.Expr{ast.NewIdent(fmt.Sprint(len(switchStmt.Body.List)))},
			caseStmts,
		),
	)

	// Apply changes
	funcDecl.Body.List = funcDecl.Body.List[:beginBodyListIndex]
	funcDecl.Body.List = append(funcDecl.Body.List,
		switchStmt,
	)
	return true
}

func funcDeclCallsCoroutineBegin(fd *ast.FuncDecl, coroutineParamName string) bool {

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

		if funcCallHasLhsName(pkgFuncCallExpr, coroutineParamName) {
			return true
		}
	}

	return false
}

func getCoroutineParamNameFromFuncDecl(fd *ast.FuncDecl) string {

	for _, p := range fd.Type.Params.List {

		ptrExpr, ok := p.Type.(*ast.StarExpr)
		if !ok {
			continue
		}

		// indexList because coroutine type takes multiple generic parameters, creating an indexed list
		indexListExpr, ok := ptrExpr.X.(*ast.IndexListExpr)
		if !ok {
			continue
		}

		selExpr, ok := indexListExpr.X.(*ast.SelectorExpr)
		if !ok {
			continue
		}

		if !selExprIs(selExpr, "cogo", "Coroutine") {
			continue
		}

		return p.Names[0].Name
	}

	return ""
}

// func getIdentNameFromExprOrPanic(e ast.Expr) string {
// 	return e.(*ast.Ident).Name
// }

func funcCallHasLhsName(selExpr *ast.SelectorExpr, pkgName string) bool {
	pkgIdent, ok := selExpr.X.(*ast.Ident)
	return ok && pkgIdent.Name == pkgName
}

func selExprIs(selExpr *ast.SelectorExpr, pkgName, typeName string) bool {

	pkgIdentExpr, ok := selExpr.X.(*ast.Ident)
	if !ok {
		return false
	}

	return pkgIdentExpr.Name == pkgName && selExpr.Sel.Name == typeName
}

// func filter[T any](arr []T, where func(x T) bool) []T {

// 	out := []T{}
// 	for i := 0; i < len(arr); i++ {

// 		if !where(arr[i]) {
// 			continue
// 		}

// 		out = append(out, arr[i])
// 	}

// 	return out
// }

func getCaseWithStmts(caseConditions []ast.Expr, stmts []ast.Stmt) *ast.CaseClause {
	return &ast.CaseClause{
		List: caseConditions,
		Body: stmts,
	}
}
