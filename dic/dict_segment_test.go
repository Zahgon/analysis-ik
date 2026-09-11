package dic

import (
	"testing"

	"github.com/infinilabs/analysis-ik/jdk"
)

func fill(root *dictSegment, words ...string) {
	for _, word := range words {
		root.fillSegmentAll(jdk.EncodeUTF16(word))
	}
}

func TestMatchReportsWordAndPrefixTogether(t *testing.T) {
	root := newDictSegment(0)
	fill(root, "中华", "中华人民共和国")

	units := jdk.EncodeUTF16("中华人民")
	hit := root.match(units, 0, 2, nil)
	if !hit.IsMatch() {
		t.Error("中华 should match")
	}
	if !hit.IsPrefix() {
		t.Error("中华 should also be a prefix of 中华人民共和国")
	}
	if hit.IsUnmatch() {
		t.Error("a hit with any state set is not unmatched")
	}
	if got, want := hit.Begin(), 0; got != want {
		t.Errorf("Begin = %d, want %d", got, want)
	}

	// A prefix that is not itself a word.
	hit = root.match(units, 0, 3, nil)
	if hit.IsMatch() {
		t.Error("中华人 is not a word")
	}
	if !hit.IsPrefix() {
		t.Error("中华人 is a prefix")
	}

	// Nothing at all.
	if hit := root.match(jdk.EncodeUTF16("xyz"), 0, 3, nil); !hit.IsUnmatch() {
		t.Error("xyz should be unmatched")
	}
}

// A pending hit is reset and continued rather than restarted, which is how the
// CJK segmenter walks a word one character at a time.
func TestMatchWithAPendingHitContinuesTheSearch(t *testing.T) {
	root := newDictSegment(0)
	fill(root, "数据", "数据库")

	units := jdk.EncodeUTF16("数据库")
	hit := root.match(units, 0, 1, nil)
	if !hit.IsPrefix() {
		t.Fatal("数 should be a prefix")
	}

	hit = hit.matchedDictSegment.match(units, 1, 1, hit)
	if !hit.IsMatch() || !hit.IsPrefix() {
		t.Fatalf("数据 should match and be a prefix: match=%t prefix=%t", hit.IsMatch(), hit.IsPrefix())
	}
	if got, want := hit.Begin(), 0; got != want {
		t.Errorf("Begin = %d, want %d — the anchor must survive", got, want)
	}
	if got, want := hit.End(), 1; got != want {
		t.Errorf("End = %d, want %d", got, want)
	}

	hit = hit.matchedDictSegment.match(units, 2, 1, hit)
	if !hit.IsMatch() {
		t.Error("数据库 should match")
	}
	if hit.IsPrefix() {
		t.Error("数据库 has no longer word after it")
	}
}

// A node holds three children in a sorted array and then switches to a map. The
// switch has to preserve every child and keep lookups working.
func TestChildStorageMigratesFromArrayToMap(t *testing.T) {
	root := newDictSegment(0)
	fill(root, "d", "b", "a")
	if root.childrenArray == nil || root.childrenMap != nil {
		t.Fatal("three children should still live in the array")
	}
	if got, want := root.storeSize, 3; got != want {
		t.Fatalf("storeSize = %d, want %d", got, want)
	}
	// The array is kept sorted so the binary search works.
	for i := 1; i < root.storeSize; i++ {
		if root.childrenArray[i-1].nodeChar > root.childrenArray[i].nodeChar {
			t.Fatal("the child array is not sorted")
		}
	}

	fill(root, "c")
	if root.childrenArray != nil {
		t.Error("the array should be released after the switch")
	}
	if root.childrenMap == nil {
		t.Fatal("the map should have been created")
	}
	if got, want := root.storeSize, 4; got != want {
		t.Errorf("storeSize = %d, want %d", got, want)
	}
	for _, word := range []string{"a", "b", "c", "d"} {
		if !root.matchAll(jdk.EncodeUTF16(word)).IsMatch() {
			t.Errorf("%q was lost in the migration", word)
		}
	}
	// And a fifth child goes straight into the map.
	fill(root, "e")
	if !root.matchAll(jdk.EncodeUTF16("e")).IsMatch() {
		t.Error("e should match")
	}
}

// Disabling a word clears its terminal flag but keeps the nodes, so a longer
// word sharing the prefix still resolves.
func TestDisableSegmentKeepsLongerWords(t *testing.T) {
	root := newDictSegment(0)
	fill(root, "数据", "数据库")

	root.disableSegment(jdk.EncodeUTF16("数据"))
	if root.matchAll(jdk.EncodeUTF16("数据")).IsMatch() {
		t.Error("数据 should be disabled")
	}
	if !root.matchAll(jdk.EncodeUTF16("数据库")).IsMatch() {
		t.Error("数据库 should survive")
	}
	// Re-adding it turns the flag back on.
	fill(root, "数据")
	if !root.matchAll(jdk.EncodeUTF16("数据")).IsMatch() {
		t.Error("数据 should be back")
	}
}

func TestSearchChildrenFindsAndMisses(t *testing.T) {
	root := newDictSegment(0)
	fill(root, "a", "b", "c")
	if got := searchChildren(root.childrenArray, root.storeSize, 'b'); got < 0 {
		t.Error("b should be found")
	}
	if got := searchChildren(root.childrenArray, root.storeSize, 'z'); got != -1 {
		t.Errorf("z should be missing, got index %d", got)
	}
	if got := searchChildren(root.childrenArray, 0, 'a'); got != -1 {
		t.Errorf("an empty range should miss, got index %d", got)
	}
}

func TestHitStateBits(t *testing.T) {
	h := &Hit{}
	if !h.IsUnmatch() {
		t.Error("a fresh hit is unmatched")
	}
	h.SetPrefix()
	if h.IsUnmatch() || h.IsMatch() || !h.IsPrefix() {
		t.Error("a prefix-only hit is not unmatched and not a match")
	}
	h.SetMatch()
	if !h.IsMatch() || !h.IsPrefix() {
		t.Error("match and prefix coexist")
	}
	h.SetUnmatch()
	if !h.IsUnmatch() || h.IsMatch() || h.IsPrefix() {
		t.Error("SetUnmatch clears every bit")
	}
	h.SetBegin(3)
	h.SetEnd(7)
	if h.Begin() != 3 || h.End() != 7 {
		t.Errorf("span = [%d,%d), want [3,7)", h.Begin(), h.End())
	}
}
