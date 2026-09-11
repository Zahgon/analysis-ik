package core

// quickSortSet is the ordered lexeme collection the engine builds its results
// in: a doubly linked list kept sorted by Lexeme.CompareTo, which refuses
// duplicates and inserts by walking back from the tail. Input arrives almost in
// order, so that walk is short.
type quickSortSet struct {
	head *cell
	tail *cell
	size int
}

// cell is one link of the list.
type cell struct {
	prev   *cell
	next   *cell
	lexeme *Lexeme
}

func newCell(lexeme *Lexeme) *cell {
	if lexeme == nil {
		panic("lexeme must not be null")
	}
	return &cell{lexeme: lexeme}
}

func (c *cell) compareTo(other *cell) int { return c.lexeme.CompareTo(other.lexeme) }

func (c *cell) getPrev() *cell     { return c.prev }
func (c *cell) getNext() *cell     { return c.next }
func (c *cell) getLexeme() *Lexeme { return c.lexeme }

// addLexeme inserts a lexeme in sorted position, reporting false when an equal
// one is already present.
func (q *quickSortSet) addLexeme(lexeme *Lexeme) bool {
	newCell := newCell(lexeme)
	if q.size == 0 {
		q.head = newCell
		q.tail = newCell
		q.size++
		return true
	}

	switch {
	case q.tail.compareTo(newCell) == 0:
		return false
	case q.tail.compareTo(newCell) < 0:
		q.tail.next = newCell
		newCell.prev = q.tail
		q.tail = newCell
		q.size++
		return true
	case q.head.compareTo(newCell) > 0:
		q.head.prev = newCell
		newCell.next = q.head
		q.head = newCell
		q.size++
		return true
	}

	// Somewhere in the middle: walk back from the tail until the list stops
	// sorting after the newcomer.
	index := q.tail
	for index != nil && index.compareTo(newCell) > 0 {
		index = index.prev
	}
	switch {
	case index.compareTo(newCell) == 0:
		return false
	case index.compareTo(newCell) < 0:
		newCell.prev = index
		newCell.next = index.next
		index.next.prev = newCell
		index.next = newCell
		q.size++
		return true
	}
	return false
}

// peekFirst returns the first lexeme without removing it.
func (q *quickSortSet) peekFirst() *Lexeme {
	if q.head != nil {
		return q.head.lexeme
	}
	return nil
}

// pollFirst removes and returns the first lexeme.
func (q *quickSortSet) pollFirst() *Lexeme {
	switch {
	case q.size == 1:
		first := q.head.lexeme
		q.head = nil
		q.tail = nil
		q.size--
		return first
	case q.size > 1:
		first := q.head.lexeme
		q.head = q.head.next
		q.size--
		return first
	default:
		return nil
	}
}

// peekLast returns the last lexeme without removing it.
func (q *quickSortSet) peekLast() *Lexeme {
	if q.tail != nil {
		return q.tail.lexeme
	}
	return nil
}

// pollLast removes and returns the last lexeme.
func (q *quickSortSet) pollLast() *Lexeme {
	switch {
	case q.size == 1:
		last := q.head.lexeme
		q.head = nil
		q.tail = nil
		q.size--
		return last
	case q.size > 1:
		last := q.tail.lexeme
		q.tail = q.tail.prev
		q.size--
		return last
	default:
		return nil
	}
}

// getSize returns the number of lexemes held.
func (q *quickSortSet) getSize() int { return q.size }

// isEmpty reports whether the set holds no lexemes.
func (q *quickSortSet) isEmpty() bool { return q.size == 0 }

// getHead returns the first link of the chain.
func (q *quickSortSet) getHead() *cell { return q.head }
