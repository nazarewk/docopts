// vim: set ts=4 sw=4 sts=4 noet:
//
// unit test for docopt_language.go
//
package docopt_language

import (
	"github.com/docopt/docopts/grammar/lexer"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func parse_usage(filename string) (*DocoptParser, error) {
	// data is []byte
	data, err := os.ReadFile(filename)
	p, err := ParserInit(data)
	if err != nil {
		return p, err
	}
	// AST is available from p.ast
	p.Parse()
	return p, err
}

var DocoptNodes map[string]DocoptNodeType

func init_DocoptNodes() {
	DocoptNodes = make(map[string]DocoptNodeType)
	for t := Root; t < Last_node_type; t++ {
		DocoptNodes[t.String()] = t
	}
}

func load_usage(t *testing.T, usage_filename string) (string, *DocoptParser, error) {
	usage_dir := "../grammar/usages/valid"
	filename := usage_dir + "/" + usage_filename
	if _, err := os.Stat(filename); err != nil {
		t.Errorf("doctop file is missing: '%s'", filename)
		return filename, nil, err
	}

	p, err := parse_usage(filename)
	if err != nil {
		t.Errorf("parse_usage failed for: %s", filename)
	} else if p.ast == nil {
		t.Errorf("ast is nil")
	}
	return filename, p, err
}

func TestParseUsages(t *testing.T) {
	filename, p, _ := load_usage(t, "docopts.docopt")

	usage_dir := filepath.Dir(filename)
	ast_dir := usage_dir + "/../ast"
	ast_file := ast_dir + "/" + strings.Replace(filepath.Base(filename), ".docopt", "_ast.yaml", 1)

	if _, err := os.Stat(ast_file); err != nil {
		t.Errorf("ast_file is missing: '%s'", ast_file)
	}

	ast_from_yaml, err := Load_ast_from_yaml(ast_file)
	if err != nil {
		t.Errorf("error reading ast yaml file: '%s'", ast_file)
	}

	init_DocoptNodes()
	Match_ast(t, ast_from_yaml, p.ast)
}

// Compare all node from AstNode and DocoptAst
func Match_ast(t *testing.T, n *AstNode, parsed *DocoptAst) bool {
	expect := DocoptNodes[n.Node]
	if parsed.Type != expect {
		t.Errorf("expected node '%s' got %v", expect, parsed.Type)
		return false
	}

	if n.Token != nil {
		if n.Token.Value != parsed.Token.Value {
			t.Errorf("expected token '%s' got %v", n.Token.Value, parsed.Token.Value)
			return false
		}
	}

	nb_children := len(n.Children)
	if nb_children != len(parsed.Children) {
		t.Errorf("expected nb_children %d got %d", nb_children, len(parsed.Children))
		return false
	}

	if nb_children > 0 {
		for i := 0; i < nb_children; i++ {
			if !Match_ast(t, n.Children[i], parsed.Children[i]) {
				return false
			}
		}
	}

	return true
}

func Test_transform_Options_section_to_map(t *testing.T) {
	_, p, _ := load_usage(t, "docopts.docopt")

	options, err := p.transform_Options_section_to_map()
	if err != nil {
		t.Errorf("transform_Options_section_to_map error: %v", err)
	}

	if len(options) == 0 {
		t.Errorf("transform_Options_section_to_map: options map has no element")
	}

	if options["-s"].Arg_count != 1 {
		t.Errorf("transform_Options_section_to_map: options[\"-s\"] (separator) as not 1 mandatory argument count")
	}

	expected := "<str>"
	if *options["--separator"].Argument_name != expected {
		t.Errorf("transform_Options_section_to_map: options[\"--separator\"].Argument_name got: %q expected %q",
			expected,
			*options["--separator"].Argument_name)
	}

	if options["-s"] != options["--separator"] {
		t.Errorf("transform_Options_section_to_map: options[\"-s\"] != options[\"--separator\"]")
	}
}

func consume_me(p *DocoptParser) (Reason, error) {
	p.current_node.AddNode(NONE_node, nil)
	var dummy Reason_Value = 33
	return Reason{dummy, true}, nil
}

func check_first_child_type(t *testing.T, n *DocoptAst, expected DocoptNodeType) bool {
	if n.Type != expected {
		t.Errorf("Children[0] wrong type: got %s expected %s", n.Type, expected)
		return false
	}
	return true
}

func Test_Consume_loop(t *testing.T) {
	data := []byte("Usage: pipo molo")
	p, err := ParserInit(data)
	if err != nil {
		t.Errorf("ParserInit failed: %s", err)
	}

	p.CreateNode(Root, nil)
	if p.ast.Type != Root {
		t.Errorf("create Root: got %s expected %s", p.ast.Type, Root)
	}

	var our_def DocoptNodeType = Last_node_type + 1
	p.Parse_def[our_def] = &Consumer_Definition{
		create_self_node: true,
		create_node:      true,
		toplevel_node:    Usage_Expr,
		consume_func:     consume_me,
	}

	reason, err := p.Consume_loop(our_def)
	if err != nil {
		t.Errorf("Consume_loop returned err: %s", err)
	}

	if reason.Value != 33 {
		t.Errorf("Consume_loop returned reason value: got %d expected %d", reason.Value, 33)
	}

	c := p.ast.Children[0]
	if check_first_child_type(t, c, our_def) {
		c2 := c.Children[0]
		check_first_child_type(t, c2, Usage_Expr)
	}
}

// helper for DRY code
type Match_func func(*DocoptAst) (bool, error)

func GetFunctionName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

func helper_ensure_matched(t *testing.T, f Match_func, node *DocoptAst) {
	matched, err := f(node)
	funcname := GetFunctionName(f)
	if err != nil {
		t.Errorf("%s: error %s", funcname, err)
	}
	if !matched {
		t.Errorf("%s: not matched node: %v", funcname, node)
	}
}

func Test_Match_Usage_node(t *testing.T) {
	// ============================================================== Usage_command
	node := &DocoptAst{
		Type: Usage_command,
		Token: &lexer.Token{
			Type:  IDENT,
			Value: "run",
			// not used in this context yet
			//Pos:        lexer.Position{Filename: "non-filename"},
			//Regex_name: "a regex",
			//State_name: "a state",
		},
	}

	if node.Type != Usage_command {
		t.Errorf("node Type error: got %s expected %s", node.Type, Usage_command)
	}

	command := "run"
	m := &MatchEngine{
		opts: DocoptOpts{},
		i:    0,
		argv: []string{command},
	}

	helper_ensure_matched(t, m.Match_Usage_node, node)
	if len(m.opts) != 1 {
		t.Errorf("Match_Usage_node: m.opts map wrong size, got %d expect %d", len(m.opts), 1)
	}
	if val, present := m.opts[command]; !present {
		t.Errorf("Match_Usage_node: map m.opts[%s] doesn't exists ", command)
	} else {
		if val != true {
			t.Errorf("Match_Usage_node: m.opts[%s] got %s expected true", command, val)
		}
	}
	if m.i != 1 {
		t.Errorf("Match_Usage_node: i should have increased got %d expected %d", m.i, 1)
	}

	// --------------------------------------- retest as Repeat-able argument
	node.Repeat = true
	m.i = 0
	// reset map
	m.opts = DocoptOpts{}
	helper_ensure_matched(t, m.Match_Usage_node, node)
	if len(m.opts) != 1 {
		t.Errorf("Match_Usage_node: m.opts map wrong size, got %d expect %d", len(m.opts), 1)
	}
	if val, present := m.opts[command]; !present {
		t.Errorf("Match_Usage_node: map m.opts[%s] doesn't exists ", command)
	} else {
		if val != 1 {
			t.Errorf("Match_Usage_node: m.opts[%s] got %s expected 1", command, val)
		}
	}
	if m.i != 1 {
		t.Errorf("Match_Usage_node: m.i should have increased got %d expected %d", m.i, 1)
	}

	// Repeat-able counted 2 times
	// another time (we rewind the m.argv index)
	m.i = 0
	helper_ensure_matched(t, m.Match_Usage_node, node)
	if val, present := m.opts[command]; !present {
		t.Errorf("Match_Usage_node: map m.opts[%s] doesn't exists ", command)
	} else {
		if val != 2 {
			t.Errorf("Match_Usage_node: m.opts[%s] got %v expected %d", command, val, 2)
		}
	}

	// ============================================================== Usage_argument
	name := "FILE"
	node = &DocoptAst{
		Type: Usage_argument,
		Token: &lexer.Token{
			Type:  IDENT,
			Value: name,
		},
	}

	m.i = 0
	// reset map
	m.opts = DocoptOpts{}
	helper_ensure_matched(t, m.Match_Usage_node, node)
	if val, present := m.opts[name]; !present {
		t.Errorf("Match_Usage_node: map m.opts[%s] doesn't exists ", name)
	} else {
		if val != m.argv[0] {
			t.Errorf("Match_Usage_node: m.opts[%s] got %v expected '%s'", name, val, m.argv[0])
		}
	}

	// -------------------------------- Repeat-able Usage_argument
	node.Repeat = true
	m.i = 0
	// reset map
	m.opts = DocoptOpts{}
	helper_ensure_matched(t, m.Match_Usage_node, node)
	if val, present := m.opts[name].([]string); !present {
		t.Errorf("Match_Usage_node: map m.opts[%s] doesn't exists ", name)
	} else {
		if len(val) != 1 {
			t.Errorf("Match_Usage_node: m.opts[%s] size got %d expected %d", name, len(val), 1)
		}
		if val[0] != m.argv[0] {
			t.Errorf("Match_Usage_node: m.opts[%s] => val[0] got %v expected %s", name, val[0], m.argv[0])
		}
	}
}

func Test_Match_Usage_node_Usage_long_option(t *testing.T) {
	option_name := "--myopt"
	m := &MatchEngine{
		opts: DocoptOpts{},
		i:    0,
		argv: []string{option_name},
	}

	// ========================================== node  Usage_long_option without child
	node := &DocoptAst{
		Type: Usage_long_option,
		Token: &lexer.Token{
			Type:  LONG,
			Value: option_name,
		},
	}
	helper_ensure_matched(t, m.Match_Usage_node, node)
	if val, present := m.opts[option_name].(bool); !present {
		t.Errorf("Match_Usage_node: m.opts[%s] doesn't exists ", option_name)
	} else {
		if !val {
			t.Errorf("Match_Usage_node: m.opts[%s] got %v expected true", option_name, val)
		}
	}

	// --------------------------------------- retest as Repeat-able Usage_long_option
	node.Repeat = true
	m.i = 0
	// reset map
	m.opts = DocoptOpts{}
	helper_ensure_matched(t, m.Match_Usage_node, node)
	if len(m.opts) != 1 {
		t.Errorf("Match_Usage_node: m.opts map wrong size, got %d expect %d", len(m.opts), 1)
	}
	if val, present := m.opts[option_name]; !present {
		t.Errorf("Match_Usage_node: map m.opts[%s] doesn't exists ", option_name)
	} else {
		if val != 1 {
			t.Errorf("Match_Usage_node: m.opts[%s] got %s expected 1", option_name, val)
		}
	}
	if m.i != 1 {
		t.Errorf("Match_Usage_node: m.i should have increased got %d expected %d", m.i, 1)
	}

	// Repeat-able counted 2 times
	// another time (we rewind the argument index)
	m.i = 0
	helper_ensure_matched(t, m.Match_Usage_node, node)
	if val, present := m.opts[option_name]; !present {
		t.Errorf("Match_Usage_node: map m.opts[%s] doesn't exists ", option_name)
	} else {
		if val != 2 {
			t.Errorf("Match_Usage_node: m.opts[%s] got %v expected %d", option_name, val, 2)
		}
	}
}

// TODO:
// ensure one Usage section
// ensure Usage matched case insensitive
// check p.options_node pointing to Options_section:
