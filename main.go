package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"io"
	"math/rand"
	"os"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/imports"
)

var (
	demo = flag.Bool("demo", false, "")
)

func main() {

	flag.Parse()

	if *demo {
		runDemo()
		return
	}

	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	genCogoFuncs(cwd)
	genHasGenChecksOnOriginalFuncs(cwd)
}

func genCogoFuncs(cwd string) {

	pkgs, err := packages.Load(&packages.Config{
		Dir:   cwd,
		Mode:  packages.NeedName | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax,
		Tests: false,
	})
	if err != nil {
		panic(err)
	}

	for _, pkg := range pkgs {

		p := &processor{
			fset:             pkg.Fset,
			funcDeclsToWrite: []*ast.FuncDecl{},
		}

		for i, synFile := range pkg.Syntax {
			pkg.Syntax[i] = astutil.Apply(synFile, p.genCogoFuncsNodeProcessor, nil).(*ast.File)

			if len(p.funcDeclsToWrite) > 0 {

				root := &ast.File{
					Name:    synFile.Name,
					Imports: synFile.Imports,
					Decls:   []ast.Decl{},
				}

				for _, v := range p.funcDeclsToWrite {
					root.Decls = append(root.Decls, &ast.FuncDecl{
						Name: ast.NewIdent(v.Name.Name + "_cogo"),
						Type: v.Type,
						Body: v.Body,
					})
				}

				origFName := pkg.Fset.File(synFile.Pos()).Name()
				newFName := strings.Split(origFName, ".")[0] + ".cogo.go"
				writeAst(newFName, "// Code generated by 'cogo'; DO NOT EDIT.\n", pkg.Fset, root)

				p.funcDeclsToWrite = p.funcDeclsToWrite[:0]
			}
		}
	}
}

func genHasGenChecksOnOriginalFuncs(cwd string) {

	pkgs, err := packages.Load(&packages.Config{
		Dir:   cwd,
		Mode:  packages.NeedName | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax,
		Tests: false,
	})
	if err != nil {
		panic(err)
	}

	for _, pkg := range pkgs {

		p := &processor{
			fset:             pkg.Fset,
			funcDeclsToWrite: []*ast.FuncDecl{},
		}

		for i, synFile := range pkg.Syntax {

			pkg.Syntax[i] = astutil.Apply(synFile, p.genHasGenChecksOnOriginalFuncsNodeProcessor, nil).(*ast.File)

			if len(p.funcDeclsToWrite) > 0 {
				origFName := pkg.Fset.File(synFile.Pos()).Name()
				writeAst(origFName, "", pkg.Fset, pkg.Syntax[i])

				p.funcDeclsToWrite = p.funcDeclsToWrite[:0]
			}
		}
	}

}

type processor struct {
	fset             *token.FileSet
	funcDeclsToWrite []*ast.FuncDecl
}

func (p *processor) genCogoFuncsNodeProcessor(c *astutil.Cursor) bool {

	n := c.Node()
	if n == nil {
		return false
	}

	funcDecl, ok := n.(*ast.FuncDecl)
	if !ok || funcDecl.Body == nil {
		return true
	}

	coroutineParamName := getCoroutineParamNameFromFuncDecl(funcDecl)
	if coroutineParamName == "" {
		return false
	}

	hasBegin := funcHasSelInBody(funcDecl, coroutineParamName, "Begin")
	hasYield := funcHasSelInBody(funcDecl, coroutineParamName, "Yield")
	if !hasBegin && !hasYield {
		return false
	}

	if hasYield && !hasBegin {
		panic(fmt.Sprintf("Function '%s' in file '%s' has a 'Yield()' call but no 'Begin()'. Please ensure your coroutines have 'Begin()'", funcDecl.Name.Name, p.fset.File(funcDecl.Pos()).Name()))
	}

	beginBodyListIndex := -1
	lastCaseEndBodyListIndex := -1
	switchStmt := &ast.SwitchStmt{
		Tag: ast.NewIdent(coroutineParamName + ".State"),
		Body: &ast.BlockStmt{
			List: []ast.Stmt{},
		},
	}

	subSwitchStmt := &ast.SwitchStmt{
		Tag: ast.NewIdent(coroutineParamName + ".SubState"),
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				getCaseWithStmts(nil, []ast.Stmt{}),
			},
		},
	}

	for i := 0; i < len(funcDecl.Body.List); i++ {

		stmt := funcDecl.Body.List[i]

		var cogoFuncSelExpr *ast.SelectorExpr
		var blockStmt *ast.BlockStmt

		if ifStmt, ifStmtOk := stmt.(*ast.IfStmt); ifStmtOk {

			if ifStmtIsHasGen(ifStmt) {
				funcDecl.Body.List[i] = &ast.EmptyStmt{}
				continue
			}

			blockStmt = ifStmt.Body

		} else if forStmt, forStmtOk := stmt.(*ast.ForStmt); forStmtOk {

			outInitBlock, postInitStmt, outIfStmt, subStateNums := p.genCogoForStmt(forStmt, coroutineParamName, len(switchStmt.Body.List))

			if len(subStateNums) > 1 {
				panic("For loops currently don't support more than one yield")
			}

			if len(subStateNums) == 1 {

				// Insert post condition case
				subSwitchStmt.Body.List = append(subSwitchStmt.Body.List,
					getCaseWithStmts(
						[]ast.Expr{ast.NewIdent(fmt.Sprint(subStateNums[0]))},
						[]ast.Stmt{
							postInitStmt,
							&ast.BranchStmt{
								Tok:   token.GOTO,
								Label: ast.NewIdent(getLblNameFromSubStateNum(subStateNums[0])),
							},
						},
					),
				)

				// Insert inital condition block
				funcDecl.Body.List[i] = outInitBlock

				// // Insert lable after initial condition
				var stmtInterface ast.Stmt = &ast.LabeledStmt{
					Label: ast.NewIdent(getLblNameFromSubStateNum(subStateNums[0])),
					Stmt:  &ast.EmptyStmt{},
				}
				funcDecl.Body.List = insertIntoArr(funcDecl.Body.List, i+1, stmtInterface)

				// // Replace for with if
				stmtInterface = outIfStmt
				funcDecl.Body.List = insertIntoArr(funcDecl.Body.List, i+2, stmtInterface)
				i += 2
				continue
			}

		} else if bStmt, blockStmtOk := stmt.(*ast.BlockStmt); blockStmtOk {
			blockStmt = bStmt
		}

		if blockStmt != nil {

			subStateNums := p.genCogoBlockStmt(blockStmt, coroutineParamName, len(switchStmt.Body.List))
			for _, subStateNum := range subStateNums {

				subSwitchStmt.Body.List = append(subSwitchStmt.Body.List,
					getCaseWithStmts(
						[]ast.Expr{ast.NewIdent(fmt.Sprint(subStateNum))},
						[]ast.Stmt{
							&ast.BranchStmt{
								Tok:   token.GOTO,
								Label: ast.NewIdent(getLblNameFromSubStateNum(subStateNum)),
							},
						},
					),
				)

				var stmtToInsert ast.Stmt = &ast.LabeledStmt{
					Label: ast.NewIdent(getLblNameFromSubStateNum(subStateNum)),
					Stmt:  &ast.EmptyStmt{},
				}
				funcDecl.Body.List = insertIntoArr(funcDecl.Body.List, i+1, stmtToInsert)
				i += 1
			}

			continue
		}

		// Find functions calls in the style of 'xyz.ABC123()'
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

		if !selExprHasLhsName(cogoFuncSelExpr, coroutineParamName) {
			continue
		}

		// Now that we found a call to cogo decide what to do
		if cogoFuncSelExpr.Sel.Name == "Begin" {

			beginBodyListIndex = i
			lastCaseEndBodyListIndex = i
			continue
		} else if cogoFuncSelExpr.Sel.Name == "Yield" {

			// Add everything from the last begin/yield until this yield into a case
			stmtsSinceLastCogo := funcDecl.Body.List[lastCaseEndBodyListIndex+1 : i]

			caseStmts := make([]ast.Stmt, 0, len(stmtsSinceLastCogo)+4)

			caseStmts = append(caseStmts, subSwitchStmt)
			subSwitchStmt = &ast.SwitchStmt{
				Tag: ast.NewIdent(coroutineParamName + ".SubState"),
				Body: &ast.BlockStmt{
					List: []ast.Stmt{
						getCaseWithStmts(nil, []ast.Stmt{}),
					},
				},
			}

			caseStmts = append(caseStmts, stmtsSinceLastCogo...)
			caseStmts = append(caseStmts,
				&ast.IncDecStmt{
					Tok: token.INC,
					X:   ast.NewIdent(coroutineParamName + ".State"),
				},
				&ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent(coroutineParamName + ".SubState")},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{ast.NewIdent("-1")},
				},
				&ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent(coroutineParamName + ".Out")},
					Tok: token.ASSIGN,
					Rhs: callExpr.Args,
				},
				&ast.ReturnStmt{},
			)

			switchStmt.Body.List = append(switchStmt.Body.List,
				getCaseWithStmts(
					[]ast.Expr{ast.NewIdent(fmt.Sprint(len(switchStmt.Body.List)))},
					caseStmts,
				),
			)

			lastCaseEndBodyListIndex = i

		} else if cogoFuncSelExpr.Sel.Name == "YieldTo" {

			// Add everything from the last begin/yield until this yield into a case
			stmtsSinceLastCogo := funcDecl.Body.List[lastCaseEndBodyListIndex+1 : i]

			caseStmts := make([]ast.Stmt, 0, len(stmtsSinceLastCogo)+5)

			caseStmts = append(caseStmts, subSwitchStmt)
			subSwitchStmt = &ast.SwitchStmt{
				Tag: ast.NewIdent(coroutineParamName + ".SubState"),
				Body: &ast.BlockStmt{
					List: []ast.Stmt{
						getCaseWithStmts(nil, []ast.Stmt{}),
					},
				},
			}

			caseStmts = append(caseStmts, stmtsSinceLastCogo...)
			caseStmts = append(caseStmts,
				&ast.IncDecStmt{
					Tok: token.INC,
					X:   ast.NewIdent(coroutineParamName + ".State"),
				},
				&ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent(coroutineParamName + ".SubState")},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{ast.NewIdent("-1")},
				},
				&ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent(coroutineParamName + ".Yielder")},
					Tok: token.ASSIGN,
					Rhs: callExpr.Args,
				},
				&ast.ReturnStmt{},
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

	caseStmts = append(caseStmts, subSwitchStmt)
	subSwitchStmt = &ast.SwitchStmt{
		Tag: ast.NewIdent(coroutineParamName + ".SubState"),
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				getCaseWithStmts(nil, []ast.Stmt{}),
			},
		},
	}

	caseStmts = append(caseStmts,
		&ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent(coroutineParamName + ".State")},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{ast.NewIdent("-1")},
		},
		&ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent(coroutineParamName + ".SubState")},
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

		// default case
		getCaseWithStmts(
			nil,
			[]ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent(coroutineParamName + ".State")},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{ast.NewIdent("-1")},
				},
				&ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent(coroutineParamName + ".SubState")},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{ast.NewIdent("-1")},
				},
				&ast.ReturnStmt{},
			},
		),
	)

	// Apply changes
	funcDecl.Body.List = funcDecl.Body.List[:beginBodyListIndex]
	funcDecl.Body.List = append(funcDecl.Body.List,
		switchStmt,
	)

	originalList := funcDecl.Body.List
	funcDecl.Body.List = make([]ast.Stmt, 0, len(funcDecl.Body.List)+1)

	funcDecl.Body.List = append(funcDecl.Body.List, originalList...)

	p.funcDeclsToWrite = append(p.funcDeclsToWrite, funcDecl)

	return true
}

/*
* Init is done in a block just before the for loop condition
* Post is done in the case that jumps over the loop condition
 */

func (p *processor) genCogoForStmt(forStmt *ast.ForStmt, coroutineParamName string, currCase int) (outInitBlock *ast.BlockStmt, postInitStmt ast.Stmt, outIfStmt *ast.IfStmt, subStateNums []int32) {

	outInitBlock = &ast.BlockStmt{
		List: []ast.Stmt{
			forStmt.Init,
		},
	}

	postInitLblNum := rand.Int31() + 1000_000
	subStateNums = append(subStateNums, postInitLblNum)

	if forStmt.Post == nil {
		postInitStmt = &ast.EmptyStmt{}
	} else {
		postInitStmt = forStmt.Post
	}

	outIfStmt = &ast.IfStmt{
		Cond: forStmt.Cond,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{},
		},
	}

	forBodyList := forStmt.Body.List
	for i, stmt := range forBodyList {

		selExpr, selExprArgs := tryGetSelExprFromStmt(stmt, coroutineParamName, "Yield")
		if selExpr == nil {
			continue
		}

		forBodyList[i] = &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent(coroutineParamName + ".State")},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{ast.NewIdent(fmt.Sprint(currCase))},
				},
				&ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent(coroutineParamName + ".SubState")},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{ast.NewIdent(fmt.Sprint(postInitLblNum))},
				},
				&ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent(coroutineParamName + ".Out")},
					Tok: token.ASSIGN,
					Rhs: selExprArgs,
				},
				&ast.ReturnStmt{},
			},
		}
	}

	outIfStmt.Body.List = forBodyList

	return outInitBlock, postInitStmt, outIfStmt, subStateNums
}

func (p *processor) genCogoBlockStmt(blockStmt *ast.BlockStmt, coroutineParamName string, currCase int) (subStateNums []int32) {

	for i, stmt := range blockStmt.List {

		selExpr, selExprArgs := tryGetSelExprFromStmt(stmt, coroutineParamName, "Yield")
		if selExpr == nil {
			continue
		}

		// @TODO: Ensure that subStateNums don't get reused
		// subStateNum >= 1000_000
		newSubStateNum := rand.Int31() + 1000_000
		subStateNums = append(subStateNums, newSubStateNum)
		blockStmt.List[i] = &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent(coroutineParamName + ".State")},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{ast.NewIdent(fmt.Sprint(currCase))},
				},
				&ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent(coroutineParamName + ".SubState")},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{ast.NewIdent(fmt.Sprint(newSubStateNum))},
				},
				&ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent(coroutineParamName + ".Out")},
					Tok: token.ASSIGN,
					Rhs: selExprArgs,
				},
				&ast.ReturnStmt{},
			},
		}
	}

	return subStateNums
}

func getLblNameFromSubStateNum(subStateNum int32) string {
	return fmt.Sprint("cogo_", subStateNum)
}

func insertIntoArr[T any](a []T, index int, value T) []T {

	if len(a) == index {
		return append(a, value)
	}

	a = append(a[:index+1], a[index:]...)
	a[index] = value
	return a
}

func tryGetSelExprFromStmt(stmt ast.Stmt, lhs, rhs string) (selExpr *ast.SelectorExpr, args []ast.Expr) {

	exprStmt, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return nil, nil
	}

	callExpr, ok := exprStmt.X.(*ast.CallExpr)
	if !ok {
		return nil, nil
	}
	args = callExpr.Args

	selExpr, ok = callExpr.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, nil
	}

	if selExprIs(selExpr, lhs, rhs) {
		return selExpr, args
	}

	return nil, nil
}

func (p *processor) genHasGenChecksOnOriginalFuncsNodeProcessor(c *astutil.Cursor) bool {

	n := c.Node()
	if n == nil {
		return false
	}

	funcDecl, ok := n.(*ast.FuncDecl)
	if !ok || funcDecl.Body == nil {
		return true
	}

	coroutineParamName := getCoroutineParamNameFromFuncDecl(funcDecl)
	if coroutineParamName == "" || !funcHasSelInBody(funcDecl, coroutineParamName, "Begin") {
		return false
	}

	for i, stmt := range funcDecl.Body.List {

		// If one already exists update it and return
		ifStmt, ok := stmt.(*ast.IfStmt)
		if ok && ifStmtIsHasGen(ifStmt) {
			funcDecl.Body.List[i] = createHasGenIfStmt(funcDecl, coroutineParamName)
			p.funcDeclsToWrite = append(p.funcDeclsToWrite, funcDecl)
			return true
		}
	}

	// If the check doesn't exist add it to the beginning of the function
	origList := funcDecl.Body.List
	funcDecl.Body.List = make([]ast.Stmt, 0, len(origList)+1)
	funcDecl.Body.List = append(funcDecl.Body.List, createHasGenIfStmt(funcDecl, coroutineParamName))
	funcDecl.Body.List = append(funcDecl.Body.List, origList...)

	p.funcDeclsToWrite = append(p.funcDeclsToWrite, funcDecl)

	return true
}

func createHasGenIfStmt(funcDecl *ast.FuncDecl, coroutineParamName string) *ast.IfStmt {
	return &ast.IfStmt{
		Cond: createStmtFromSelFuncCall("cogo", "HasGen").(*ast.ExprStmt).X,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ExprStmt{
					X: &ast.CallExpr{
						Fun:  ast.NewIdent(funcDecl.Name.Name + "_cogo"),
						Args: []ast.Expr{ast.NewIdent(coroutineParamName)},
					}},
				&ast.ReturnStmt{},
			},
		},
	}
}

func ifStmtIsHasGen(stmt *ast.IfStmt) bool {

	callExpr, ok := stmt.Cond.(*ast.CallExpr)
	if !ok {
		return false
	}

	selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	return selExprIs(selExpr, "cogo", "HasGen")
}

func createStmtFromSelFuncCall(lhs, rhs string) ast.Stmt {
	return &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent(lhs),
				Sel: ast.NewIdent(rhs),
			},
		},
	}
}

func funcHasSelInBody(fd *ast.FuncDecl, selLhs, selRhs string) bool {

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

		selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}

		if selExprIs(selExpr, selLhs, selRhs) {
			return true
		}
	}

	return false
}

func getCoroutineParamNameFromFuncDecl(fd *ast.FuncDecl) string {

	// If func doesn't take one parameter then it is not a coroutine
	if len(fd.Type.Params.List) == 0 {
		return ""
	}

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

func selExprHasLhsName(selExpr *ast.SelectorExpr, pkgName string) bool {
	pkgIdent, ok := selExpr.X.(*ast.Ident)
	return ok && pkgIdent.Name == pkgName
}

func selExprIs(selExpr *ast.SelectorExpr, lhs, rhs string) bool {

	pkgIdentExpr, ok := selExpr.X.(*ast.Ident)
	if !ok {
		return false
	}

	return pkgIdentExpr.Name == lhs && selExpr.Sel.Name == rhs
}

func getCaseWithStmts(caseConditions []ast.Expr, stmts []ast.Stmt) *ast.CaseClause {
	return &ast.CaseClause{
		List: caseConditions,
		Body: stmts,
	}
}

func writeAst(fName, topComment string, fset *token.FileSet, node any) {

	f, err := os.Create(fName)
	if err != nil {
		panic("Failed to create file to write new AST. Err: " + err.Error())
	}
	defer f.Close()

	if topComment != "" {
		f.WriteString(topComment)
	}

	err = format.Node(f, fset, node)
	if err != nil {
		panic(err.Error())
	}

	b, err := imports.Process(fName, nil, nil)
	if err != nil {
		format.Node(os.Stdout, fset, node)
		panic("Failed to process imports on file " + fName + ". Err: " + err.Error())
	}

	f.Seek(0, io.SeekStart)
	_, err = f.Write(b)
	if err != nil {
		panic(err.Error())
	}
}
