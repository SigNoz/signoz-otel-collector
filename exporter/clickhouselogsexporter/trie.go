package clickhouselogsexporter

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// promotedTrieNode is used to extract promoted paths in a single pass.
type promotedTrieNode struct {
	children map[string]*promotedTrieNode
	literals map[string]struct{}
	terminal bool
	fullPath string
}

func newPromotedTrieNode() *promotedTrieNode {
	return &promotedTrieNode{
		children: make(map[string]*promotedTrieNode),
		literals: make(map[string]struct{}),
	}
}

func buildPromotedTrie(paths map[string]struct{}) *promotedTrieNode {
	root := newPromotedTrieNode()
	for p := range paths {
		cur := root
		remaining := p
		for {
			// allow literal match of the remaining path at this node
			cur.literals[remaining] = struct{}{}
			dot := strings.IndexByte(remaining, '.')
			if dot == -1 {
				// last segment
				seg := remaining
				child, ok := cur.children[seg]
				if !ok {
					child = newPromotedTrieNode()
					cur.children[seg] = child
				}
				child.terminal = true
				child.fullPath = p
				break
			}
			head := remaining[:dot]
			tail := remaining[dot+1:]
			child, ok := cur.children[head]
			if !ok {
				child = newPromotedTrieNode()
				cur.children[head] = child
			}
			cur = child
			remaining = tail
		}
	}
	return root
}

// walkBodyWithTrie performs single-pass extraction based on trie; mutates body and writes into promoted.
func walkBodyWithTrie(body pcommon.Value, node *promotedTrieNode, promoted pcommon.Value) {
	if body.Type() != pcommon.ValueTypeMap || node == nil {
		return
	}
	bm := body.Map()
	pm := promoted.Map()

	// Literal matches at this level
	if len(node.literals) > 0 {
		bm.Range(func(k string, v pcommon.Value) bool {
			if _, ok := node.literals[k]; ok {
				dst := pm.PutEmpty(k)
				v.CopyTo(dst)
				bm.Remove(k)
			}
			return true
		})
	}

	// Descend into child segments
	for seg, child := range node.children {
		if child == nil {
			continue
		}
		if v, ok := bm.Get(seg); ok {
			if child.terminal {
				// Extract entire value at this key as promoted for the full path
				dst := pm.PutEmpty(child.fullPath)
				v.CopyTo(dst)
				bm.Remove(seg)
				continue
			}
			if v.Type() == pcommon.ValueTypeMap {
				subPromoted := pcommon.NewValueMap()
				walkBodyWithTrie(v, child, subPromoted)
				if subPromoted.Map().Len() > 0 {
					subPromoted.Map().Range(func(pk string, pv pcommon.Value) bool {
						dst := pm.PutEmpty(pk)
						pv.CopyTo(dst)
						return true
					})
				}
				if v.Map().Len() == 0 {
					bm.Remove(seg)
				}
			}
		}
	}
}
