package resolver

func (r *resolver) nameBlocks() {
	// find to root block, which should already be r.env at the end of a chunk
	// resolve, but just to make sure.
	for r.env != nil && r.env.parent != nil {
		r.env = r.env.parent
	}

	// walk the blocks tree, assigning a name to each. the root is '_', then 'a',
	// 'b', 'c', etc. with children appending their corresponding letter.
	nameBlock(r.env)
}

func nameBlock(b *block) {
	if b.parent == nil {
		b.name = "_"
		for _, bdg := range b.bindings {
			bdg.BlockName = b.name
		}
	}

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
