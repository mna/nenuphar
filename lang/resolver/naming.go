package resolver

func (r *resolver) nameBlocks() {
	// walk the blocks tree, assigning a name to each. the root is '_', then 'a',
	// 'b', 'c', etc. with children appending their corresponding letter.
	root := r.root
	root.name = "_"
	for _, bdg := range root.bindings {
		bdg.BlockName = root.name
	}
	nameBlock(root)
}

func nameBlock(b *block) {
	for i, cb := range b.children {
		cb.name = b.name + letterFor(i)
		for _, bdg := range cb.bindings {
			if bdg.BlockName == "" {
				bdg.BlockName = cb.name
			}
		}
		nameBlock(cb)
	}
}

func letterFor(i int) string {
	if i < 26 {
		return string(rune(i) + 'a')
	}
	if i < 52 {
		return string(rune(i-26) + 'A')
	}
	// too many child blocks, give up naming it
	return "?"
}
