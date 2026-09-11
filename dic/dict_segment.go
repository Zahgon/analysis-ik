package dic

import (
	"sort"

	"github.com/infinilabs/analysis-ik/jdk"
)

// arrayLengthLimit is the number of children a node stores in a sorted array
// before it switches to a map. Nearly every node in a 275k-word dictionary has
// only a handful of children, so the array keeps the trie small.
const arrayLengthLimit = 3

// dictSegment is one branch of the dictionary trie: the code unit stored at
// this node, its children, and whether the path from the root spells a word.
type dictSegment struct {
	childrenMap   map[jdk.Char]*dictSegment
	childrenArray []*dictSegment

	nodeChar  jdk.Char
	storeSize int
	// nodeState is 1 when the path from the root to this node is a word, and 0
	// when it is not — or when the word has been disabled.
	nodeState int
}

func newDictSegment(nodeChar jdk.Char) *dictSegment {
	return &dictSegment{nodeChar: nodeChar}
}

// hasNextNode reports whether this node has any children.
func (d *dictSegment) hasNextNode() bool { return d.storeSize > 0 }

// matchAll matches the whole slice from its start.
func (d *dictSegment) matchAll(charArray []jdk.Char) *Hit {
	return d.match(charArray, 0, len(charArray), nil)
}

// match walks length code units from begin, one per level.
//
// searchHit is reused across calls so a partially matched word can be carried
// forward by the segmenters: a nil hit starts a fresh search anchored at begin,
// and a non-nil one is reset to unmatched and continued.
func (d *dictSegment) match(charArray []jdk.Char, begin, length int, searchHit *Hit) *Hit {
	if searchHit == nil {
		searchHit = &Hit{}
		searchHit.SetBegin(begin)
	} else {
		searchHit.SetUnmatch()
	}
	searchHit.SetEnd(begin)

	keyChar := charArray[begin]
	var ds *dictSegment

	// Read the containers into locals so a concurrent migration from array to
	// map cannot be observed half-done.
	segmentArray := d.childrenArray
	segmentMap := d.childrenMap

	switch {
	case segmentArray != nil:
		if position := searchChildren(segmentArray, d.storeSize, keyChar); position >= 0 {
			ds = segmentArray[position]
		}
	case segmentMap != nil:
		ds = segmentMap[keyChar]
	}

	if ds != nil {
		switch {
		case length > 1:
			return ds.match(charArray, begin+1, length-1, searchHit)
		case length == 1:
			if ds.nodeState == 1 {
				searchHit.SetMatch()
			}
			if ds.hasNextNode() {
				searchHit.SetPrefix()
				searchHit.matchedDictSegment = ds
			}
			return searchHit
		}
	}
	return searchHit
}

// fillSegmentAll adds a word to the trie.
func (d *dictSegment) fillSegmentAll(charArray []jdk.Char) {
	d.fillSegment(charArray, 0, len(charArray), 1)
}

// disableSegment marks a word as not-a-word without removing its nodes.
func (d *dictSegment) disableSegment(charArray []jdk.Char) {
	d.fillSegment(charArray, 0, len(charArray), 0)
}

func (d *dictSegment) fillSegment(charArray []jdk.Char, begin, length, enabled int) {
	keyChar := charArray[begin]
	ds := d.lookforSegment(keyChar, enabled)
	if ds == nil {
		return
	}
	switch {
	case length > 1:
		ds.fillSegment(charArray, begin+1, length-1, enabled)
	case length == 1:
		ds.nodeState = enabled
	}
}

// lookforSegment finds the child holding keyChar, creating it when create is 1.
func (d *dictSegment) lookforSegment(keyChar jdk.Char, create int) *dictSegment {
	var ds *dictSegment

	if d.storeSize <= arrayLengthLimit {
		segmentArray := d.getChildrenArray()
		if position := searchChildren(segmentArray, d.storeSize, keyChar); position >= 0 {
			ds = segmentArray[position]
		}

		if ds == nil && create == 1 {
			ds = newDictSegment(keyChar)
			if d.storeSize < arrayLengthLimit {
				segmentArray[d.storeSize] = ds
				d.storeSize++
				sortChildren(segmentArray[:d.storeSize])
			} else {
				// The array is full: move its children into a map, add the new
				// child there, and only then release the array.
				segmentMap := d.getChildrenMap()
				migrate(segmentArray, segmentMap)
				segmentMap[keyChar] = ds
				d.storeSize++
				d.childrenArray = nil
			}
		}
		return ds
	}

	segmentMap := d.getChildrenMap()
	ds = segmentMap[keyChar]
	if ds == nil && create == 1 {
		ds = newDictSegment(keyChar)
		segmentMap[keyChar] = ds
		d.storeSize++
	}
	return ds
}

func (d *dictSegment) getChildrenArray() []*dictSegment {
	if d.childrenArray == nil {
		d.childrenArray = make([]*dictSegment, arrayLengthLimit)
	}
	return d.childrenArray
}

func (d *dictSegment) getChildrenMap() map[jdk.Char]*dictSegment {
	if d.childrenMap == nil {
		d.childrenMap = make(map[jdk.Char]*dictSegment, arrayLengthLimit*2)
	}
	return d.childrenMap
}

func migrate(segmentArray []*dictSegment, segmentMap map[jdk.Char]*dictSegment) {
	for _, segment := range segmentArray {
		if segment != nil {
			segmentMap[segment.nodeChar] = segment
		}
	}
}

// searchChildren binary-searches the first storeSize entries of a sorted child
// array, returning the index of keyChar or -1.
func searchChildren(segmentArray []*dictSegment, storeSize int, keyChar jdk.Char) int {
	lo, hi := 0, storeSize-1
	for lo <= hi {
		mid := int(uint(lo+hi) >> 1)
		switch c := segmentArray[mid].nodeChar; {
		case c < keyChar:
			lo = mid + 1
		case c > keyChar:
			hi = mid - 1
		default:
			return mid
		}
	}
	return -1
}

func sortChildren(children []*dictSegment) {
	sort.Slice(children, func(i, j int) bool {
		return children[i].nodeChar < children[j].nodeChar
	})
}
