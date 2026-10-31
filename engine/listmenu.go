package engine

// Lists (ovr26:0D9A, remade): ListMenu publishes the lines as View.List
// with the options of its question and waits for a decision: ActChoose N
// with the line X (N -1: the line itself), ActClose. Scrolling, paging
// and the highlight are the presenter's.

// ListNode is a line of a list: header lines (Flag non-zero) and Dim ones
// are not picked.
type ListNode struct {
	Text string
	Flag byte
	Dim  bool // (extra) greyed out and not picked, as headers
	Grey bool // (extra) greyed out but picked like any line (a creation drop-down's entry that forces changes)
	// Without (extra): options left out while this line is the one chosen
	Without []string
}

// skipped: the line is not picked.
func (p *ListNode) skipped() bool { return p.Flag != 0 || p.Dim }

// newList = ovr26:1140: n empty nodes (n<2 gives 1 node).
func newList(n int) []*ListNode {
	if n < 1 {
		n = 1
	}
	l := make([]*ListNode, n)
	for i := range l {
		l[i] = &ListNode{}
	}
	return l
}

// ListMenu = ovr26:0D9A: the list and its options (menu, and Exit with
// showExit) until a decision. Returns the option chosen (PickEnter: the
// line itself; NoPick on exit) and sets *index and *selected to the line
// chosen (nil on exit). The colours, box and redraw are the original's
// and unused.
func (e *Engine) ListMenu(title string, menu Bar, _, _, _, _, _, _, _ int,
	list []*ListNode, showExit bool, _ *bool, index *int, selected **ListNode) Pick {
	if len(list) == 0 {
		*index = 0
		*selected = nil
		return Pick{ID: NoPick}
	}
	if showExit {
		menu = append(menu[:len(menu):len(menu)], Entry{"Exit", PickExit})
	}
	outer := e.snap.list
	e.snap.list = list
	defer func() { e.snap.list = outer }()
	for {
		e.ext.chosen = -1
		p := e.DisplayInput(title, menu, colLightMagenta, colLightGreen, colWhite, true, false)
		i := e.ext.chosen
		switch {
		case p.ID == PickExit || p.ID == NoPick:
			*selected = nil
			return Pick{ID: NoPick}
		case p.ID == PickRelist: // (extra) another member (F1-F8): the caller rebuilds the list
			*selected = nil
			return p
		case p.ID == PickMove: // the pickers stepping their value (creation)
			if len(menu) == 0 {
				*selected = list[min(max(*index, 0), len(list)-1)]
				return p
			}
		case p.ID == PickRedraw || p.ID == PickSlots:
		case i >= 0 && i < len(list) && !list[i].skipped():
			*index, *selected = i, list[i]
			return p
		case p.ID != PickEnter: // an option without a line
			*selected = nil
			return p
		}
	}
}
