// Package jsonj expands raw JSON data as per specified rules.
// JSON structures parsing based on rules described at https://www.json.org/json-en.html
package jsonj

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"
)

// RuleSet describes set of Rule to expand raw JSON data.
type RuleSet struct {
	rules map[string]*Rule
	re    *regexp.Regexp
}

func NewRuleSet(rules ...*Rule) *RuleSet {
	var set RuleSet
	for _, rule := range rules {
		set.AddRule(rule)
	}
	return &set
}

func (set *RuleSet) AddRule(rule *Rule) {
	mark := rule.mark
	if _, exists := set.rules[mark]; exists {
		panic("rule for the mark already exists: " + mark)
	}
	if set.rules == nil {
		set.rules = make(map[string]*Rule)
	}
	set.rules[mark] = rule
	set.re = nil
}

func (set *RuleSet) regexp() *regexp.Regexp {
	if set.re != nil {
		return set.re
	}

	marks := make([]string, 0, len(set.rules))
	for m := range set.rules {
		marks = append(marks, regexp.QuoteMeta(m))
	}
	// determine position of leading comma and whitespace for deletion mode
	exp := `(,[ \t\n\r]*)?"(` + strings.Join(marks, "|") + `)"[ \t\n\r]*:`
	set.re = regexp.MustCompile(exp)
	return set.re
}

// RuleMode determines Rule behavior mode
type RuleMode int

const (
	ModeUndefined RuleMode = iota
	ModeInsert
	ModeDelete
	ModeReplace
	ModeReplaceValue
)

func (i RuleMode) String() string {
	switch i {
	case ModeUndefined:
		return "Undefined"
	case ModeReplaceValue:
		return "ReplaceValue"
	case ModeReplace:
		return "Replace"
	case ModeInsert:
		return "Insert"
	case ModeDelete:
		return "Delete"
	default:
		panic("unknown mode value")
	}
}

// Pass sets number of repeats for ruleset
type Pass struct {
	RuleSet *RuleSet
	Repeats int // no less than count of marks name connectivity in RuleSet, see pet_api_example_test.go
}

type Rule struct {
	mark        string   // mark used for search and will be replaced by preparedKey
	preparedKey string   // key with quotes
	mode        RuleMode // replace, insert, delete?
	genBatch    GenerateFragmentBatchFunc
}

func (r *Rule) String() string {
	return fmt.Sprintf("%s(%s)", r.mode, r.mark)
}

func NewInsertRule(mark, key string, batchFunc GenerateFragmentBatchFunc) *Rule {
	return NewRule(ModeInsert, mark, key, batchFunc)
}

func NewReplaceRule(mark string, batchFunc GenerateFragmentBatchFunc) *Rule {
	return NewRule(ModeReplace, mark, "", batchFunc)
}

func NewReplaceValueRule(mark, key string, batchFunc GenerateFragmentBatchFunc) *Rule {
	return NewRule(ModeReplaceValue, mark, key, batchFunc)
}

func NewDeleteRule(mark string) *Rule {
	return NewRule(ModeDelete, mark, "", nil)
}

// NewRule creates new rule using specified params
// mark is searchable field and key is new key value that replaces mark
// For example, mark is '_uuid_', key is 'uuid'
func NewRule(mode RuleMode, mark, key string, batchFunc GenerateFragmentBatchFunc) *Rule {
	if mode == ModeUndefined {
		panic("mode undefined")
	}
	if mark == "" {
		panic("mark is missing")
	}
	if mark == key {
		panic("key should not be equal mark")
	}
	if mode == ModeDelete {
		return &Rule{
			mark:        mark,
			preparedKey: "",
			mode:        mode,
			genBatch:    EmptyFragmentsGenerator,
		}
	}

	if batchFunc == nil {
		panic("batchFunc is missing")
	}

	if mode != ModeReplace && key == "" {
		panic("key is missing")
	}
	key = `"` + strings.ReplaceAll(key, `"`, `\"`) + `"`
	return &Rule{
		mark:        mark,
		preparedKey: key,
		mode:        mode,
		genBatch:    batchFunc,
	}
}

// FragmentIterator allows fragments generators func iterates over json data to be replaced during a pass.
// See GenerateFragmentBatchFunc implementation examples.
type FragmentIterator interface {
	Next() bool
	Count() int
	BindParams(v interface{}) error
}

// GenerateFragmentBatchFunc returns batch of generated fragments for each of marks
type GenerateFragmentBatchFunc func(ctx context.Context, marks FragmentIterator, p interface{}) ([]interface{}, error)

// ProcessParams describes parameters of Process
type ProcessParams struct {
	Passes []Pass // the order of passes is important, see children depths at pet_api_example_test.go
	Params interface{}
}

// Process passes data changes using ProcessParams
func Process(ctx context.Context, data []byte, params ProcessParams) (b []byte, err error) {
	for _, pass := range params.Passes {
		for i := 0; i < pass.Repeats; i++ {
			data, err = doPassBatch(ctx, data, pass.RuleSet, params.Params)
			if err != nil {
				return nil, fmt.Errorf("unable to do pass %d: %v", i, err)
			}
		}
	}
	return data, nil
}

type fragEntry struct {
	rule     *Rule
	commaPos int
	markPos  int
	argsPos  int
	endPos   int
	fragment interface{}
}

func (e fragEntry) String() string {
	return fmt.Sprintf("%s at position %d", e.rule.String(), e.markPos)
}

// Marshal maybe overwritten to indent output
var Marshal = json.Marshal

// writeForInsertMode writes FRAGMENT marshaled to json.
//
// Format: `,<FRAGMENT>`
func (e *fragEntry) writeForInsertMode(w io.Writer) (int, error) {
	v := reflect.Indirect(reflect.ValueOf(e.fragment))
	if v.Kind() != reflect.Struct {
		panic("insert mode suspects Struct fragment, got " + v.String() + ": " + e.String())
	}
	enc, err := Marshal(e.fragment)
	if err != nil {
		return 0, fmt.Errorf("unable to encode fragment '%s': %v", e, err)
	}
	if bytes.Equal(enc, []byte(`{}`)) {
		return 0, nil
	}
	// trim brackets, add leading comma
	enc[0] = ','           // leading bracket replaced by comma
	enc = enc[:len(enc)-1] // closing bracket removed
	return w.Write(enc)
}

func (e *fragEntry) writeForReplaceValueMode(w io.Writer) (int, error) {
	enc, err := Marshal(e.fragment)
	if err != nil {
		return 0, fmt.Errorf("unable to encode fragment '%s': %v", e, err)
	}
	return w.Write(enc)
}

// writeForReplaceMode expects fragment to be Struct
func (e *fragEntry) writeForReplaceMode(w io.Writer) (int, error) {
	v := reflect.Indirect(reflect.ValueOf(e.fragment))
	if v.Kind() != reflect.Struct {
		panic("replace mode suspects Struct fragment, got " + v.String() + ": " + e.String())
	}
	enc, err := Marshal(e.fragment)
	if err != nil {
		return 0, fmt.Errorf("unable to encode fragment '%s': %v", e, err)
	}
	if bytes.Equal(enc, []byte(`{}`)) {
		return 0, nil
	}
	// trim brackets
	return w.Write(enc[1 : len(enc)-1])
}

type fragEntryListIter struct {
	data    []byte
	entries []*fragEntry
	idx     int
}

func newFragEntryListIter(entries []*fragEntry, data []byte) *fragEntryListIter {
	return &fragEntryListIter{
		data:    data,
		entries: entries,
		idx:     -1,
	}
}

func (iter *fragEntryListIter) Next() bool {
	iter.idx++
	return iter.idx < len(iter.entries)
}

func (iter *fragEntryListIter) Count() int {
	return len(iter.entries)
}

func (iter *fragEntryListIter) BindParams(v interface{}) error {
	entry := iter.entries[iter.idx]
	return json.Unmarshal(iter.data[entry.argsPos:entry.endPos], v)
}

// iterateMarks iterates json data using RuleSet regexp like `(,[ \n\r\t]*)?"(mark1|mark2|mark3)"[ \n\r\t]*:`
func iterateMarks(
	data []byte,
	re *regexp.Regexp,
	callback func(mark string, pos, valuePos, endPos, commaPos int),
) {
	i := 0
	for {
		// FindSubMatchIndex indexes returns indexes array:
		// , "key" : "value"
		// ^^ ^ ^  ^
		// 0^ ^ ^  1
		// 23 ^ ^
		//    4 5
		loc := re.FindSubmatchIndex(data[i:])
		if loc == nil {
			break
		}
		commaPos := -1
		if loc[2] != -1 { // prefix comma exists
			commaPos = i + loc[2]
		}
		markPos := i + loc[4] - 1         // position of "key" starts
		mark := data[i+loc[4] : i+loc[5]] // key
		i += loc[1]                       // position of "key": ends
		argsPos := i
		i += findJSONFragmentEnd(data[i:])
		endPos := i

		callback(string(mark), markPos, argsPos, endPos, commaPos)
	}
}

func doPassBatch(ctx context.Context, data []byte, set *RuleSet, flags interface{}) ([]byte, error) {
	var fragments []*fragEntry
	entriesPerRule := make(map[*Rule][]*fragEntry)

	// group marks by rules to process their batches
	iterateMarks(data, set.regexp(), func(mark string, pos, valuePos, endPos, commaPos int) {
		rule, ok := set.rules[mark]
		if !ok {
			panic("none rule specified for mark: " + mark)
		}
		n := len(fragments)
		fragments = append(fragments, &fragEntry{
			rule:     set.rules[mark],
			commaPos: commaPos,
			markPos:  pos,
			argsPos:  valuePos,
			endPos:   endPos,
		})
		entriesPerRule[rule] = append(entriesPerRule[rule], fragments[n])
	})
	if len(entriesPerRule) == 0 {
		return data, nil
	}

	// generate new fragments of each fragEntry
	for rule, list := range entriesPerRule {
		iter := newFragEntryListIter(list, data)
		result, err := rule.genBatch(ctx, iter, flags)
		if err != nil {
			return nil, fmt.Errorf("fragments generation error for rule '%s': %v", rule, err)
		}
		if len(list) != len(result) {
			panic("unexpected case")
		}
		for i := range list {
			list[i].fragment = result[i]
		}
	}

	return expandDataFragments(data, fragments)
}

// expandDataFragments returns merged old data and new fragments
func expandDataFragments(data []byte, fragments []*fragEntry) ([]byte, error) {
	var (
		b   bytes.Buffer
		pos int
	)
	for _, frag := range fragments {
		switch mode := frag.rule.mode; mode {
		case ModeReplaceValue:
			// ModeReplaceValue writes new fragment over old value:
			//  {
			//    "<preparedKey>": <FRAGMENT>
			//  }
			b.Write(data[pos:frag.markPos])
			pos = frag.endPos
			b.WriteString(frag.rule.preparedKey + `:`)  // writes `"<preparedKey>":`
			_, err := frag.writeForReplaceValueMode(&b) // writes <FRAGMENT>
			if err != nil {
				return nil, fmt.Errorf("unable to encode fragment '%s': %v", frag, err)
			}
		case ModeReplace:
			// ModeReplace writes new fragment over old mark/value pair:
			//  {
			//    <FRAGMENT>
			//  }
			b.Write(data[pos:frag.markPos])
			pos = frag.markPos
			count, err := frag.writeForReplaceMode(&b) // writes <FRAGMENT>
			if err != nil {
				return nil, fmt.Errorf("unable to encode fragment '%s': %v", frag, err)
			}
			if count == 0 { // keep old data
				b.Write(data[pos:frag.endPos])
			}
			pos = frag.endPos
		case ModeInsert:
			// ModeInsert appends fragment after value as below:
			//  {
			//    "<preparedKey>": "value",
			//    <FRAGMENT>
			//  }
			b.Write(data[pos:frag.markPos])
			pos = frag.endPos
			b.WriteString(frag.rule.preparedKey + `:`) // writes `"<preparedKey>":`
			b.Write(data[frag.argsPos:frag.endPos])    // writes `value`
			_, err := frag.writeForInsertMode(&b)      // writes `,<FRAGMENT>`
			if err != nil {
				return nil, fmt.Errorf("unable to encode fragment '%s': %v", frag, err)
			}
		case ModeDelete:
			if frag.commaPos > 0 { // leading comma exists
				b.Write(data[pos:frag.commaPos])
				pos = frag.endPos
			} else { // no leading comma exists
				b.Write(data[pos:frag.markPos])
				pos = frag.endPos
				if commaPos, found := findCommaPos(data[frag.endPos:]); found {
					pos += commaPos + 1 // skip forward comma
				}
			}
		}
	}
	b.Write(data[pos:]) // write tail
	return b.Bytes(), nil
}

var (
	nullLiteral  = []byte("null")
	trueLiteral  = []byte("true")
	falseLiteral = []byte("false")
	asciiSpace   = [256]uint8{'\t': 1, '\n': 1, '\v': 1, '\f': 1, '\r': 1, ' ': 1}
)

// findJSONFragmentEnd based on https://www.json.org/json-en.html
func findJSONFragmentEnd(data []byte) int {
	for i := 0; i < len(data); i++ {
		c := data[i]
		if asciiSpace[c] == 1 {
			continue
		}
		if c == '"' {
			return i + findJSONStringEnd(data[i:]) + 1
		}
		if c == '[' || c == '{' {
			return i + findJSONValueEnd(data[i:]) + 1
		}
		if c == '-' || ('0' <= c && c <= '9') { //nolint:gocritic
			return i + findJSONNumberEnd(data[i:])
		}
		if c == 'n' && bytes.Equal(data[i:i+len(nullLiteral)], nullLiteral) {
			return i + len(nullLiteral)
		}
		if c == 't' && bytes.Equal(data[i:i+len(trueLiteral)], trueLiteral) {
			return i + len(trueLiteral)
		}
		if c == 'f' && bytes.Equal(data[i:i+len(falseLiteral)], falseLiteral) {
			return i + len(falseLiteral)
		}
		break
	}
	panic("invalid json:\n" + string(data))
}

// findJSONStringEnd returns length of quoted prefix string.
//
// Expected format is "string".*
// For example, []byte(`"value", ...`) returns len of `"value"` (7)
func findJSONStringEnd(data []byte) int {
	for i := 1; i < len(data); i++ {
		switch data[i] {
		case '\\':
			i++ // skip next char
		case '"':
			return i
		}
	}
	panic("invalid json")
}

// findJSONNumberEnd returns length of leading json number of data bytes.
//
// Expected format is [+-0-9eE\.]+.*
// For example, []byte(`12.34, ...`) returns len of `12.34` (5)
func findJSONNumberEnd(data []byte) int {
	for i := 1; i < len(data); i++ {
		switch data[i] {
		case '+', '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'e', 'E', '.':
		default:
			return i
		}
	}
	panic("invalid json")
}

// findJSONValueEnd returns length of leading json array/object of data bytes.
//
// It expects first char is '{' or '[' and returns correspond ending literal position. For example:
// []byte(`[1,2,3], ...`) returns len of `[1,2,3]` (7)
// []byte(`{}, ...`) returns len of `{}` (2)
func findJSONValueEnd(data []byte) int {
	var end byte
	switch data[0] {
	case '{':
		end = '}'
	case '[':
		end = ']'
	}
	for c := 1; c < len(data); c++ {
		switch data[c] {
		case '"':
			c += findJSONStringEnd(data[c:])
		case '{', '[':
			c += findJSONValueEnd(data[c:])
		case end:
			return c
		}
	}
	panic("invalid json: " + string(data))
}

// findCommaPos returns first comma occurrence in data, skips only whitespaces
//
// It returns (-1, false) if not found
func findCommaPos(data []byte) (int, bool) {
	for i := 0; i < len(data); i++ {
		c := data[i]
		if asciiSpace[c] == 1 {
			continue
		}
		if c == ',' {
			return i, true
		}
		return -1, false
	}
	panic("invalid json")
}

func EmptyFragmentsGenerator(_ context.Context, iterator FragmentIterator, _ interface{}) ([]interface{}, error) {
	var entities []interface{}
	for iterator.Next() {
		entities = append(entities, struct{}{})
	}
	return entities, nil
}
