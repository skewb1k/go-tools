// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package golang

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/gopls/internal/protocol"
	"golang.org/x/tools/gopls/internal/util/safetoken"
	"golang.org/x/tools/internal/diff"
)

// replaceWithVarDeclaration reports whether we can convert between raw and interpreted
// string literals in the [start, end) range, along with a CodeAction containing the edits.
//
// Only the following conditions are true, the action in result is valid
//   - [start, end) is enclosed by a string literal
//   - if the string is interpreted string, need check whether the convert is allowed
func replaceWithVarDeclaration(req *codeActionsRequest) {
	path, _ := astutil.PathEnclosingInterval(req.pgf.File, req.start, req.end)
	var assign *ast.AssignStmt
	for _, n := range path {
		switch n := n.(type) {
		case *ast.AssignStmt:
			assign = n
		}
	}
	if len(assign.Lhs) != 1 {
		return
	}

	var indent string
	split := bytes.Split(req.pgf.Src, []byte("\n"))
	targetLineNumber := safetoken.StartPosition(req.pkg.FileSet(), assign.Pos()).Line
	firstLine := string(split[targetLineNumber-1])
	trimmed := strings.TrimSpace(firstLine)
	indent = firstLine[:strings.Index(firstLine, trimmed)]

	info := req.pkg.TypesInfo()
	var buf bytes.Buffer
	for _, expr := range assign.Lhs {
		ident, _ := expr.(*ast.Ident)
		obj := info.ObjectOf(ident)
		if obj == nil {
			return
		}

		// var <name> <type>
		fmt.Fprintf(&buf, "var %s %s\n", ident.Name, obj.Type().String())

		// <name> = <rhs expression>
		fmt.Fprintf(&buf, "%s%s = ", indent, ident.Name)
	}

	start, err := safetoken.Offset(req.pgf.Tok, assign.Pos())
	end, err := safetoken.Offset(req.pgf.Tok, assign.End())

	edits := []diff.Edit{{
		Start: start,
		End:   end,
		New:   buf.String(),
	}}
	textedits, err := protocol.EditsFromDiffEdits(req.pgf.Mapper, edits)
	if err != nil {
		panic(fmt.Sprintf("failed to convert diff.Edit to protocol.TextEdit:%v", err))
	}
	req.addEditAction("Replace := with var declaration", nil, protocol.DocumentChangeEdit(req.fh, textedits))
}
