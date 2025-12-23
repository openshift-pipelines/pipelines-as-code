package matcher

import (
	"strings"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

// NodeMatcher is a predicate function that inspects a CEL AST node.
// It returns true if the node matches the criteria being searched for.
type NodeMatcher func(expr *exprpb.Expr) bool

// walkExprAST recursively walks a CEL expression AST and returns true
// if any node matches the provided matcher function.
//
// CEL AST Node Types:
//   - ConstExpr: Literal constant values (strings, numbers, bools, null, bytes)
//   - IdentExpr: Variable references (e.g., "event_type", "body")
//   - SelectExpr: Field access with dot notation (e.g., body.labels)
//   - CallExpr: Function calls and operators (e.g., size(), _[_] for bracket notation)
//   - ListExpr: List literals (e.g., [1, 2, 3])
//   - StructExpr: Map/struct literals (e.g., {"key": "value"})
//   - ComprehensionExpr: List operations (e.g., list.exists(x, condition))
func walkExprAST(expr *exprpb.Expr, matcher NodeMatcher) bool {
	if expr == nil {
		return false
	}

	// Check current node first
	if matcher(expr) {
		return true
	}

	// Recurse into children based on node type
	switch e := expr.GetExprKind().(type) {
	case *exprpb.Expr_ConstExpr:
		// Constants have no children
		return false

	case *exprpb.Expr_IdentExpr:
		// Identifiers have no children
		return false

	case *exprpb.Expr_SelectExpr:
		return walkExprAST(e.SelectExpr.GetOperand(), matcher)

	case *exprpb.Expr_CallExpr:
		if walkExprAST(e.CallExpr.GetTarget(), matcher) {
			return true
		}
		for _, arg := range e.CallExpr.GetArgs() {
			if walkExprAST(arg, matcher) {
				return true
			}
		}

	case *exprpb.Expr_ListExpr:
		for _, elem := range e.ListExpr.GetElements() {
			if walkExprAST(elem, matcher) {
				return true
			}
		}

	case *exprpb.Expr_StructExpr:
		for _, entry := range e.StructExpr.GetEntries() {
			if walkExprAST(entry.GetMapKey(), matcher) {
				return true
			}
			if walkExprAST(entry.GetValue(), matcher) {
				return true
			}
		}

	case *exprpb.Expr_ComprehensionExpr:
		comp := e.ComprehensionExpr
		if walkExprAST(comp.GetIterRange(), matcher) {
			return true
		}
		if walkExprAST(comp.GetAccuInit(), matcher) {
			return true
		}
		if walkExprAST(comp.GetLoopCondition(), matcher) {
			return true
		}
		if walkExprAST(comp.GetLoopStep(), matcher) {
			return true
		}
		if walkExprAST(comp.GetResult(), matcher) {
			return true
		}
	}

	return false
}

// matchIdentifier returns a NodeMatcher that matches IdentExpr nodes
// with the specified variable name.
func matchIdentifier(name string) NodeMatcher {
	return func(expr *exprpb.Expr) bool {
		if ident := expr.GetIdentExpr(); ident != nil {
			return ident.GetName() == name
		}
		return false
	}
}

// matchFieldAccess returns a NodeMatcher that matches SelectExpr nodes
// (dot notation field access) with any of the specified field names.
// Example: body.labels matches field "labels".
func matchFieldAccess(fieldNames ...string) NodeMatcher {
	fieldSet := make(map[string]bool, len(fieldNames))
	for _, f := range fieldNames {
		fieldSet[f] = true
	}
	return func(expr *exprpb.Expr) bool {
		if sel := expr.GetSelectExpr(); sel != nil {
			return fieldSet[sel.GetField()]
		}
		return false
	}
}

// matchBracketAccess returns a NodeMatcher that matches bracket notation
// field access (body["labels"]) with any of the specified key names.
// In CEL AST, bracket notation is represented as CallExpr with function "_[_]".
func matchBracketAccess(keyNames ...string) NodeMatcher {
	keySet := make(map[string]bool, len(keyNames))
	for _, k := range keyNames {
		keySet[k] = true
	}
	return func(expr *exprpb.Expr) bool {
		call := expr.GetCallExpr()
		if call == nil || call.GetFunction() != "_[_]" || len(call.GetArgs()) != 2 {
			return false
		}
		// Check if the key (second argument) is a string literal matching our keys
		if keyExpr := call.GetArgs()[1]; keyExpr != nil {
			if constExpr := keyExpr.GetConstExpr(); constExpr != nil {
				return keySet[constExpr.GetStringValue()]
			}
		}
		return false
	}
}

// combinedMatcher returns a NodeMatcher that returns true if any of
// the provided matchers match.
func combinedMatcher(matchers ...NodeMatcher) NodeMatcher {
	return func(expr *exprpb.Expr) bool {
		for _, m := range matchers {
			if m(expr) {
				return true
			}
		}
		return false
	}
}

// containsRefsHeadsLiteral checks if the AST contains a string literal "refs/heads/".
// This is used to determine if the user explicitly included the refs/heads/ prefix
// in their CEL expression when comparing branches.
func containsRefsHeadsLiteral(expr *exprpb.Expr) bool {
	return walkExprAST(expr, func(e *exprpb.Expr) bool {
		if constExpr := e.GetConstExpr(); constExpr != nil {
			return strings.Contains(constExpr.GetStringValue(), "refs/heads/")
		}
		return false
	})
}
