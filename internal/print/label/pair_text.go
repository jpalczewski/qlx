package label

import (
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
)

// Text primitives of the paired-label engine: measuring, line breaking and
// flowing runs of mixed weight. Everything here works on integer pixels so the
// layout is deterministic.

const (
	pairFontRegular = "noto-sans"
	pairFontBold    = "noto-sans-bold"
	pairEllipsis    = "…"
)

// pairFace returns the cached face for the weight and pixel size. Embedded
// fonts always load; basicfont is only a defensive fallback (RenderPair checks
// the fonts up front).
func pairFace(bold bool, px int) font.Face {
	name := pairFontRegular
	if bold {
		name = pairFontBold
	}
	f, err := LoadFace(name, float64(px))
	if err != nil {
		return basicfont.Face7x13
	}
	return f
}

// pairRun is a piece of text drawn in one weight.
type pairRun struct {
	text string
	bold bool
}

type pairMeasureKey struct {
	bold bool
	px   int
	s    string
}

// pairMeasurer measures strings and memoises the results for one render.
// Lookups never iterate the map, so output does not depend on map order.
type pairMeasurer struct {
	cache map[pairMeasureKey]int
}

func newPairMeasurer() *pairMeasurer {
	return &pairMeasurer{cache: map[pairMeasureKey]int{}}
}

// width returns the advance of s in whole pixels (rounded up).
func (m *pairMeasurer) width(bold bool, px int, s string) int {
	k := pairMeasureKey{bold, px, s}
	if w, ok := m.cache[k]; ok {
		return w
	}
	w := font.MeasureString(pairFace(bold, px), s).Ceil()
	m.cache[k] = w
	return w
}

// space returns the width of one inter-word space in the regular weight.
func (m *pairMeasurer) space(px int) int { return m.width(false, px, " ") }

// pairIsSingle reports whether w is a single-letter Polish word (w, z, i, o,
// u, a) that must not end a line.
func pairIsSingle(w string) bool {
	switch strings.ToLower(w) {
	case "w", "z", "i", "o", "u", "a":
		return true
	}
	return false
}

// pairNormalize collapses runs of whitespace and trims the text.
func pairNormalize(s string) string { return strings.Join(strings.Fields(s), " ") }

// pairTokens splits text into unbreakable tokens: words, with every
// single-letter word glued (with a plain space) to the word that follows.
func pairTokens(text string) []string {
	ws := strings.Fields(text)
	var out []string
	for i := 0; i < len(ws); i++ {
		tok := ws[i]
		for pairIsSingle(ws[i]) && i+1 < len(ws) {
			i++
			tok += " " + ws[i]
		}
		out = append(out, tok)
	}
	return out
}

// pairTok is a unit for the greedy line breaker. brk forces a line break after
// the token (used between the pieces of a word that had to be split).
type pairTok struct {
	s   string
	brk bool
}

// splitWord cuts a word wider than maxW into pieces that each fit. It is the
// last resort of line breaking.
func (m *pairMeasurer) splitWord(bold bool, px int, w string, maxW int) []pairTok {
	var out []pairTok
	var cur []rune
	for _, r := range w {
		if len(cur) > 0 && m.width(bold, px, string(append(cur[:len(cur):len(cur)], r))) > maxW {
			out = append(out, pairTok{s: string(cur), brk: true})
			cur = nil
		}
		cur = append(cur, r)
	}
	if len(cur) > 0 {
		out = append(out, pairTok{s: string(cur)})
	}
	return out
}

// fitTokens turns text into breakable tokens that each fit maxW. Glued
// single-letter words are unglued when the glued token is too wide; a word
// that is still too wide is split only when split is true, otherwise ok is
// false.
func (m *pairMeasurer) fitTokens(bold bool, px int, text string, maxW int, split bool) (toks []pairTok, ok bool) {
	for _, t := range pairTokens(text) {
		if m.width(bold, px, t) <= maxW {
			toks = append(toks, pairTok{s: t})
			continue
		}
		for _, w := range strings.Fields(t) {
			if m.width(bold, px, w) <= maxW {
				toks = append(toks, pairTok{s: w})
				continue
			}
			if !split {
				return nil, false
			}
			toks = append(toks, m.splitWord(bold, px, w, maxW)...)
		}
	}
	return toks, true
}

// greedy fills lines left to right up to width w.
func (m *pairMeasurer) greedy(bold bool, px int, toks []pairTok, w int) []string {
	var lines []string
	cur, forced := "", false
	for _, t := range toks {
		if cur != "" && !forced {
			if try := cur + " " + t.s; m.width(bold, px, try) <= w {
				cur, forced = try, t.brk
				continue
			}
		}
		if cur != "" {
			lines = append(lines, cur)
		}
		cur, forced = t.s, t.brk
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

// wrap breaks text into balanced lines: the minimal number of lines first,
// then the narrowest width that still gives that many lines (binary search on
// the width). maxLines 0 means unlimited. ok is false when the text needs more
// than maxLines or contains a word wider than maxW (and split is false).
func (m *pairMeasurer) wrap(bold bool, px int, text string, maxW, maxLines int, split bool) (lines []string, ok bool) {
	toks, ok := m.fitTokens(bold, px, text, maxW, split)
	if !ok {
		return nil, false
	}
	if len(toks) == 0 {
		return nil, true
	}
	minW := 0
	for _, t := range toks {
		minW = max(minW, m.width(bold, px, t.s))
	}
	need := len(m.greedy(bold, px, toks, maxW))
	if maxLines > 0 && need > maxLines {
		return nil, false
	}
	lo, hi := minW, maxW
	for lo < hi {
		mid := (lo + hi) / 2
		if len(m.greedy(bold, px, toks, mid)) <= need {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return m.greedy(bold, px, toks, lo), true
}

// ellipsize shortens s (by whole runes) until s + "…" fits maxW; text that
// fits is returned unchanged. It returns "" when not even "…" fits.
func (m *pairMeasurer) ellipsize(bold bool, px int, s string, maxW int) string {
	if m.width(bold, px, s) <= maxW {
		return s
	}
	r := []rune(s)
	for len(r) > 0 {
		r = r[:len(r)-1]
		t := strings.TrimRight(string(r), " ") + pairEllipsis
		if m.width(bold, px, t) <= maxW {
			return t
		}
	}
	return ""
}

// flowPart is a text that flows as unbreakable runs, optionally followed by a
// separator glued to its last word (" ›" in the breadcrumb, " ·" in lists).
type flowPart struct {
	text   string
	bold   bool
	suffix string
}

// flowRuns converts parts to runs no wider than maxW: a part stays one run
// while it fits, otherwise it breaks at words (the suffix stays on the last
// word). ok is false if a word is still too wide and split is false.
func (m *pairMeasurer) flowRuns(px int, parts []flowPart, maxW int, split bool) (runs []pairRun, ok bool) {
	for _, p := range parts {
		whole := p.text + p.suffix
		if m.width(p.bold, px, whole) <= maxW {
			runs = append(runs, pairRun{whole, p.bold})
			continue
		}
		words := strings.Fields(p.text)
		for i, w := range words {
			if i == len(words)-1 {
				w += p.suffix
			}
			if m.width(p.bold, px, w) <= maxW {
				runs = append(runs, pairRun{w, p.bold})
				continue
			}
			if !split {
				return nil, false
			}
			pieces := m.splitWord(p.bold, px, w, maxW)
			for _, pc := range pieces {
				runs = append(runs, pairRun{pc.s, p.bold})
			}
		}
	}
	return runs, true
}

// flow places runs on lines of width maxW, separated by a regular space.
func (m *pairMeasurer) flow(px int, runs []pairRun, maxW int) [][]pairRun {
	sp := m.space(px)
	var lines [][]pairRun
	var cur []pairRun
	x := 0
	for _, r := range runs {
		w := m.width(r.bold, px, r.text)
		if len(cur) > 0 && x+sp+w > maxW {
			lines = append(lines, cur)
			cur, x = nil, 0
		}
		if len(cur) > 0 {
			x += sp
		}
		cur = append(cur, r)
		x += w
	}
	if len(cur) > 0 {
		lines = append(lines, cur)
	}
	return lines
}

// pairPlural picks the Polish plural form for n: one for 1, few for 2-4
// (except 12-14, also after the tens: 22-24, 32-34 ...), many otherwise.
func pairPlural(n int, one, few, many string) string {
	if n == 1 {
		return one
	}
	if n%10 >= 2 && n%10 <= 4 && (n%100 < 12 || n%100 > 14) {
		return few
	}
	return many
}
