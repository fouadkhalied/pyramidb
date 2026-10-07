// Package memtable holds the sorted, in-memory write buffer of the LSM tree.
package memtable

import (
	"bytes"
	"lsmdb/internal/config"
	"math/rand"
)

const (
	maxHeight = 12 // 4^12 ≈ 16M entries before the top level stops helping
	branching = 4  // a node reaches the next level with probability 1/4 (LevelDB's choice)
)

// Compare returns <0 if a<b, 0 if a==b, >0 if a>b.
// Taking the comparator as a parameter lets us swap in an internal-key
// comparator (user key asc, sequence number desc) when we reach MVCC.
type Compare func(a, b []byte) int

// BytewiseCompare orders config lexicographically.
func BytewiseCompare(a, b []byte) int { return bytes.Compare(a, b) }

type version struct {
	seq  uint64
	kind config.Kind
	val  []byte
}

type node struct {
	key      []byte
	versions []version // append-only; newest is last
	next     []*node
}

// return the latest version
func (n *node) latest() version { return n.versions[len(n.versions)-1] }

// SkipList is a sorted map from []byte to []byte.
// It is NOT safe for concurrent use; MemTable adds the locking.
type SkipList struct {
	head   *node
	height int // levels currently in use, always >= 1
	length int
	cmp    Compare
	rnd    *rand.Rand
}

// NewSkipList creates an empty list. The seed only affects tower heights,
// never correctness, so tests can pass a fixed one.
func NewSkipList(cmp Compare, seed int64) *SkipList {
	return &SkipList{
		head:   &node{next: make([]*node, maxHeight)},
		height: 1,
		cmp:    cmp,
		rnd:    rand.New(rand.NewSource(seed)),
	}
}

// Len returns the number of distinct config.
func (s *SkipList) Len() int { return s.length }

// randomHeight flips a biased coin: height h has probability (1/4)^(h-1).
func (s *SkipList) randomHeight() int {
	h := 1
	for h < maxHeight && s.rnd.Intn(branching) == 0 {
		h++
	}
	return h
}

// findGE returns the first node whose key is >= target, or nil.
// If prev is non-nil it is filled with the predecessor at every level in use,
// which is exactly what Put needs to splice a new node in.
func (s *SkipList) findGE(target []byte, prev []*node) *node {
	x := s.head
	for level := s.height - 1; level >= 0; level-- {
		for next := x.next[level]; next != nil && s.cmp(next.key, target) < 0; next = x.next[level] {
			x = next // walk right while we haven't reached the target
		}
		if prev != nil {
			prev[level] = x // then drop down a level
		}
	}
	return x.next[0]
}

// Get returns the value for key.
func (s *SkipList) Get(key []byte) ([]byte, bool) {
	if n := s.findGE(key, nil); n != nil && s.cmp(n.key, key) == 0 {
		return n.latest().val, true
	}
	return nil, false
}

// Put inserts or overwrites key. It reports whether an existing key was replaced.
// The list keeps its own copy of key and value.
func (s *SkipList) Put(key []byte, seq uint64, kind config.Kind, value []byte) (replaced bool) {
	var prev [maxHeight]*node
	if n := s.findGE(key, prev[:]); n != nil && s.cmp(n.key, key) == 0 {
		n.versions = append(n.versions, version{
			val:  bytes.Clone(value),
			seq:  seq,
			kind: kind,
		})
		return true
	}

	h := s.randomHeight()
	for level := s.height; level < h; level++ {
		prev[level] = s.head // levels above the old height start at the head
	}
	if h > s.height {
		s.height = h
	}

	n := &node{
		key: bytes.Clone(key),

		versions: []version{{
			val:  bytes.Clone(value),
			seq:  seq,
			kind: config.KindSet,
		}},

		next: make([]*node, h)}

	for level := 0; level < h; level++ {
		n.next[level] = prev[level].next[level]
		prev[level].next[level] = n
	}
	s.length++
	return false
}

func (s *SkipList) GetAt(key []byte, snap uint64) (version, bool) {
	n := s.findGE(key, nil)
	if n == nil || s.cmp(n.key, key) != 0 {
		return version{}, false
	}
	for i := len(n.versions) - 1; i >= 0; i-- { // newest first
		if n.versions[i].seq <= snap {
			return n.versions[i], true
		}
	}
	return version{}, false
}

// Iterator walks the bottom level in key order.
type Iterator struct {
	list *SkipList
	cur  *node
}

func (s *SkipList) NewIterator() *Iterator { return &Iterator{list: s} }

func (it *Iterator) Valid() bool        { return it.cur != nil }
func (it *Iterator) Key() []byte        { return it.cur.key }
func (it *Iterator) Value() []byte      { return it.cur.latest().val }
func (it *Iterator) Next()              { it.cur = it.cur.next[0] }
func (it *Iterator) SeekToFirst()       { it.cur = it.list.head.next[0] }
func (it *Iterator) Seek(target []byte) { it.cur = it.list.findGE(target, nil) }
