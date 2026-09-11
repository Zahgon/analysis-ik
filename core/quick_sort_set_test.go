package core

import "testing"

func collect(q *quickSortSet) []string {
	var out []string
	for c := q.getHead(); c != nil; c = c.getNext() {
		out = append(out, c.getLexeme().String())
	}
	return out
}

func TestAddLexemeKeepsTheSetSorted(t *testing.T) {
	q := &quickSortSet{}
	// Deliberately out of order, including an insert that lands in the middle.
	for _, l := range []*Lexeme{
		NewLexeme(0, 2, 2, TypeCNWord),
		NewLexeme(0, 0, 2, TypeCNWord),
		NewLexeme(0, 4, 2, TypeCNWord),
		NewLexeme(0, 1, 2, TypeCNWord),
		NewLexeme(0, 0, 4, TypeCNWord),
	} {
		if !q.addLexeme(l) {
			t.Fatalf("failed to add %v", l)
		}
	}
	if got := q.getSize(); got != 5 {
		t.Fatalf("size = %d, want 5", got)
	}
	want := []int{0, 0, 1, 2, 4}
	wantLen := []int{4, 2, 2, 2, 2}
	i := 0
	for c := q.getHead(); c != nil; c = c.getNext() {
		if got := c.getLexeme().Begin(); got != want[i] {
			t.Errorf("position %d: begin = %d, want %d", i, got, want[i])
		}
		if got := c.getLexeme().Length(); got != wantLen[i] {
			t.Errorf("position %d: length = %d, want %d", i, got, wantLen[i])
		}
		i++
	}
}

// Dropping duplicates is what lets the three letter state machines run over the
// same characters without emitting the same span three times.
func TestAddLexemeDropsDuplicates(t *testing.T) {
	q := &quickSortSet{}
	q.addLexeme(NewLexeme(0, 0, 2, TypeCNWord))
	q.addLexeme(NewLexeme(0, 4, 2, TypeCNWord))
	q.addLexeme(NewLexeme(0, 2, 2, TypeCNWord))

	cases := []*Lexeme{
		NewLexeme(0, 0, 2, TypeLetter), // duplicate of the head
		NewLexeme(0, 4, 2, TypeLetter), // duplicate of the tail
		NewLexeme(0, 2, 2, TypeLetter), // duplicate in the middle
	}
	for _, l := range cases {
		if q.addLexeme(l) {
			t.Errorf("%v should have been rejected as a duplicate", l)
		}
	}
	if got := q.getSize(); got != 3 {
		t.Errorf("size = %d, want 3", got)
	}
}

func TestPollAndPeekWalkFromBothEnds(t *testing.T) {
	q := &quickSortSet{}
	if !q.isEmpty() {
		t.Error("a new set should be empty")
	}
	if q.pollFirst() != nil || q.pollLast() != nil || q.peekFirst() != nil || q.peekLast() != nil {
		t.Error("an empty set should yield nil")
	}

	for i := 0; i < 3; i++ {
		q.addLexeme(NewLexeme(0, i*2, 2, TypeCNWord))
	}
	if got := q.peekFirst().Begin(); got != 0 {
		t.Errorf("peekFirst = %d, want 0", got)
	}
	if got := q.peekLast().Begin(); got != 4 {
		t.Errorf("peekLast = %d, want 4", got)
	}
	if got := q.pollFirst().Begin(); got != 0 {
		t.Errorf("pollFirst = %d, want 0", got)
	}
	if got := q.pollLast().Begin(); got != 4 {
		t.Errorf("pollLast = %d, want 4", got)
	}
	if got := q.getSize(); got != 1 {
		t.Errorf("size = %d, want 1", got)
	}
	if got := q.pollLast().Begin(); got != 2 {
		t.Errorf("pollLast = %d, want 2", got)
	}
	if !q.isEmpty() {
		t.Error("the set should be empty again")
	}
}

func TestCellRejectsANilLexeme(t *testing.T) {
	defer func() {
		if r := recover(); r != "lexeme must not be null" {
			t.Errorf("recover() = %v", r)
		}
	}()
	newCell(nil)
}

func TestCellLinksBothWays(t *testing.T) {
	q := &quickSortSet{}
	q.addLexeme(NewLexeme(0, 0, 2, TypeCNWord))
	q.addLexeme(NewLexeme(0, 2, 2, TypeCNWord))

	head := q.getHead()
	if head.getPrev() != nil {
		t.Error("the head should have no predecessor")
	}
	if head.getNext().getPrev() != head {
		t.Error("the second cell should link back to the head")
	}
	if got := len(collect(q)); got != 2 {
		t.Errorf("walked %d cells, want 2", got)
	}
}
