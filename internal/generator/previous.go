package generator

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
)

// tablesFile is the generated file holding the lookup tables.
const tablesFile = "tables_gen.go"

// readPrevious reads back the tables the generator last wrote, and returns each
// emoji name they held with its emoji: the unmodified form where Unicode
// defines one, and otherwise the first modified form.
//
// It is how a rename is noticed. Comparing against the package's own output
// rather than fetching Unicode's previous release needs no network, and
// catches a release Unicode republishes under the same version number.
//
// A missing file means there is nothing to compare against. A file that cannot
// be read back is an error: generating anyway would silently retire any name
// Unicode has changed.
func readPrevious(dir string) (map[string]string, error) {
	path := filepath.Join(dir, tablesFile)
	src, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	prev, err := parseTables(src, path)
	if err != nil {
		return nil, fmt.Errorf("read back %s: %w", path, err)
	}
	return prev, nil
}

func parseTables(src []byte, path string) (map[string]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), path, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	values := make(map[string]ast.Expr)
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 {
				continue
			}
			values[vs.Names[0].Name] = vs.Values[0]
		}
	}

	emojiBlob, err := stringValue(values, "emojiBlob")
	if err != nil {
		return nil, err
	}
	nameBlob, err := stringValue(values, "nameBlob")
	if err != nil {
		return nil, err
	}
	names, err := spanValues(values, "baseNames", nameBlob)
	if err != nil {
		return nil, err
	}
	plain, err := spanValues(values, "baseEmoji", emojiBlob)
	if err != nil {
		return nil, err
	}
	keys, err := intValues(values, "styledKeys")
	if err != nil {
		return nil, err
	}
	styled, err := spanValues(values, "styledEmoji", emojiBlob)
	if err != nil {
		return nil, err
	}
	if len(names) != len(plain) || len(keys) != len(styled) {
		return nil, fmt.Errorf("tables disagree in length: %d names, %d emoji, %d keys, %d styled emoji",
			len(names), len(plain), len(keys), len(styled))
	}

	// Keys are sorted, so the first one for a row is its first modified form.
	for i, key := range keys {
		row := int(key >> 16)
		if row >= len(plain) {
			return nil, fmt.Errorf("styledKeys[%d] names row %d of %d", i, row, len(plain))
		}
		if plain[row] == "" {
			plain[row] = styled[i]
		}
	}

	prev := make(map[string]string, len(names))
	for i, name := range names {
		prev[name] = plain[i]
	}
	return prev, nil
}

// stringValue evaluates a constant built from concatenated string literals.
func stringValue(values map[string]ast.Expr, name string) (string, error) {
	expr, ok := values[name]
	if !ok {
		return "", fmt.Errorf("no %s", name)
	}
	var eval func(ast.Expr) (string, error)
	eval = func(e ast.Expr) (string, error) {
		switch e := e.(type) {
		case *ast.BasicLit:
			if e.Kind == token.STRING {
				return strconv.Unquote(e.Value)
			}
		case *ast.BinaryExpr:
			if e.Op == token.ADD {
				x, err := eval(e.X)
				if err != nil {
					return "", err
				}
				y, err := eval(e.Y)
				return x + y, err
			}
		}
		return "", fmt.Errorf("%s is not a concatenation of string literals", name)
	}
	return eval(expr)
}

// intValues reads an array of integer literals.
func intValues(values map[string]ast.Expr, name string) ([]uint64, error) {
	lit, ok := values[name].(*ast.CompositeLit)
	if !ok {
		return nil, fmt.Errorf("no %s array", name)
	}
	out := make([]uint64, 0, len(lit.Elts))
	for i, elt := range lit.Elts {
		n, err := intLit(elt)
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", name, i, err)
		}
		out = append(out, n)
	}
	return out, nil
}

// spanValues reads an array of spans and resolves each against its blob.
func spanValues(values map[string]ast.Expr, name, blob string) ([]string, error) {
	lit, ok := values[name].(*ast.CompositeLit)
	if !ok {
		return nil, fmt.Errorf("no %s array", name)
	}
	out := make([]string, 0, len(lit.Elts))
	for i, elt := range lit.Elts {
		pair, ok := elt.(*ast.CompositeLit)
		if !ok || len(pair.Elts) != 2 {
			return nil, fmt.Errorf("%s[%d] is not an {off, end} pair", name, i)
		}
		off, err := intLit(pair.Elts[0])
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", name, i, err)
		}
		end, err := intLit(pair.Elts[1])
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", name, i, err)
		}
		if off > end || end > uint64(len(blob)) {
			return nil, fmt.Errorf("%s[%d] spans [%d, %d) of a %d-byte blob", name, i, off, end, len(blob))
		}
		out = append(out, blob[off:end])
	}
	return out, nil
}

func intLit(e ast.Expr) (uint64, error) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return 0, fmt.Errorf("not an integer literal")
	}
	return strconv.ParseUint(lit.Value, 0, 64)
}
