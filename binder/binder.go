package binder

import (
	"ReCT-Go-Compiler/builtins"
	"ReCT-Go-Compiler/lexer"
	"ReCT-Go-Compiler/nodes"
	"ReCT-Go-Compiler/nodes/boundnodes"
	"ReCT-Go-Compiler/print"
	"ReCT-Go-Compiler/symbols"
	"fmt"
	"os"
)

type Binder struct {
	MemberScope    Scope
	Scopes         []Scope
	ActiveScope    *Scope
	FunctionSymbol symbols.FunctionSymbol

	LabelCounter   int
	BreakLabels    []boundnodes.BoundLabel
	ContinueLabels []boundnodes.BoundLabel
}

// helpers for the label stacks
func (bin *Binder) PushLabels(breakLabel boundnodes.BoundLabel, continueLabel boundnodes.BoundLabel) {
	bin.BreakLabels = append(bin.BreakLabels, breakLabel)
	bin.ContinueLabels = append(bin.ContinueLabels, continueLabel)
}

func (bin *Binder) PopLabels() {
	bin.BreakLabels[len(bin.BreakLabels)-1] = ""
	bin.ContinueLabels[len(bin.ContinueLabels)-1] = ""
	bin.BreakLabels = bin.BreakLabels[:len(bin.BreakLabels)-1]
	bin.ContinueLabels = bin.ContinueLabels[:len(bin.ContinueLabels)-1]
}

func (bin *Binder) GetLabels() (boundnodes.BoundLabel, boundnodes.BoundLabel) {
	return bin.BreakLabels[len(bin.BreakLabels)-1],
		bin.ContinueLabels[len(bin.ContinueLabels)-1]
}

// helpers for the scope list
func (bin *Binder) PushScope(s Scope) {
	bin.Scopes = append(bin.Scopes, s)
	bin.ActiveScope = &bin.Scopes[len(bin.Scopes)-1]
}

func (bin *Binder) PopScope() {
	bin.ActiveScope = bin.ActiveScope.Parent
}

// constructor
func CreateBinder(parent Scope, functionSymbol symbols.FunctionSymbol) *Binder {
	binder := Binder{
		MemberScope:    CreateScope(&parent),
		Scopes:         make([]Scope, 0),
		FunctionSymbol: functionSymbol,
		LabelCounter:   0,
		BreakLabels:    make([]boundnodes.BoundLabel, 0),
		ContinueLabels: make([]boundnodes.BoundLabel, 0),
	}

	binder.ActiveScope = &binder.MemberScope

	if binder.FunctionSymbol.Exists {
		for _, param := range binder.FunctionSymbol.Parameters {
			binder.ActiveScope.TryDeclareSymbol(param)
		}
	}

	return &binder
}

// binder action

// <MEMBERS> -----------------------------------------------------------------

func (bin *Binder) BindFunctionDeclaration(mem nodes.FunctionDeclarationMember) {
	boundParameters := make([]symbols.ParameterSymbol, 0)

	for i, param := range mem.Parameters {
		pName := param.Identifier.Value
		pType, _ := bin.BindTypeClause(param.TypeClause)

		// check if we've registered this param name before
		for i, p := range boundParameters {
			if p.Name == pName {
				// I haven't done bound nodes yet so I just get the syntax node parameter using the same index
				line, column, length := mem.Parameters[i].Position() // Very hacky :/
				print.Error(
					"BINDER",
					print.DuplicateParameterError,
					line,
					column,
					length,
					// Kind of a hacky way of getting the values and positions needed for the error
					"a parameter with the name \"%s\" already exists for function \"%s\"!",
					pName,
					mem.Identifier.Value,
				)
				os.Exit(-1)
			}
		}

		boundParameters = append(boundParameters, symbols.CreateParameterSymbol(pName, i, pType))
	}

	returnType, exists := bin.BindTypeClause(mem.TypeClause)
	if !exists {
		returnType = builtins.Void
	}

	functionSymbol := symbols.CreateFunctionSymbol(mem.Identifier.Value, boundParameters, returnType, mem)
	if !bin.ActiveScope.TryDeclareSymbol(functionSymbol) {
		//print.PrintC(print.Red, "Function '"+functionSymbol.Name+"' could not be defined! Seems like a function with the same name alredy exists!")
		line, column, length := mem.Position()
		print.Error(
			"BINDER",
			print.DuplicateFunctionError,
			line,
			column,
			length,
			"a function with the name \"%s\" already exists! \"%s\" could not be defined!",
			functionSymbol.Name,
			functionSymbol.Name,
		)
		os.Exit(-1)
	}
}

// </MEMBERS> ----------------------------------------------------------------
// <STATEMENTS> ---------------------------------------------------------------
func (bin *Binder) BindStatement(stmt nodes.StatementNode) boundnodes.BoundStatementNode {
	result := bin.BindStatementInternal(stmt)

	// only specific expressions are allowed to be used as statements
	// like function calls and variable assignments
	if result.NodeType() == boundnodes.BoundExpressionStatement {
		exprStmt := result.(boundnodes.BoundExpressionStatementNode)
		allowed := exprStmt.Expression.NodeType() == boundnodes.BoundErrorExpression ||
			exprStmt.Expression.NodeType() == boundnodes.BoundCallExpression ||
			exprStmt.Expression.NodeType() == boundnodes.BoundTypeCallExpression ||
			exprStmt.Expression.NodeType() == boundnodes.BoundAssignmentExpression ||
			exprStmt.Expression.NodeType() == boundnodes.BoundArrayAssignmentExpression

		if !allowed {
			//print.PrintC(print.Red, "Only call and assignment expressions are allowed to be used as statements!")
			line, column, length := stmt.Position()
			print.Error(
				"BINDER",
				print.UnexpectedExpressionStatementError,
				line,
				column,
				length,
				"cannot use \"%s\" as statement, only call and assignment expressions can be used as statements!",
				exprStmt.NodeType(),
			)
			os.Exit(-1)
		}
	}

	return result
}

func (bin *Binder) BindStatementInternal(stmt nodes.StatementNode) boundnodes.BoundStatementNode {
	switch stmt.NodeType() {
	case nodes.BlockStatement:
		return bin.BindBlockStatement(stmt.(nodes.BlockStatementNode))
	case nodes.VariableDeclaration:
		return bin.BindVariableDeclaration(stmt.(nodes.VariableDeclarationStatementNode))
	case nodes.IfStatement:
		return bin.BindIfStatement(stmt.(nodes.IfStatementNode))
	case nodes.ReturnStatement:
		return bin.BindReturnStatement(stmt.(nodes.ReturnStatementNode))
	case nodes.ForStatement:
		return bin.BindForStatement(stmt.(nodes.ForStatementNode))
	case nodes.WhileStatement:
		return bin.BindWhileStatement(stmt.(nodes.WhileStatementNode))
	case nodes.FromToStatement:
		return bin.BindFromToStatement(stmt.(nodes.FromToStatementNode))
	case nodes.BreakStatement:
		return bin.BindBreakStatement(stmt.(nodes.BreakStatementNode))
	case nodes.ContinueStatement:
		return bin.BindContinueStatement(stmt.(nodes.ContinueStatementNode))
	case nodes.ExpressionStatement:
		return bin.BindExpressionStatement(stmt.(nodes.ExpressionStatementNode))
	}

	// print.PrintC(print.Red, "Unexpected statement node! Got: '"+string(stmt.NodeType())+"'")
	line, column, length := stmt.Position()
	print.Error(
		"Parser",
		print.UnknownStatementError,
		line,
		column,
		length,
		"\"%s\" Statement found. This was unexpected!",
	)
	os.Exit(-1)
	return nil
}

func (bin *Binder) BindBlockStatement(stmt nodes.BlockStatementNode) boundnodes.BoundBlockStatementNode {
	// array of our new and improved bound statements
	statements := make([]boundnodes.BoundStatementNode, 0)

	for _, statement := range stmt.Statements {
		statements = append(statements, bin.BindStatement(statement))
	}

	return boundnodes.CreateBoundBlockStatementNode(statements)
}

func (bin *Binder) BindVariableDeclaration(stmt nodes.VariableDeclarationStatementNode) boundnodes.BoundVariableDeclarationStatementNode {
	// find out if this should be a global var or not
	isGlobal := stmt.Keyword.Kind == lexer.SetKeyword
	typeClause, clauseExists := bin.BindTypeClause(stmt.TypeClause)

	var initializer boundnodes.BoundExpressionNode
	var convertedInitializer boundnodes.BoundExpressionNode
	var variableType symbols.TypeSymbol

	// if there's an initializer -> bind and use it
	if stmt.Initializer != nil {
		initializer = bin.BindExpression(stmt.Initializer)
		variableType = initializer.Type()
	}

	if clauseExists {
		variableType = typeClause
	}

	// if there's no clause but also no initializer -> throw error!
	if variableType.Name == "" && stmt.Initializer == nil {
		line, column, length := stmt.Position()
		print.Error(
			"BINDER",
			print.IllegalVariableDeclaration,
			line+1,
			column,
			length,
			"Variable declaration is neither given a type, nor an ",
		)
		os.Exit(-1)
	}

	variable := bin.BindVariableCreation(stmt.Identifier, false, isGlobal, variableType)

	if initializer != nil {
		convertedInitializer = bin.BindConversion(initializer, variableType, false)
	}

	return boundnodes.CreateBoundVariableDeclarationStatementNode(variable, convertedInitializer)
}

func (bin *Binder) BindIfStatement(stmt nodes.IfStatementNode) boundnodes.BoundIfStatementNode {
	condition := bin.BindExpression(stmt.Condition)
	convertedCondition := bin.BindConversion(condition, builtins.Bool, false)

	thenStatement := bin.BindStatement(stmt.ThenStatement)
	elseStatement := bin.BindElseClause(stmt.ElseClause)

	return boundnodes.CreateBoundIfStatementNode(convertedCondition, thenStatement, elseStatement)
}

func (bin *Binder) BindElseClause(clause nodes.ElseClauseNode) boundnodes.BoundStatementNode {
	if !clause.ClauseIsSet {
		return nil
	}

	return bin.BindStatement(clause.ElseStatement)
}

func (bin *Binder) BindThreadStatement(stmt nodes.ThreadExpressionNode) boundnodes.BoundThreadExpressionNode {
	symbol := bin.ActiveScope.TryLookupSymbol(stmt.Expression.Identifier.Value)

	if symbol == nil || symbol.SymbolType() != symbols.Function {
		line, column, length := stmt.Expression.Position()
		print.Error(
			"BINDER",
			print.UndefinedFunctionCallError,
			line+1,
			column,
			length,
			"Function \"%s\" does not exist! (THREAD)",
			stmt.Expression.Identifier.Value,
		)
		os.Exit(-1)
	}

	functionSymbol := symbol.(symbols.FunctionSymbol)

	// This technically shouldn't be possible, but never underestimate the human spirit... And shitty code.
	if len(functionSymbol.Parameters) > 0 {
		line, column, length := stmt.Expression.Position()
		print.Error(
			"BINDER",
			print.BadNumberOfParametersError,
			line,
			column,
			length,
			"type function \"%s\" expects %d arguments but got %d!",
			stmt.Expression.Identifier,
			0,
			len(functionSymbol.Parameters),
		)
		os.Exit(-1)
	}

	return boundnodes.CreateBoundThreadExpressionNode(functionSymbol)
}

func (bin *Binder) BindReturnStatement(stmt nodes.ReturnStatementNode) boundnodes.BoundReturnStatementNode {
	var expression boundnodes.BoundExpressionNode = nil

	if stmt.Expression != nil {
		expression = bin.BindExpression(stmt.Expression)
	}

	// if we're not in any function
	if !bin.FunctionSymbol.Exists {
		//print.PrintC(print.Red, "Cannot return when outside of a function!")
		line, column, length := stmt.Position()
		print.Error(
			"BINDER",
			print.OutsideReturnError,
			line,
			column,
			length,
			"cannot use \"%s\" outside of a function!",
			stmt.Keyword.Value,
		)
		os.Exit(-1)
	}

	// if we are in a function but the return type is void, and we are trying to return something
	if bin.FunctionSymbol.Exists &&
		bin.FunctionSymbol.Type.Fingerprint() == builtins.Void.Fingerprint() &&
		expression != nil {
		//print.PrintC(print.Red, "Cannot return a value inside a void function!")
		line, column, length := stmt.Position()
		print.Error(
			"BINDER",
			print.VoidReturnError,
			line,
			column,
			length,
			"cannot use \"%s\" inside of a void function!",
			stmt.Keyword.Value,
		)
		os.Exit(-1)
	}

	return boundnodes.CreateBoundReturnStatementNode(expression)
}

func (bin *Binder) BindForStatement(stmt nodes.ForStatementNode) boundnodes.BoundForStatementNode {
	bin.PushScope(CreateScope(bin.ActiveScope))

	variable := bin.BindVariableDeclaration(stmt.Initaliser)

	condition := bin.BindExpression(stmt.Condition)
	convertedCondition := bin.BindConversion(condition, builtins.Bool, false)

	updation := bin.BindStatement(stmt.Updation)

	body, breakLabel, continueLabel := bin.BindLoopBody(stmt.Statement)

	bin.PopScope()

	return boundnodes.CreateBoundForStatementNode(variable, convertedCondition, updation, body, breakLabel, continueLabel)
}

func (bin *Binder) BindLoopBody(stmt nodes.StatementNode) (boundnodes.BoundStatementNode, boundnodes.BoundLabel, boundnodes.BoundLabel) {
	bin.LabelCounter++

	breakLabel := boundnodes.BoundLabel(fmt.Sprintf("break%d", bin.LabelCounter))
	continueLabel := boundnodes.BoundLabel(fmt.Sprintf("continue%d", bin.LabelCounter))

	bin.PushLabels(breakLabel, continueLabel)
	loopBody := bin.BindStatement(stmt)
	bin.PopLabels()

	return loopBody, breakLabel, continueLabel
}

func (bin *Binder) BindWhileStatement(stmt nodes.WhileStatementNode) boundnodes.BoundWhileStatementNode {
	bin.PushScope(CreateScope(bin.ActiveScope))

	condition := bin.BindExpression(stmt.Condition)
	convertedCondition := bin.BindConversion(condition, builtins.Bool, false)

	body, breakLabel, continueLabel := bin.BindLoopBody(stmt.Statement)

	bin.PopScope()
	return boundnodes.CreateBoundWhileStatementNode(convertedCondition, body, breakLabel, continueLabel)
}

func (bin *Binder) BindFromToStatement(stmt nodes.FromToStatementNode) boundnodes.BoundFromToStatementNode {
	bin.PushScope(CreateScope(bin.ActiveScope))

	variable := bin.BindVariableCreation(stmt.Identifier, true, false, builtins.Int)
	lowerBound := bin.BindExpression(stmt.LowerBound)
	upperBound := bin.BindExpression(stmt.UpperBound)

	if lowerBound.Type().Fingerprint() != builtins.Int.Fingerprint() {
		line, column, length := stmt.LowerBound.Position()
		print.Error(
			"BINDER",
			print.UnexpectedNonIntegerValueError,
			line,
			column,
			length,
			"FromTo statement was expecting an integer value but instead got \"%s\"!\n",
			lowerBound.Type().Name,
		)
		os.Exit(-1)
	} else if upperBound.Type().Fingerprint() != builtins.Int.Fingerprint() {
		line, column, length := stmt.UpperBound.Position()
		print.Error(
			"BINDER",
			print.UnexpectedNonIntegerValueError,
			line,
			column,
			length,
			"FromTo statement was expecting an integer value but instead got \"%s\"!\n",
			upperBound.Type().Name,
		)
		os.Exit(-1)
	}

	body, breakLabel, continueLabel := bin.BindLoopBody(stmt.Statement)

	bin.PopScope()
	return boundnodes.CreateBoundFromToStatementNode(variable, lowerBound, upperBound, body, breakLabel, continueLabel)
}

func (bin *Binder) BindBreakStatement(stmt nodes.BreakStatementNode) boundnodes.BoundGotoStatementNode {
	// if we're not in any loop
	if len(bin.BreakLabels) == 0 {
		//print.PrintC(print.Red, "Cannot use break statement outside a loop!")
		line, column, length := stmt.Position()
		print.Error(
			"BINDER",
			print.OutsideBreakError,
			line,
			column,
			length,
			"cannot use \"%s\" outside of a loop!",
			stmt.Keyword.Value,
		)
		os.Exit(-1)
	}

	breakLabel, _ := bin.GetLabels()
	return boundnodes.CreateBoundGotoStatementNode(breakLabel)
}

func (bin *Binder) BindContinueStatement(stmt nodes.ContinueStatementNode) boundnodes.BoundGotoStatementNode {
	// if we're not in any loop
	if len(bin.BreakLabels) == 0 {
		//print.PrintC(print.Red, "Cannot use continue statement outside a loop!")
		line, column, length := stmt.Position()
		print.Error(
			"BINDER",
			print.OutsideContinueError,
			line,
			column,
			length,
			"cannot use \"%s\" outside of a loop!",
			stmt.Keyword.Value,
		)
		os.Exit(-1)
	}

	_, continueLabel := bin.GetLabels()
	return boundnodes.CreateBoundGotoStatementNode(continueLabel)
}

func (bin *Binder) BindExpressionStatement(stmt nodes.ExpressionStatementNode) boundnodes.BoundExpressionStatementNode {
	expression := bin.BindExpression(stmt.Expression)
	return boundnodes.CreateBoundExpressionStatementNode(expression)
}

// </STATEMENTS> --------------------------------------------------------------
// <EXPRESSIONS> --------------------------------------------------------------

func (bin *Binder) BindExpression(expr nodes.ExpressionNode) boundnodes.BoundExpressionNode {
	switch expr.NodeType() {
	case nodes.LiteralExpression:
		return bin.BindLiteralExpression(expr.(nodes.LiteralExpressionNode))
	case nodes.ParenthesisedExpression:
		return bin.BindParenthesisedExpression(expr.(nodes.ParenthesisedExpressionNode))
	case nodes.NameExpression:
		return bin.BindNameExpression(expr.(nodes.NameExpressionNode))
	case nodes.AssignmentExpression:
		return bin.BindAssignmentExpression(expr.(nodes.AssignmentExpressionNode))
	case nodes.VariableEditorExpression:
		return bin.BindVariableEditorExpression(expr.(nodes.VariableEditorExpressionNode))
	case nodes.ArrayAccessExpression:
		return bin.BindArrayAccessExpression(expr.(nodes.ArrayAccessExpressionNode))
	case nodes.ArrayAssignmentExpression:
		return bin.BindArrayAssignmentExpression(expr.(nodes.ArrayAssignmentExpressionNode))
	case nodes.ThreadExpression: // :(  // :) - RedCube
		return bin.BindThreadStatement(expr.(nodes.ThreadExpressionNode))
	case nodes.MakeArrayExpression:
		return bin.BindMakeArrayExpression(expr.(nodes.MakeArrayExpressionNode))
	case nodes.CallExpression:
		return bin.BindCallExpression(expr.(nodes.CallExpressionNode))
	case nodes.UnaryExpression:
		return bin.BindUnaryExpression(expr.(nodes.UnaryExpressionNode))
	case nodes.TypeCallExpression:
		return bin.BindTypeCallExpression(expr.(nodes.TypeCallExpressionNode))
	case nodes.BinaryExpression:
		return bin.BindBinaryExpression(expr.(nodes.BinaryExpressionNode))
	case nodes.TernaryExpression:
		return bin.BindTernaryExpression(expr.(nodes.TernaryExpressionNode))

	default:
		//print.PrintC(print.Red, "Not implemented!")
		line, column, length := expr.Position()
		print.Error(
			"BINDER",
			print.NotImplementedError,
			line,
			column,
			length,
			"\"%s\" is not implemented yet! (cringe)",
			expr.NodeType(),
		)
		os.Exit(-1)
		return nil
	}
}

func (bin *Binder) BindLiteralExpression(expr nodes.LiteralExpressionNode) boundnodes.BoundLiteralExpressionNode {
	return boundnodes.CreateBoundLiteralExpressionNode(expr.LiteralValue)
}

func (bin *Binder) BindParenthesisedExpression(expr nodes.ParenthesisedExpressionNode) boundnodes.BoundExpressionNode {
	return bin.BindExpression(expr.Expression)
}

func (bin *Binder) BindNameExpression(expr nodes.NameExpressionNode) boundnodes.BoundExpressionNode {
	symbol := bin.ActiveScope.TryLookupSymbol(expr.Identifier.Value)
	if symbol == nil || symbol.SymbolType() != symbols.Function {
		variable := bin.BindVariableReference(expr.Identifier.Value)
		return boundnodes.CreateBoundVariableExpressionNode(variable)
	} else {
		functionSymbol := symbol.(symbols.FunctionSymbol)
		return boundnodes.CreateBoundFunctionExpressionNode(functionSymbol)
	}

}

func (bin *Binder) BindAssignmentExpression(expr nodes.AssignmentExpressionNode) boundnodes.BoundAssignmentExpressionNode {
	variable := bin.BindVariableReference(expr.Identifier.Value)
	expression := bin.BindExpression(expr.Expression)
	convertedExpression := bin.BindConversion(expression, variable.VarType(), false)

	return boundnodes.CreateBoundAssignmentExpressionNode(variable, convertedExpression)
}

func (bin *Binder) BindVariableEditorExpression(expr nodes.VariableEditorExpressionNode) boundnodes.BoundAssignmentExpressionNode {
	// bind the variable
	variable := bin.BindVariableReference(expr.Identifier.Value)

	// create a placeholder expression of value 1
	var expression boundnodes.BoundExpressionNode = boundnodes.CreateBoundLiteralExpressionNode(1)

	// if we have an expression given, use it instead
	if expr.Expression != nil {
		expression = bin.BindExpression(expr.Expression)
	}

	binaryExpression := bin.BindBinaryExpressionInternal(
		boundnodes.CreateBoundVariableExpressionNode(variable),
		expression,
		expr.Operator.Kind,
	)

	// return it as an assignment
	return boundnodes.CreateBoundAssignmentExpressionNode(variable, binaryExpression)
}

func (bin *Binder) BindArrayAccessExpression(expr nodes.ArrayAccessExpressionNode) boundnodes.BoundArrayAccessExpressionNode {
	// bind the value
	baseExpression := bin.BindExpression(expr.Base)

	// check if the variable is an array
	if baseExpression.Type().Name != "array" {
		// TODO(Tokorv): hey could you add some cool errors? i have no idea how the error system works lol
		print.PrintCF(print.Red, "Trying to Array access non-Array type (%s)", baseExpression.Type().Name)
		os.Exit(-1)
	}

	// bind the index expression
	index := bin.BindExpression(expr.Index)

	// return it as an assignment
	return boundnodes.CreateBoundArrayAccessExpressionNode(baseExpression, index)
}

func (bin *Binder) BindArrayAssignmentExpression(expr nodes.ArrayAssignmentExpressionNode) boundnodes.BoundArrayAssignmentExpressionNode {
	// bind the value
	baseExpression := bin.BindExpression(expr.Base)

	// check if the variable is an array
	if baseExpression.Type().Name != "array" {
		// TODO(Tokorv): hey could you add some cool errors? i have no idea how the error system works lol #2
		print.PrintCF(print.Red, "Trying to Array access non-Array type (%s)", baseExpression.Type().Name)
		os.Exit(-1)
	}

	// bind the index expression
	index := bin.BindExpression(expr.Index)

	// bind the value
	value := bin.BindExpression(expr.Value)

	// check if the value matches the array's type
	if value.Type().Fingerprint() != baseExpression.Type().SubTypes[0].Fingerprint() {
		// TODO(Tokorv): hey could you add some cool errors? i have no idea how the error system works lol #3
		print.PrintCF(print.Red, "Array assignment types dont match! (trying to put %s into %s-Array)", value.Type().Name, baseExpression.Type().SubTypes[0].Name)
		os.Exit(-1)
	}

	// return it as an assignment
	return boundnodes.CreateBoundArrayAssignmentExpressionNode(baseExpression, index, value)
}

func (bin *Binder) BindMakeArrayExpression(expr nodes.MakeArrayExpressionNode) boundnodes.BoundMakeArrayExpressionNode {
	// resolve the type symbol
	baseType, _ := LookupType(expr.Type, false)

	// bind the length expression
	length := bin.BindExpression(expr.Length)

	// return the bound node
	return boundnodes.CreateBoundMakeArrayExpressionNode(baseType, length)
}

func (bin *Binder) BindTypeCallExpression(expr nodes.TypeCallExpressionNode) boundnodes.BoundTypeCallExpressionNode {
	baseExpression := bin.BindExpression(expr.Base)

	// This line will error out because CallIdentifier.RealValue is nil (interface)
	// I've replaced it with CallIdentifier.Value which seems to do the trick.
	function := bin.LookupTypeFunction(expr.CallIdentifier.Value, baseExpression.Type()) // Should be a string anyway
	if function.OriginType.Name != baseExpression.Type().Name {
		line, column, length := expr.Position()
		print.Error(
			"BINDER",
			print.IncorrectTypeFunctionCallError,
			line,
			column,
			length,
			"the use of builtin function \"%s\" on \"%s\" datatype is undefined!",
			function.Name,
			baseExpression.Type().Name,
		)
		os.Exit(-1)
	}

	// bind all given arguments
	boundArguments := make([]boundnodes.BoundExpressionNode, 0)
	for _, arg := range expr.Arguments {
		boundArg := bin.BindExpression(arg)
		boundArguments = append(boundArguments, boundArg)
	}

	// make sure we got the right number of arguments
	if len(boundArguments) != len(function.Parameters) {
		//print.PrintCF(print.Red, "Type function '%s' expects %d arguments, got %d!", function.Name, len(function.Parameters), len(boundArguments))
		line, column, length := expr.Position()
		print.Error(
			"BINDER",
			print.BadNumberOfParametersError,
			line,
			column,
			length,
			"type function \"%s\" expects %d arguments but got %d!",
			function.Name,
			len(function.Parameters),
			len(boundArguments),
		)
		os.Exit(-1)
	}

	// make sure all arguments are the right type
	for i, arg := range boundArguments {
		boundArguments[i] = bin.BindConversion(arg, function.Parameters[i].VarType(), false)
	}

	return boundnodes.CreateBoundTypeCallExpressionNode(baseExpression, function, boundArguments)
}

func (bin *Binder) BindCallExpression(expr nodes.CallExpressionNode) boundnodes.BoundExpressionNode {
	// check if this is a cast
	// -----------------------

	// check if it's a primitive cast
	typeSymbol, exists := LookupPrimitiveType(expr.Identifier.Value, true)

	// if it worked -> create a primitive conversion
	if exists && len(expr.Arguments) == 1 {
		// bind the expression and return a conversion
		expression := bin.BindExpression(expr.Arguments[0])
		return bin.BindConversion(expression, typeSymbol, true)
	}

	// check if it's a complex cast
	complexTypeSymbol, exists := LookupType(expr.CastingType, true)

	// if it worked -> create a complex conversion
	if exists && len(expr.Arguments) == 1 {
		// bind the expression and return a conversion
		expression := bin.BindExpression(expr.Arguments[0])
		return bin.BindConversion(expression, complexTypeSymbol, true)
	}

	// normal function calling
	// -----------------------

	boundArguments := make([]boundnodes.BoundExpressionNode, 0)
	for _, arg := range expr.Arguments {
		boundArg := bin.BindExpression(arg)
		boundArguments = append(boundArguments, boundArg)
	}

	symbol := bin.ActiveScope.TryLookupSymbol(expr.Identifier.Value)
	if symbol == nil ||
		symbol.SymbolType() != symbols.Function {
		line, column, length := expr.Position()
		print.Error(
			"BINDER",
			print.UndefinedFunctionCallError,
			line+1,
			column,
			length,
			"Function \"%s\" does not exit!",
			expr.Identifier.Value,
		)
		os.Exit(-1)
	}

	functionSymbol := symbol.(symbols.FunctionSymbol)
	if len(boundArguments) != len(functionSymbol.Parameters) {
		//fmt.Printf("%sFunction '%s' expects %d arguments, got %d!%s\n", print.ERed, functionSymbol.Name, len(functionSymbol.Parameters), len(boundArguments), print.EReset)
		line, column, length := expr.Position()
		print.Error(
			"BINDER",
			print.BadNumberOfParametersError,
			line,
			column,
			length,
			"type function \"%s\" expects %d arguments but got %d!",
			expr.Identifier,
			len(functionSymbol.Parameters),
			len(expr.Arguments),
		)
		os.Exit(-1)
	}

	for i := 0; i < len(boundArguments); i++ {
		boundArguments[i] = bin.BindConversion(boundArguments[i], functionSymbol.Parameters[i].VarType(), false)
	}

	return boundnodes.CreateBoundCallExpressionNode(functionSymbol, boundArguments)
}

func (bin *Binder) BindUnaryExpression(expr nodes.UnaryExpressionNode) boundnodes.BoundUnaryExpressionNode {
	operand := bin.BindExpression(expr.Operand)
	op := boundnodes.BindUnaryOperator(expr.Operator.Kind, operand.Type())

	if !op.Exists {
		//print.PrintC(print.Red, "Unary operator '"+expr.Operator.Value+"' is not defined for type '"+operand.Type().Name+"'!")
		line, column, length := expr.Position()
		print.Error(
			"BINDER",
			print.UnaryOperatorTypeError,
			line,
			column,
			length,
			"the use of unary operator \"%s\" with type \"%s\" is undefined!",
			expr.Operator.Value,
			operand.Type().Name,
		)
		os.Exit(-1)
	}

	return boundnodes.CreateBoundUnaryExpressionNode(op, operand)
}

func (bin *Binder) BindBinaryExpression(expr nodes.BinaryExpressionNode) boundnodes.BoundBinaryExpressionNode {
	left := bin.BindExpression(expr.Left)
	right := bin.BindExpression(expr.Right)

	return bin.BindBinaryExpressionInternal(left, right, expr.Operator.Kind)
}

func (bin *Binder) BindBinaryExpressionInternal(left boundnodes.BoundExpressionNode, right boundnodes.BoundExpressionNode, opkind lexer.TokenKind) boundnodes.BoundBinaryExpressionNode {
	op := boundnodes.BindBinaryOperator(opkind, left.Type(), right.Type())

	if !op.Exists {
		// if the operation doesn't exit, see if the right side can be casted to the left
		conv := ClassifyConversion(right.Type(), left.Type())

		// if the conversion exists and its allowed to be done, do that (this also allows for explicit conversions)
		if conv.Exists && !conv.IsIdentity {
			right = bin.BindConversion(right, left.Type(), true)
			op = boundnodes.BindBinaryOperator(opkind, left.Type(), right.Type())
		}

		// now that we may or may not have converted our right value -> check the operation again#
		if !op.Exists {
			//print.PrintC(print.Red, "Binary operator '"+expr.Operator.Value+"' is not defined for types '"+left.Type().Name+"' and '"+right.Type().Name+"'!")
			line, column, length := 0, 0, 0
			print.Error(
				"BINDER",
				print.BinaryOperatorTypeError,
				line,
				column,
				length,
				"the use of binary operator \"%s\" with types \"%s\" and \"%s\" is undefined!",
				opkind,
				left.Type().Name,
				right.Type().Name,
			)
			os.Exit(-1)
		}

	}

	return boundnodes.CreateBoundBinaryExpressionNode(left, op, right)
}

func (bin *Binder) BindTernaryExpression(expr nodes.TernaryExpressionNode) boundnodes.BoundTernaryExpressionNode {
	// bind condition
	condition := bin.BindExpression(expr.Condition)

	// the condition needs to be a bool!
	if condition.Type().Fingerprint() != builtins.Bool.Fingerprint() {
		line, column, length := expr.Condition.Position()
		print.Error(
			"BINDER",
			print.BinaryOperatorTypeError,
			line,
			column,
			length,
			"Condition of ternary operation needs to be of type 'bool'!",
		)
		os.Exit(-1)
	}

	// bind the sides
	left := bin.BindExpression(expr.If)
	right := bin.BindExpression(expr.Else)

	// check if the left and right types are the same
	if left.Type().Fingerprint() != right.Type().Fingerprint() {
		line, column, length := expr.Else.Position()
		print.Error(
			"BINDER",
			print.BinaryOperatorTypeError,
			line,
			column,
			length,
			"Types of left and right side of ternary need to match!",
		)
		os.Exit(-1)
	}

	return boundnodes.CreateBoundTernaryExpressionNode(condition, left, right)
}

// </EXPRESSIONS> -------------------------------------------------------------
// <SYMBOLS> ------------------------------------------------------------------

func (bin *Binder) BindVariableCreation(id lexer.Token, isReadOnly bool, isGlobal bool, varType symbols.TypeSymbol) symbols.VariableSymbol {
	var variable symbols.VariableSymbol

	if isGlobal {
		variable = symbols.CreateGlobalVariableSymbol(id.Value, isReadOnly, varType)
	} else {
		variable = symbols.CreateLocalVariableSymbol(id.Value, isReadOnly, varType)
	}

	if !bin.ActiveScope.TryDeclareSymbol(variable) {
		//print.PrintC(print.Red, "Couldn't declare variable '"+id.Value+"'! Seems like a variable with this name has already been declared!")
		print.Error(
			"BINDER",
			print.DuplicateVariableDeclarationError,
			id.Line,
			id.Column,
			len(id.Value)+3+2+len(variable.VarType().Name), // Probably wrong, but it works - that's the tokorv guarantee
			"Variable \"%s\" could not be declared! Variable with this name has already been declared!",
			id.Value,
		)
		os.Exit(-1)
	}

	return variable
}

func (bin *Binder) BindVariableReference(name string) symbols.VariableSymbol {
	variable := bin.ActiveScope.TryLookupSymbol(name)

	if variable == nil ||
		!(variable.SymbolType() == symbols.GlobalVariable ||
			variable.SymbolType() == symbols.LocalVariable ||
			variable.SymbolType() == symbols.Parameter) {
		//print.PrintC(print.Red, "Could not find variable '"+name+"'!")
		print.Error(
			"BINDER",
			print.UndefinedVariableReferenceError,
			0,
			0, // We have no data for where this variable reference is?
			0,
			"Could not find variable \"%s\"! Are you sure it exists?",
			name,
		)
		os.Exit(-1)
	}

	return variable.(symbols.VariableSymbol)
}

// </SYMBOLS> -----------------------------------------------------------------
// <IDEK> ---------------------------------------------------------------------

func (bin *Binder) BindTypeClause(tc nodes.TypeClauseNode) (symbols.TypeSymbol, bool) {
	// if this type clause doesnt actually exist
	if !tc.ClauseIsSet {
		return symbols.TypeSymbol{}, false
	}

	typ, _ := LookupType(tc, false)
	return typ, true
}

func (bin *Binder) LookupTypeFunction(name string, baseType symbols.TypeSymbol) symbols.TypeFunctionSymbol {
	switch name {
	case "GetLength":
		if baseType.Fingerprint() == builtins.String.Fingerprint() {
			// string length
			return builtins.GetLength
		} else {
			// array length
			return builtins.GetArrayLength
		}
	case "Substring":
		return builtins.Substring
	case "Push":
		if baseType.Name == builtins.Array.Name {
			if baseType.SubTypes[0].IsObject {
				// push function for object arrays
				return builtins.Push
			} else {
				// push function for primitive arrays
				return builtins.PPush
			}
		}
	case "Start":
		return builtins.Start
	case "Join":
		return builtins.Join
	case "Kill":
		return builtins.Kill
	default:
		/*print.PrintC(
			print.Red,
			fmt.Sprintf("Could not find builtin TypeFunctionSymbol \"%s\"!", name),
		)*/
		print.Error(
			"BINDER",
			print.TypeFunctionDoesNotExistError,
			0,
			0, // needs extra data added and passed into the function
			0, // Probably wrong, but it works - that's the tokorv guarantee
			"Could not find builtin TypeFunctionSymbol \"%s\"!",
			name,
		)
		os.Exit(-1)
	}
	return symbols.TypeFunctionSymbol{}
}

// </IDEK> --------------------------------------------------------------------
// <TYPES> --------------------------------------------------------------------

func (bin *Binder) BindConversion(expr boundnodes.BoundExpressionNode, to symbols.TypeSymbol, allowExplicit bool) boundnodes.BoundExpressionNode {
	conversionType := ClassifyConversion(expr.Type(), to)

	if !conversionType.Exists {
		//print.PrintC(print.Red, "Cannot convert type '"+expr.Type().Name+"' to '"+to.Name+"'!")
		print.Error(
			"BINDER",
			print.ConversionError,
			0,
			0, // needs extra data added and passed into the function
			0, // Probably wrong, but it works - that's the tokorv guarantee
			"Cannot convert type \"%s\" to \"%s\"!",
			expr.Type().Name,
			to.Name,
		)
		os.Exit(-1)
		return boundnodes.BoundErrorExpressionNode{}
	}

	if conversionType.IsExplicit && !allowExplicit {
		//print.PrintC(print.Red, "Cannot convert type '"+expr.Type().Name+"' to '"+to.Name+"'! (An explicit conversion exists. Are you missing a cast?)")
		print.Error(
			"BINDER",
			print.ExplicitConversionError,
			0,
			0, // needs extra data added and passed into the function
			0, // Probably wrong, but it works - that's the tokorv guarantee
			"Cannot convert type \"%s\" to \"%s\"! (An explicit conversion exists. Are you missing a cast?)",
			expr.Type().Name,
			to.Name,
		)
		os.Exit(-1)
		return boundnodes.BoundErrorExpressionNode{}
	}

	if conversionType.IsIdentity {
		return expr
	}

	return boundnodes.CreateBoundConversionExpressionNode(to, expr)
}

func LookupType(typeClause nodes.TypeClauseNode, canFail bool) (symbols.TypeSymbol, bool) {
	switch typeClause.TypeIdentifier.Value {
	case "void":
		return builtins.Void, true
	case "bool":
		return builtins.Bool, true
	case "byte":
		return builtins.Byte, true
	case "int":
		return builtins.Int, true
	case "float":
		return builtins.Float, true
	case "string":
		return builtins.String, true
	case "any":
		return builtins.Any, true
	case "array":
		if len(typeClause.SubClauses) != 1 {
			line, column, length := typeClause.Position()
			print.Error(
				"BINDER",
				print.ExplicitConversionError,
				line,
				column, // needs extra data added and passed into the function
				length, // Probably wrong, but it works - that's the tokorv guarantee
				"Datatype \"%s\" takes in exactly one subtype!",
				typeClause.TypeIdentifier.Value,
			)
			os.Exit(-1)
		}

		baseType, _ := LookupType(typeClause.SubClauses[0], false)
		return symbols.CreateTypeSymbol("array", []symbols.TypeSymbol{baseType}, true), true

	default:
		if !canFail {
			line, column, length := typeClause.Position()
			print.Error(
				"BINDER",
				print.ExplicitConversionError,
				line,
				column, // needs extra data added and passed into the function
				length, // Probably wrong, but it works - that's the tokorv guarantee
				"Couldn't find datatype \"%s\"! Are you sure it exists?",
				typeClause.TypeIdentifier.Value,
			)
			os.Exit(-1)
		}

		return symbols.TypeSymbol{}, false
	}
}

func LookupPrimitiveType(name string, canFail bool) (symbols.TypeSymbol, bool) {
	switch name {
	case "void":
		return builtins.Void, true
	case "bool":
		return builtins.Bool, true
	case "byte":
		return builtins.Byte, true
	case "int":
		return builtins.Int, true
	case "float":
		return builtins.Float, true
	case "string":
		return builtins.String, true
	case "any":
		return builtins.Any, true
	default:
		if !canFail {
			//print.PrintC(print.Red, "Couldnt find Datatype '"+name+"'!")
			print.Error(
				"BINDER",
				print.ExplicitConversionError,
				0,
				0, // needs extra data added and passed into the function
				0, // Probably wrong, but it works - that's the tokorv guarantee
				"Couldn't find datatype \"%s\"! Are you sure it exists?",
				name,
			)
			os.Exit(-1)
		}

		return symbols.TypeSymbol{}, false
	}
}

// </TYPES> -------------------------------------------------------------------
