package gdom

import (
	"bytes"
	"container/list"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

// Name include namespace Space and localname Local
type Name xml.Name

func NewName(space, local string) Name {
	n := xml.Name{
		Space: space,
		Local: local,
	}
	return Name(n)
}

// Write Name to w
func (n *Name) Write(w io.Writer) error {
	if n.Space != "" {
		_, err := w.Write([]byte(n.Space))
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(":"))
		if err != nil {
			return err
		}
	}
	_, err := w.Write([]byte(n.Local))
	if err != nil {
		return err
	}
	return nil
}

// Node holding one of the types: *Ele, *Comment, *ProcInst, *CharData, *Directive
// make sure that node.Value == node
type Node interface {
	// make a Copy of the node
	Copy() Node
	// Write the node into w
	Write(w io.Writer) error

	pos() *list.Element

	clearpos()

	syncElement(element *list.Element)

	GetParent() Iparent

	clearParent()
}

func RemoveSelf(n Node) {
	p := n.GetParent()
	if p != nil {
		p.getNodes().Remove(n.pos())
	}
}

// only *Ele, *Doc
type Iparent interface {
	// the struct implement Iparent should return the list holding the nodes
	getNodes() *list.List
	// call list.New()
	renewNodes()
}

// Node Iteration Function def, return false to abort iteration
type IterNodeFunc func(node Node) bool

func iterNode(p Iparent, f IterNodeFunc) {
	rt := true
	for x := p.getNodes().Front(); rt && x != nil; {
		nxt := x.Next()
		rt = f(x.Value.(Node))
		x = nxt
	}
}

// Doc xml document type
type Doc struct {
	nodes *list.List
	root  *Ele
}

func (d *Doc) IterNode(f IterNodeFunc) {
	iterNode(d, f)
}

func NewDoc(rootName Name) *Doc {
	nodes := list.New()
	root := NewEle(rootName, nil)
	nodes.PushBack(root)
	return &Doc{
		nodes: nodes,
		root:  root,
	}
}

func (d *Doc) getNodes() *list.List {
	return d.nodes
}

func (d *Doc) renewNodes() {
	d.nodes = list.New()
}

func (d *Doc) Beautiful() {
	d.Root().Beautiful()
}

// n is the node to insert, pos is the *list.Element holding a node which is in the Doc
// n can't be a *Ele
func (e *Doc) InsertBefore(n Node, pos Node) error {

	_, ok := n.(*Ele)
	if ok {
		return errors.New("can't insert ele")
	}
	return insertBefore(e, n, pos)
}

// n is the node to insert, pos is the *list.Element holding a node which is in the Doc
func (e *Doc) InsertAfter(n Node, pos Node) error {
	_, ok := n.(*Ele)
	if ok {
		return errors.New("can't insert ele")
	}
	return insertAfter(e, n, pos)
}

// return the root *Ele of the xml doc
func (d *Doc) Root() *Ele {
	return d.root
}

func (d *Doc) SetRoot(e *Ele) {
	d.root = e
	for x := d.nodes.Front(); x != nil; {
		nxt := x.Next()
		_, ok := x.Value.(*Ele)
		if ok {
			d.nodes.Remove(x)
		}
		x = nxt
	}
}

// return a slice of *list.Element, each of it is holding a *Comment
func (e *Doc) AllComments() []*Comment {
	return allComments(e)
}

// return a slice of *list.Element, each of it is holding a *Directive
func (e *Doc) AllDirectives() []*Directive {
	return allDirectives(e)
}

// return a slice of *list.Element, each of it is holding a *ProcInst
func (e *Doc) AllProcInsts() []*ProcInst {
	return allProcInsts(e)
}

// return a slice of *list.Element, each of it is holding a *CharData
func (e *Doc) AllCharData() []*CharData {
	return allCharData(e)
}

// Write the xml doc into w
func (d *Doc) Write(w io.Writer) error {
	for x := d.nodes.Front(); x != nil; x = x.Next() {
		v := x.Value.(Node)
		err := v.Write(w)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *Doc) ToString() string {
	buf := bytes.NewBuffer(make([]byte, 0, 256))
	d.Write(buf)
	return buf.String()
}

func (d *Doc) ToBytes() []byte {
	buf := bytes.NewBuffer(make([]byte, 0, 256))
	d.Write(buf)
	return buf.Bytes()
}

func Parse(r io.Reader) (d *Doc, err error) {
	decoder := xml.NewDecoder(r)
	return parse(decoder)
}

func ParseString(s string) (d *Doc, err error) {
	r := strings.NewReader(s)
	return Parse(r)
}

func ParseBytes(bts []byte) (d *Doc, err error) {
	r := bytes.NewReader(bts)
	return Parse(r)
}

func parse(decoder *xml.Decoder) (d *Doc, err error) {
	d = &Doc{
		nodes: list.New(),
		root:  nil,
	}
	var curEle *Ele = nil
	var ok bool = false
	for token, err := decoder.RawToken(); err == nil; token, err = decoder.RawToken() {
		switch t := token.(type) {
		case xml.StartElement:
			ele := NewEle(Name(t.Name), curEle)
			for i := 0; i < len(t.Attr); i++ {
				ele.SetAttr(NewAttr(Name(t.Attr[i].Name), t.Attr[i].Value))
			}
			if curEle != nil {
				addEle(curEle, ele)
			} else {
				tmp := d.nodes.PushBack(ele)
				ele.syncElement(tmp)
				if d.root != nil {
					err = errors.New("wrong format, muilti root element")
				}
				d.root = ele
			}
			curEle = ele
		case xml.EndElement:
			curEle, ok = curEle.parent.(*Ele)
			if !ok {
				curEle = nil
			}
		case xml.CharData:
			cd := NewCharData(string(t))
			if curEle != nil {
				addCharData(curEle, cd)
			} else {
				tmp := d.nodes.PushBack(cd)
				cd.syncElement(tmp)
			}
		case xml.Comment:
			cmt := NewComment(string(t))
			if curEle != nil {
				addComment(curEle, cmt)
			} else {
				tmp := d.nodes.PushBack(cmt)
				cmt.syncElement(tmp)
			}
		case xml.ProcInst:
			pi := NewProcInst(t.Target, string(t.Inst))
			if curEle != nil {
				addProcInst(curEle, pi)
			} else {
				tmp := d.nodes.PushBack(pi)
				pi.syncElement(tmp)
			}
		case xml.Directive:
			di := NewDirective(string(t))
			if curEle != nil {
				addDirective(curEle, di)
			} else {
				tmp := d.nodes.PushBack(di)
				di.syncElement(tmp)
			}
		}
	}
	if err == io.EOF {
		err = nil
	}
	return d, err
}

// ProcInst like : <?...?>, contains Target(string) and Inst([]byte)
type ProcInst struct {
	*list.Element
	Target string
	Inst   string
	parent Iparent
}

func NewProcInst(target string, inst string) *ProcInst {
	pi := ProcInst{
		Target: target,
		Inst:   inst,
	}
	return &pi
}

func (p *ProcInst) Copy() Node {
	cp := NewProcInst(p.Target, p.Inst)
	return cp
}

func (p *ProcInst) Write(w io.Writer) error {
	buf := bytes.NewBuffer(make([]byte, 0, 64))
	buf.Write([]byte("<?"))
	buf.Write([]byte(p.Target))
	buf.Write([]byte(" "))
	buf.Write([]byte(p.Inst))
	buf.Write([]byte("?>"))
	_, err := w.Write(buf.Bytes())
	return err

}

func (p *ProcInst) pos() *list.Element {
	return p.Element
}

func (p *ProcInst) clearpos() {
	p.Element = nil
}

func (p *ProcInst) syncElement(e *list.Element) {
	p.Element = e
}

func (p *ProcInst) GetParent() Iparent {
	return p.parent
}

func (p *ProcInst) clearParent() {
	p.parent = nil
}

// Directive like <!...>
type Directive struct {
	*list.Element
	V      string
	parent Iparent
}

func NewDirective(vl string) *Directive {
	d := Directive{
		V: vl,
	}
	return &d
}

func (d *Directive) Copy() Node {
	cp := NewDirective(d.V)
	return cp
}

func (d *Directive) Write(w io.Writer) error {
	buf := bytes.NewBuffer(make([]byte, 0, 64))
	buf.Write([]byte("<!"))
	buf.Write([]byte(d.V))
	buf.Write([]byte(">"))
	_, err := w.Write(buf.Bytes())
	return err
}

func (d *Directive) pos() *list.Element {
	return d.Element
}

func (p *Directive) clearpos() {
	p.Element = nil
}

func (p *Directive) syncElement(e *list.Element) {
	p.Element = e
}

func (p *Directive) GetParent() Iparent {
	return p.parent
}

func (p *Directive) clearParent() {
	p.parent = nil
}

// Comment like <!--...-->
type Comment struct {
	*list.Element
	V      string
	parent Iparent
}

func NewComment(v string) *Comment {
	c := Comment{
		V: v,
	}
	return &c
}

func (c *Comment) Copy() Node {
	cp := NewComment(c.V)
	return cp
}

func (c *Comment) Write(w io.Writer) error {
	buf := bytes.NewBuffer(make([]byte, 0, 64))
	buf.Write([]byte("<!--"))
	buf.Write([]byte(c.V))
	buf.Write([]byte("-->"))
	_, err := w.Write(buf.Bytes())
	return err
}

func (c *Comment) pos() *list.Element {
	return c.Element
}

func (p *Comment) clearpos() {
	p.Element = nil
}

func (p *Comment) syncElement(e *list.Element) {
	p.Element = e
}

func (p *Comment) GetParent() Iparent {
	return p.parent
}

func (p *Comment) clearParent() {
	p.parent = nil
}

// the text in the Element
type CharData struct {
	*list.Element
	V      string
	parent Iparent
}

func NewCharData(ctt string) *CharData {
	c := CharData{
		V: ctt,
	}
	return &c
}

func (c *CharData) Copy() Node {
	cp := NewCharData(c.V)
	return cp
}

func (c *CharData) Write(w io.Writer) error {
	return EscapeWithoutSpace(w, []byte(c.V))
}

func (c *CharData) pos() *list.Element {
	return c.Element
}

func (p *CharData) clearpos() {
	p.Element = nil
}

func (p *CharData) syncElement(e *list.Element) {
	p.Element = e
}

func (p *CharData) GetParent() Iparent {
	return p.parent
}

func (p *CharData) clearParent() {
	p.parent = nil
}

// the xml element type
type Ele struct {
	*list.Element
	Name    Name
	attrs   *list.List
	attrMap map[Name]string
	nodes   *list.List
	parent  Iparent
}

func (e *Ele) IterNode(f IterNodeFunc) {
	iterNode(e, f)
}

// return false to abort iteration
type IterAttrFunc func(attr *Attr) bool

//in f, can't call e.RemoveAttrByName or e.RemoveAttrByStrName. if you need, call e.RemoveAttr
func (e *Ele) IterAttr(f IterAttrFunc) {
	rt := true
	for x := e.attrs.Front(); rt && x != nil; {
		nxt := x.Next()
		rt = f(x.Value.(*Attr))
		x = nxt
	}
}

func NewEle(name Name, parent *Ele) *Ele {
	return &Ele{
		Name:    name,
		nodes:   list.New(),
		attrMap: make(map[Name]string),
		attrs:   list.New(),
		parent:  parent,
	}
}

func (d *Ele) getNodes() *list.List {
	return d.nodes
}

func (d *Ele) renewNodes() {
	d.nodes = list.New()
}

func (d *Ele) ToString() string {
	buf := bytes.NewBuffer(make([]byte, 0, 256))
	d.Write(buf)
	return buf.String()
}

func (d *Ele) ToBytes() []byte {
	buf := bytes.NewBuffer(make([]byte, 0, 256))
	d.Write(buf)
	return buf.Bytes()
}

// Parse  s  and add the result into the e
func (e *Ele) AddNotParsedString(s string) error {
	return e.AddNotParsedBytes([]byte(s))
}

func (e *Ele) AddNotParsedBytes(bs []byte) error {
	buf := bytes.NewBuffer(make([]byte, 0, len(bs)+13))
	buf.Write([]byte("<fake>"))
	buf.Write(bs)
	buf.Write([]byte("</fake>"))
	d, err := ParseBytes(buf.Bytes())
	if err != nil {
		return err
	}
	for x := d.Root().nodes.Front(); x != nil; {
		next := x.Next()
		e.AddNode(x.Value.(Node))
		x = next
	}
	return nil
}

func (e *Ele) Beautiful() {
	e.beautiful(0, 4)
}

func (e *Ele) beautiful(prefix, indent int) {
	if e.nodes.Len() == 1 {
		cd, ok := e.nodes.Front().Value.(*CharData)
		if ok {
			cd.V = strings.TrimSpace(cd.V)
			return
		}
	}
	for x := e.nodes.Front(); x != nil; {
		nxt := x.Next()
		switch nd := x.Value.(type) {
		case *Ele:
			nd.beautiful(prefix+indent, indent)
			v := make([]byte, 1+prefix+indent)
			v[0] = '\n'
			for i := 1; i < len(v); i++ {
				v[i] = ([]byte(" "))[0]
			}
			cd := NewCharData(string(v))
			e.nodes.InsertBefore(cd, x)
		case *CharData:
			v := []byte(nd.V)
			v = bytes.TrimSpace(v)
			ll := len(v) + 1 + prefix + indent
			if nxt == nil {
				ll = len(v) + 1 + prefix
			}
			nv := make([]byte, ll)
			copy(nv, v)
			nv[len(v)] = '\n'
			for i := len(v) + 1; i < len(nv); i++ {
				nv[i] = ([]byte(" "))[0]
			}
			nd.V = string(nv)
			if nxt != nil {
				ee, ok := nxt.Value.(*Ele)
				if ok {
					ee.beautiful(prefix+indent, indent)
				}
				nxt = nxt.Next()
			}
		default:
			v := make([]byte, 1+prefix+indent)
			v[0] = '\n'
			for i := 1; i < len(v); i++ {
				v[i] = ([]byte(" "))[0]
			}
			cd := NewCharData(string(v))
			e.nodes.InsertBefore(cd, x)
		}
		x = nxt
	}
	if e.nodes.Len() > 0 {
		_, ok := e.nodes.Back().Value.(*CharData)
		if ok {
			return
		}
		v := make([]byte, 1+prefix)
		v[0] = '\n'
		for i := 1; i < len(v); i++ {
			v[i] = ([]byte(" "))[0]
		}
		cd := NewCharData(string(v))
		e.nodes.PushBack(cd)
	}
}

func (e *Ele) Write(w io.Writer) error {
	if e.nodes.Len() > 0 {
		_, err := w.Write([]byte("<"))
		if err != nil {
			return err
		}
		err = e.Name.Write(w)
		if err != nil {
			return err
		}

		for x := e.attrs.Front(); x != nil; x = x.Next() {
			_, err = w.Write([]byte(" "))
			if err != nil {
				return err
			}
			attr := x.Value.(*Attr)
			nm := Name(attr.Name)
			err = nm.Write(w)
			if err != nil {
				return err
			}
			_, err = w.Write([]byte("=\""))
			if err != nil {
				return err
			}
			_, err = w.Write([]byte(attr.Value))
			if err != nil {
				return err
			}
			_, err = w.Write([]byte("\""))
			if err != nil {
				return err
			}
		}
		_, err = w.Write([]byte(">"))
		if err != nil {
			return err
		}

		for x := e.nodes.Front(); x != nil; x = x.Next() {
			n := x.Value.(Node)
			err = n.Write(w)
			if err != nil {
				return err
			}
		}

		_, err = w.Write([]byte("</"))
		if err != nil {
			return err
		}
		nm := Name(e.Name)
		err = nm.Write(w)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(">"))
		if err != nil {
			return err
		}
	} else {
		_, err := w.Write([]byte("<"))
		if err != nil {
			return err
		}
		nm := Name(e.Name)
		err = nm.Write(w)
		if err != nil {
			return err
		}

		for x := e.attrs.Front(); x != nil; x = x.Next() {
			_, err = w.Write([]byte(" "))
			if err != nil {
				return err
			}
			attr := x.Value.(*Attr)
			nm := Name(attr.Name)
			err = nm.Write(w)
			if err != nil {
				return err
			}
			_, err = w.Write([]byte("=\""))
			if err != nil {
				return err
			}
			_, err = w.Write([]byte(attr.Value))
			if err != nil {
				return err
			}
			_, err = w.Write([]byte("\""))
			if err != nil {
				return err
			}
		}
		_, err = w.Write([]byte("/>"))
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *Ele) SetAttr(attr *Attr) {
	_, ok := e.attrMap[Name(attr.Name)]
	if ok {
		e.attrMap[Name(attr.Name)] = attr.Value
		for x := e.attrs.Front(); x != nil; x = x.Next() {
			a := x.Value.(Attr)
			if a.Name == attr.Name {
				x.Value = attr
				break
			}
		}
	} else {
		e.attrMap[attr.Name] = attr.Value
		tmp := e.attrs.PushBack(attr)
		attr.Element = tmp
	}
}

func (e *Ele) RemoveAttrByStrName(space, local string) (string, bool) {
	return e.RemoveAttrByName(NewName(space, local))
}

func (e *Ele) RemoveAttrByName(name Name) (string, bool) {
	rt, ok := e.attrMap[name]
	delete(e.attrMap, name)
	for x := e.attrs.Front(); ok && x != nil; x = x.Next() {
		attr := x.Value.(*Attr)
		if attr.Name == name {
			e.attrs.Remove(x)
			attr.Element = nil
			break
		}
	}
	return rt, ok
}

func (e *Ele) RemoveAttr(attr *Attr) {
	_, ok := e.attrMap[attr.Name]
	delete(e.attrMap, attr.Name)
	if ok {
		e.attrs.Remove(attr.Element)
	}
}

func (e *Ele) GetAttrByStrName(space, local string) (string, bool) {
	return e.GetAttr(NewName(space, local))
}

func (e *Ele) GetAttr(name Name) (string, bool) {
	rt, ok := e.attrMap[name]
	return rt, ok
}

func (e *Ele) Copy() Node {
	cp := NewEle(e.Name, nil)
	for x := e.attrs.Front(); x != nil; x = x.Next() {
		a := x.Value.(*Attr)
		na := NewAttr(a.Name, a.Value)
		cp.SetAttr(na)
	}
	for x := e.nodes.Front(); x != nil; x = x.Next() {
		switch n := x.Value.(type) {
		case *Ele:
			cpn := n.Copy().(*Ele)
			cpn.parent = cp
			tmp := cp.nodes.PushBack(cpn)
			cpn.syncElement(tmp)
		case Node:
			cpn := n.Copy()
			tmp := cp.nodes.PushBack(cpn)
			cpn.syncElement(tmp)
		}
	}
	return cp
}

func (e *Ele) pos() *list.Element {
	return e.Element
}

func (e *Ele) clearpos() {
	e.Element = nil
}

func (p *Ele) syncElement(e *list.Element) {
	p.Element = e
}

func (p *Ele) GetParent() Iparent {
	return p.parent
}

func (p *Ele) clearParent() {
	p.parent = nil
}

func (e *Ele) AddNode(n Node) error {
	switch nd := n.(type) {
	case *Ele:
		e.AddEle(nd)
	case *Comment:
		e.AddComment(nd)
	case *Directive:
		e.AddDirective(nd)
	case *CharData:
		e.AddCharData(nd)
	case *ProcInst:
		e.AddProcInst(nd)
	default:
		return errors.New("not a node")
	}
	return nil
}

func (e *Ele) AddEle(ele *Ele) {
	cpe := ele.Copy().(*Ele)
	addEle(e, cpe)
}

func addEle(e Iparent, ele *Ele) {
	ele.parent = e
	tmp := e.getNodes().PushBack(ele)
	ele.syncElement(tmp)
}

func (e *Ele) AddDirective(d *Directive) {
	addDirective(e, d.Copy().(*Directive))
}

func addDirective(e Iparent, d *Directive) {
	d.parent = e
	tmp := e.getNodes().PushBack(d)
	d.syncElement(tmp)
}

func (e *Ele) AddComment(c *Comment) {
	addComment(e, c.Copy().(*Comment))
}

func addComment(e Iparent, c *Comment) {
	c.parent = e
	tmp := e.getNodes().PushBack(c)
	c.syncElement(tmp)
}

func (e *Ele) AddCharData(c *CharData) {
	addCharData(e, c.Copy().(*CharData))
}

func (e *Ele) AddCharDataStr(s string) {
	c := NewCharData(s)
	addCharData(e, c)
}

func addCharData(e Iparent, c *CharData) {
	c.parent = e
	var last *CharData = nil
	ok := false
	if e.getNodes().Back() != nil {
		ok = true
	}
	if ok {
		last, ok = e.getNodes().Back().Value.(*CharData)
	}
	if ok {
		mergeinto1st(last, c)
	} else {
		tmp := e.getNodes().PushBack(c)
		c.syncElement(tmp)
	}
}

func (e *Ele) AddProcInst(p *ProcInst) {
	addProcInst(e, p.Copy().(*ProcInst))
}

func addProcInst(e Iparent, p *ProcInst) {
	p.parent = e
	tmp := e.getNodes().PushBack(p)
	p.syncElement(tmp)
}

func mergeinto1st(c1, c2 *CharData) {
	s := strings.Join([]string{c1.V, c2.V}, "")
	c1.V = s
	if c2.pos() != nil {
		c2.parent.getNodes().Remove(c2.pos())
	}
}

func insertBefore(e Iparent, n Node, npos Node) error {
	if !checkIsNode(n) {
		return errors.New("not a node")
	}
	pos := npos.pos()
	var bf *CharData = nil
	ok2 := false
	if pos.Prev() != nil {
		ok2 = true
	}

	cd, ok1 := n.(*CharData)
	if ok2 {
		bf, ok2 = pos.Prev().Value.(*CharData)
	}
	af, ok3 := pos.Value.(*CharData)
	if !ok1 || (!ok2 && !ok3) {
		nc := n.Copy()
		tmp := e.getNodes().InsertBefore(nc, pos)
		nc.syncElement(tmp)
	} else if ok1 && ok2 {
		mergeinto1st(bf, cd)
	} else if ok2 && ok3 {
		mergeinto1st(cd, af)
	} else {
		return errors.New("should never got this")
	}
	return nil
}

// n is the node to insert, pos is the *list.Element holding a node which is in e
func (e *Ele) InsertBefore(n Node, pos Node) error {
	return insertBefore(e, n, pos)
}

func insertAfter(e Iparent, n Node, npos Node) error {
	if !checkIsNode(n) {
		return errors.New("not a node")
	}
	pos := npos.pos()
	cd, ok1 := n.(*CharData)
	bf, ok2 := pos.Value.(*CharData)
	var af *CharData = nil
	ok3 := false
	if pos.Next() != nil {
		ok3 = true
	}
	if ok3 {
		af, ok3 = pos.Next().Value.(*CharData)
	}
	if !ok1 || (!ok2 && !ok3) {
		nc := n.Copy()
		tmp := e.getNodes().InsertAfter(nc, pos)
		nc.syncElement(tmp)
	} else if ok1 && ok2 {
		mergeinto1st(bf, cd)
	} else if ok2 && ok3 {
		mergeinto1st(cd, af)
	} else {
		return errors.New("should never got this")
	}

	return nil
}

// n is the node to insert, pos is the *list.Element holding a node which is in e
func (e *Ele) InsertAfter(n Node, pos Node) error {
	return insertAfter(e, n, pos)
}

func (e *Ele) AllEles() []*Ele {
	return allEles(e)
}

func allEles(e Iparent) []*Ele {
	rt := make([]*Ele, 0, e.getNodes().Len())
	for x := e.getNodes().Front(); x != nil; x = x.Next() {
		n, ok := x.Value.(*Ele)
		if ok {
			rt = append(rt, n)
		}
	}
	return rt
}

func myappendEle(sth *[]*Ele, x *Ele) {
	if len(*sth) < cap(*sth) {
		*sth = append(*sth, x)
	} else {
		extendEle(sth)
		*sth = append(*sth, x)
	}
}

func extendEle(sth *[]*Ele) {
	l := cap(*sth)
	nl := l * 2
	if nl > 128 {
		nl = l + 16
	}
	nsth := make([]*Ele, l, nl)
	copy(nsth, *sth)
	*sth = nsth
}

func (e *Ele) ElesByStrName(space, local string) []*Ele {
	return e.Eles(NewName(space, local))
}

func (e *Ele) Eles(name Name) []*Ele {
	return eles(e, name)
}

func eles(e Iparent, name Name) []*Ele {
	rt := make([]*Ele, 0, 4)
	for x := e.getNodes().Front(); x != nil; x = x.Next() {
		se, ok := x.Value.(*Ele)
		if ok && se.Name == name {
			myappendEle(&rt, se)
		}
	}
	return rt
}

func myappendComment(sth *[]*Comment, x *Comment) {
	if len(*sth) < cap(*sth) {
		*sth = append(*sth, x)
	} else {
		extendComment(sth)
		*sth = append(*sth, x)
	}
}

func extendComment(sth *[]*Comment) {
	l := cap(*sth)
	nl := l * 2
	if nl > 128 {
		nl = l + 16
	}
	nsth := make([]*Comment, l, nl)
	copy(nsth, *sth)
	*sth = nsth
}

func (e *Ele) AllComments() []*Comment {
	return allComments(e)
}

func allComments(e Iparent) []*Comment {
	rt := make([]*Comment, 0, 4)
	for x := e.getNodes().Front(); x != nil; x = x.Next() {
		cmt, ok := x.Value.(*Comment)
		if ok {
			myappendComment(&rt, cmt)
		}
	}
	return rt
}

func myappendDirective(sth *[]*Directive, x *Directive) {
	if len(*sth) < cap(*sth) {
		*sth = append(*sth, x)
	} else {
		extendDirective(sth)
		*sth = append(*sth, x)
	}
}

func extendDirective(sth *[]*Directive) {
	l := cap(*sth)
	nl := l * 2
	if nl > 128 {
		nl = l + 16
	}
	nsth := make([]*Directive, l, nl)
	copy(nsth, *sth)
	*sth = nsth
}

func (e *Ele) AllDirectives() []*Directive {
	return allDirectives(e)
}

func allDirectives(e Iparent) []*Directive {
	rt := make([]*Directive, 0, 4)
	for x := e.getNodes().Front(); x != nil; x = x.Next() {
		d, ok := x.Value.(*Directive)
		if ok {
			myappendDirective(&rt, d)
		}
	}
	return rt
}

func myappendProcInst(sth *[]*ProcInst, x *ProcInst) {
	if len(*sth) < cap(*sth) {
		*sth = append(*sth, x)
	} else {
		extendProcInst(sth)
		*sth = append(*sth, x)
	}
}

func extendProcInst(sth *[]*ProcInst) {
	l := cap(*sth)
	nl := l * 2
	if nl > 128 {
		nl = l + 16
	}
	nsth := make([]*ProcInst, l, nl)
	copy(nsth, *sth)
	*sth = nsth
}

func (e *Ele) AllProcInsts() []*ProcInst {
	return allProcInsts(e)
}

func allProcInsts(e Iparent) []*ProcInst {
	rt := make([]*ProcInst, 0, 4)
	for x := e.getNodes().Front(); x != nil; x = x.Next() {
		p, ok := x.Value.(*ProcInst)
		if ok {
			myappendProcInst(&rt, p)
		}
	}
	return rt
}

func myappendCharData(sth *[]*CharData, x *CharData) {
	if len(*sth) < cap(*sth) {
		*sth = append(*sth, x)
	} else {
		extendCharData(sth)
		*sth = append(*sth, x)
	}
}

func extendCharData(sth *[]*CharData) {
	l := cap(*sth)
	nl := l * 2
	if nl > 128 {
		nl = l + 16
	}
	nsth := make([]*CharData, l, nl)
	copy(nsth, *sth)
	*sth = nsth
}

func (e *Ele) AllCharData() []*CharData {
	return allCharData(e)
}

func allCharData(e Iparent) []*CharData {
	rt := make([]*CharData, 0, 4)
	for x := e.getNodes().Front(); x != nil; x = x.Next() {
		c, ok := x.Value.(*CharData)
		if ok {
			myappendCharData(&rt, c)
		}
	}
	return rt
}

func (e *Ele) Text() string {
	return text(e)
}

func text(e Iparent) string {
	buf := make([]string, 0, e.getNodes().Len()/2+1)
	for x := e.getNodes().Front(); x != nil; x = x.Next() {
		cd, ok := x.Value.(*CharData)
		if ok {
			buf = append(buf, cd.V)
		}
	}
	return strings.Join(buf, "")
}

func (e *Ele) TrimedText() string {
	return trimedText(e)
}

func trimedText(e Iparent) string {
	buf := make([]string, 0, e.getNodes().Len()/2+1)
	for x := e.getNodes().Front(); x != nil; x = x.Next() {
		cd, ok := x.Value.(*CharData)
		if ok {
			buf = append(buf, strings.TrimSpace(cd.V))
		}
	}
	return strings.Join(buf, "")
}

func checkIsNode(n interface{}) bool {
	switch n.(type) {
	case *Ele:
		return true
	case *Comment:
		return true
	case *CharData:
		return true
	case *ProcInst:
		return true
	case *Directive:
		return true
	default:
		return false
	}
	return false
}

func (e *Ele) RemoveNode(n Node) error {
	return removeNode(e, n)
}

func removeNode(e Iparent, n Node) error {
	if !checkIsNode(n) {
		return errors.New("not a node")
	}
	nd := n.pos()
	e.getNodes().Remove(nd)
	n.clearpos()
	n.clearParent()
	return nil
}

func (e *Ele) RemoveAllEle() {
	removeAllEle(e)
}

func removeAllEle(e Iparent) {
	for x := e.getNodes().Front(); x != nil; {
		next := x.Next()
		ele, ok := x.Value.(*Ele)
		if ok {
			e.getNodes().Remove(x)
			ele.clearpos()
			ele.clearParent()
		}
		x = next
	}
}

func (e *Ele) RemoveAllDirective() {
	removeAllDirective(e)
}

func removeAllDirective(e Iparent) {
	for x := e.getNodes().Front(); x != nil; {
		next := x.Next()
		d, ok := x.Value.(*Directive)
		if ok {
			e.getNodes().Remove(x)
			d.clearpos()
			d.clearParent()
		}
		x = next
	}
}

func (e *Ele) RemoveAllComment() {
	removeAllComment(e)
}

func removeAllComment(e Iparent) {
	for x := e.getNodes().Front(); x != nil; {
		next := x.Next()
		c, ok := x.Value.(*Comment)
		if ok {
			e.getNodes().Remove(x)
			c.clearpos()
			c.clearParent()
		}
		x = next
	}
}

func (e *Ele) RemoveAllCharData() {
	removeAllComment(e)
}

func removeAllCharDate(e Iparent) {
	for x := e.getNodes().Front(); x != nil; {
		next := x.Next()
		c, ok := x.Value.(*CharData)
		if ok {
			e.getNodes().Remove(x)
			c.clearpos()
			c.clearParent()
		}
		x = next
	}
}

func (e *Ele) RemoveAllProcInst() {
	removeAllProcInst(e)
}

func removeAllProcInst(e Iparent) {
	for x := e.getNodes().Front(); x != nil; {
		next := x.Next()
		p, ok := x.Value.(*ProcInst)
		if ok {
			e.getNodes().Remove(x)
			p.clearpos()
			p.clearParent()
		}
		x = next
	}
}

func (e *Ele) RemoveAllNodes() {
	for x := e.nodes.Front(); x != nil; x = x.Next() {
		x.Value.(Node).clearParent()
		x.Value.(Node).clearpos()
	}
	e.nodes = list.New()
}

func removeAllNodes(e Iparent) {
	for x := e.getNodes().Front(); x != nil; x = x.Next() {
		x.Value.(Node).clearParent()
		x.Value.(Node).clearpos()
	}
	e.renewNodes()
}

func (e *Ele) RemoveEleByName(name Name) {
	for x := e.nodes.Front(); x != nil; {
		next := x.Next()
		ele, ok := x.Value.(*Ele)
		if ok && ele.Name == name {
			ele.clearParent()
			e.nodes.Remove(x)
			ele.clearpos()
		}
		x = next
	}
}

func (e *Ele) RemoveEleByStrName(space, local string) {
	name := NewName(space, local)
	e.RemoveEleByName(name)
}

func (e *Ele) AllNodes() []Node {
	rt := make([]Node, 0, e.nodes.Len())
	for x := e.nodes.Front(); x != nil; x = x.Next() {
		rt = append(rt, x.Value.(Node))
	}
	return rt
}

type Attr struct {
	*list.Element
	Name  Name
	Value string
}

func NewAttr(name Name, value string) *Attr {
	return &Attr{
		Name:  name,
		Value: value,
	}
}

var (
	esc_quot = []byte("&#34;") // shorter than "&quot;"
	esc_apos = []byte("&#39;") // shorter than "&apos;"
	esc_amp  = []byte("&amp;")
	esc_lt   = []byte("&lt;")
	esc_gt   = []byte("&gt;")
	esc_fffd = []byte("\uFFFD") // Unicode replacement character
)

func EscapeWithoutSpace(w io.Writer, s []byte) error {
	var esc []byte
	last := 0
	for i := 0; i < len(s); {
		r, width := utf8.DecodeRune(s[i:])
		i += width
		switch r {
		case '"':
			esc = esc_quot
		case '\'':
			esc = esc_apos
		case '&':
			esc = esc_amp
		case '<':
			esc = esc_lt
		case '>':
			esc = esc_gt
		default:
			if !isInCharacterRange(r) || (r == 0xFFFD && width == 1) {
				esc = esc_fffd
				break
			}
			continue
		}
		if _, err := w.Write(s[last : i-width]); err != nil {
			return err
		}
		if _, err := w.Write(esc); err != nil {
			return err
		}
		last = i
	}
	if _, err := w.Write(s[last:]); err != nil {
		return err
	}
	return nil
}

func isInCharacterRange(r rune) (inrange bool) {
	return r == 0x09 ||
		r == 0x0A ||
		r == 0x0D ||
		r >= 0x20 && r <= 0xDF77 ||
		r >= 0xE000 && r <= 0xFFFD ||
		r >= 0x10000 && r <= 0x10FFFF
}
