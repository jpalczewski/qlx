package label

// Last-resort handling of label 2/2 for content that does not fit at the
// legibility floors (input far outside the documented ranges).

// dropLevel replaces the leading breadcrumb level by "…" (the nearest levels
// matter most); the last level always stays.
func dropLevel(levels []string) ([]string, bool) {
	switch {
	case len(levels) <= 1:
		return levels, false
	case levels[0] == pairEllipsis:
		if len(levels) == 2 {
			return levels, false
		}
		return append([]string{pairEllipsis}, levels[2:]...), true
	default:
		return append([]string{pairEllipsis}, levels[1:]...), true
	}
}

// fitShortenedDesc fits the blocks at the floor size, cutting the description
// line by line (at least one line stays).
func (p *pairPlanner) fitShortenedDesc(c ctx2, px int, levels []string) ([]pairBlock, bool) {
	full := 0
	if c.desc != "" {
		specs, ok := p.contextSpecs(c, px, levels, 0, false)
		if !ok {
			return nil, false
		}
		for _, s := range specs {
			if s.kind == "desc" {
				full = len(s.lines)
			}
		}
	}
	if full == 0 {
		specs, ok := p.contextSpecs(c, px, levels, 0, false)
		return specs, ok && stackHeight(p.g, specs) <= c.avail
	}
	for dl := full; dl >= 1; dl-- {
		specs, ok := p.contextSpecs(c, px, levels, dl, false)
		if ok && stackHeight(p.g, specs) <= c.avail {
			if dl < full {
				p.warn(PairWarnDescriptionShortened)
			}
			return specs, true
		}
	}
	return nil, false
}

// lastResort handles content that does not fit at the floors, in this order:
// the description is shortened (at least one line stays), then leading
// breadcrumb levels are replaced by "…", and finally the description is
// dropped. The name and the tags are never touched here.
func (p *pairPlanner) lastResort(c ctx2) ([]pairBlock, bool) {
	px := p.g.fpx(pairLocMin)
	for pass := 0; pass < 2; pass++ {
		levels := c.levels
		for {
			if specs, ok := p.fitShortenedDesc(c, px, levels); ok {
				return p.stack(specs), true
			}
			next, ok := dropLevel(levels)
			if !ok {
				break
			}
			levels = next
			p.warn(PairWarnLocationShortened)
		}
		if c.desc == "" {
			break
		}
		c.desc = ""
		p.warn(PairWarnDescriptionShortened)
	}
	// Nothing fits: lay the content out anyway (splitting words wider than the
	// label); the caller drops tags or flags the overflow.
	specs, _ := p.contextSpecs(c, px, c.levels, 1, true)
	return p.stack(specs), false
}
