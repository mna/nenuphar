package resolver

func (r *resolver) nameBlocks() {
	// walk the blocks tree, assigning a name to each. the root is '_', then 'a',
	// 'b', 'c', etc. with children appending their corresponding letter.
	root := r.root
	root.name = "_"
	assignBlockNames(root)
	nameBlock(root)
}

func nameBlock(b *block) {
	for i, cb := range b.children {
		cb.name = b.name + letterFor(i)
		assignBlockNames(cb)
		nameBlock(cb)
	}
}

func assignBlockNames(blk *block) {
	for _, bdg := range blk.bindings {
		if bdg.BlockName == "" {
			bdg.BlockName = blk.name
		}
	}
	for _, bdg := range blk.lbindings {
		if bdg.BlockName == "" {
			bdg.BlockName = blk.name
		}
	}
	for _, bdg := range blk.pendingLabels {
		if bdg.BlockName == "" {
			bdg.BlockName = blk.name
		}
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
