// Package ui is a library of functions for simple, generic gui development.
package ui

import (
	"errors"
	"log"
	"math/rand"
	"strings"
)

var (
	ErrNoTemplate = errors.New("Element template missing")
)

// NewIDgenerator returns a function used to create new IDs for Elements. It uses
// a Pseudo-Random Number Generator (PRNG) as it is disirable to have as deterministic
// IDs as possible. Notably for the mostly tstaic elements.
// Evidently, as users navigate the app differently and mya create new Elements
// in a different order (hence calling the ID generator is path-dependent), we
// do not expect to have the same id structure for different runs of a same program.
func NewIDgenerator(seed int64) func() string {
	return func() string {
		bstr := make([]byte, 32)
		rand.Seed(seed)
		_, _ = rand.Read(bstr)
		return string(bstr)
	}
}

type ElementStore struct {
	DocType   string
	Templates map[string]Element
	ByID      map[string]*Element
}

func (e ElementStore) ElementFromTemplate(name string) *Element {
	t, ok := e.Templates[name]
	if ok {
		return &t
	}
	return nil
}

func (e ElementStore) NewTemplate(t *Element) {
	e.Templates[t.Name] = *t
}

func Constructor(es ElementStore) func(string, string) (*Element, error) {
	New := func(name string, id string) (*Element, error) {
		if e := es.ElementFromTemplate(name); e != nil {
			e.Name = name
			e.ID = id
			e.DocType = es.DocType
			// TODO copy any map field
			return e, nil
		}
		return nil, ErrNoTemplate
	}
	return New
}

// Element is the building block of the User Interface. Everything is described
// as an Element having some mutable properties (graphic properties or data properties)
// From the window to the buttons on a page.
type Element struct {
	root        *Element
	subtreeRoot *Element // detached if subtree root has no parent unless subtreeroot == root
	path        *Elements

	Parent *Element

	Name    string
	ID      string
	DocType string

	UIProperties PropertyStore
	Data         DataStore

	OnMutation             MutationCallbacks      // list of mutation handlers stored at elementID/propertyName (Elements react to change in other elements they are monitoring)
	OnEvent                EventListeners         // EventHandlers are to be called when the named event has fired.
	NativeEventUnlisteners NativeEventUnlisteners // Allows to remove event listeners on the native element, registered when bridging event listeners from the native UI platform.

	Children   *Elements
	ActiveView string // holds the name of the view currently displayed. If parameterizable, holds the name of the parameter

	ViewAccessPath *viewNodes // List of views that lay on the path to the Element

	AlternateViews map[string]ViewElements // this is a  store for  named views: alternative to the Children field, used for instance to implement routes/ conditional rendering.

	Native NativeElementWrapper
}

type PropertyStore struct {
	GlobalShared map[string]interface{}

	Default map[string]interface{}

	Inherited map[string]interface{} //Inherited property cannot be mutated by the inheritor

	Local map[string]interface{}

	Inheritable map[string]interface{} // the value of a property overrides ithe value stored in any of its predecessor value store
	// map key is the address of the element's  property
	// being watched and elements is the list of elements watching this property
	// Inheritable encompasses overidden values and inherited values that are being passed down.
	Watchers map[string]*Elements
}

func (p PropertyStore) NewWatcher(propName string, watcher *Element) {
	list, ok := p.Watchers[propName]
	if !ok {
		p.Watchers[propName] = NewElements(watcher)
		return
	}
	list.Insert(watcher, len(list.List))
}
func (p PropertyStore) RemoveWatcher(propName string, watcher *Element) {
	list, ok := p.Watchers[propName]
	if !ok {
		return
	}
	list.Remove(watcher)
}

func (p PropertyStore) Get(propName string) (interface{}, bool) {
	v, ok := p.Inheritable[propName]
	if ok {
		return v, ok
	}
	v, ok = p.Local[propName]
	if ok {
		return v, ok
	}
	v, ok = p.Inherited[propName]
	if ok {
		return v, ok
	}
	v, ok = p.Default[propName]
	if ok {
		return v, ok
	}
	v, ok = p.GlobalShared[propName]
	if ok {
		return v, ok
	}
	return nil, false
}
func (p PropertyStore) Set(propName string, value interface{}, inheritable bool) {
	if inheritable {
		p.Inheritable[propName] = value
		return
	}
	p.Local[propName] = value
} // don't forget to propagate mutation event to watchers

func (p PropertyStore) Inherit(source PropertyStore) {
	if source.Inheritable != nil {
		for k, v := range source.Inheritable {
			p.Inherited[k] = v
		}
	}
}

func (p PropertyStore) SetDefault(propName string, value interface{}) {
	p.Default[propName] = value
}

type DataStore struct {
	Store     map[string]interface{}
	Immutable map[string]interface{}

	// map key is the address of the data being watched (e.g. id/dataname)
	// being watched and elements is the list of elements watching this property
	Watchers map[string]*Elements
}

func (d DataStore) NewWatcher(label string, watcher *Element) {
	list, ok := d.Watchers[label]
	if !ok {
		d.Watchers[label] = NewElements(watcher)
		return
	}
	list.Insert(watcher, len(list.List))
}
func (d DataStore) RemoveWatcher(label string, watcher *Element) {
	v, ok := d.Watchers[label]
	if !ok {
		return
	}
	v.Remove(watcher)
}

func (d DataStore) Get(label string) (interface{}, bool) {
	if v, ok := d.Immutable[label]; ok {
		return v, ok
	}
	v, ok := d.Store[label]
	return v, ok
}
func (d DataStore) Set(label string, value interface{}) {
	if _, ok := d.Immutable[label]; ok {
		return
	}
	d.Store[label] = value
}

func NewDataStore() DataStore {
	return DataStore{make(map[string]interface{}), make(map[string]interface{}), make(map[string]*Elements)}
}

type Elements struct {
	List []*Element
}

func NewElements(elements ...*Element) *Elements {
	return &Elements{elements}
}

func (e *Elements) InsertLast(elements ...*Element) *Elements {
	e.List = append(e.List, elements...)
	return e
}

func (e *Elements) InsertFirst(elements ...*Element) *Elements {
	e.List = append(elements, e.List...)
	return e
}

func (e *Elements) Insert(el *Element, index int) *Elements {
	nel := make([]*Element, 0)
	nel = append(nel, e.List[:index]...)
	nel = append(nel, el)
	nel = append(nel, e.List[index:]...)
	e.List = nel
	return e
}

func (e *Elements) Remove(el *Element) *Elements {
	index := -1
	for k, element := range e.List {
		if element == el {
			index = k
			break
		}
	}
	if index >= 0 {
		e.List = append(e.List[:index], e.List[index+1:]...)
	}
	return e
}

func (e *Elements) RemoveAll() *Elements {
	e.List = nil
	return e
}

func (e *Elements) Replace(old *Element, new *Element) *Elements {
	for k, element := range e.List {
		if element == old {
			e.List[k] = new
			return e
		}
	}
	return e
}

// Handle calls up the event handlers in charge of processing the event for which
// the Element is listening.
func (e *Element) Handle(evt Event) bool {
	evt.SetCurrentTarget(e)
	return e.OnEvent.Handle(evt)
}

// DispatchEvent is used typically to propagate UI events throughout the ui tree.
// If a nativebinding (type NativeEventBridge) is provided, the event will be dispatched
// on the native host only using the nativebinding function.
//
// It may require an event object to be created from the native event object implementation.
// Events are propagated following the model set by web browser DOM events:
// 3 phases being the capture phase, at-target and then bubbling up if allowed.
func (e *Element) DispatchEvent(evt Event, nativebinding NativeEventBridge) *Element {
	if nativebinding != nil {
		nativebinding(evt, evt.Target())
		return e
	}

	if e.Detached() {
		log.Print("Error: Element detached. should not happen.")
		// TODO review which type of event could walk up a detached subtree
		// for instance, how to update darkmode on detached elements especially
		// on attachment. (life cycles? +  globally propagated values from root + mutations propagated in spite of detachment status)
		return e // can happen if we are building a document fragment and try to dispatch a custom event
	}
	if e.path == nil {
		log.Print("Error: Element path does not exist (yet).")
		return e
	}

	// First we apply the capturing event handlers PHASE 1
	evt.SetPhase(1)
	var done bool
	for _, ancestor := range e.path.List {
		if evt.Stopped() {
			return e
		}

		done = ancestor.Handle(evt) // Handling deemed finished in user side logic
		if done || evt.Stopped() {
			return e
		}
	}

	// Second phase: we handle the events at target
	evt.SetPhase(2)
	done = e.Handle(evt)
	if done {
		return e
	}

	// Third phase : bubbling
	if !evt.Bubbles() {
		return e
	}
	evt.SetPhase(3)
	for k := len(e.path.List) - 1; k >= 0; k-- {
		ancestor := e.path.List[k]
		if evt.Stopped() {
			return e
		}
		done = ancestor.Handle(evt)
		if done {
			return e
		}
	}
	return e
}

// func (e *Element) Parse(payload string) *Element      { return e }
// func (e *Element) Unparse(outputformat string) string {}

// AddView adds a named list of children Elements to an Element creating as such
// a named version of the internal state of an Element.
// In a sense, it enables conditional rendering for an Element by allowing to
// switch between the different named internal states.
func (e *Element) AddView(v ViewElements) *Element {
	for _, child := range v.Elements().List {
		child.ViewAccessPath = child.ViewAccessPath.Prepend(newViewNode(e, v)).Prepend(e.ViewAccessPath.nodes...)
		attach(e, child, false)
	}

	e.AlternateViews[v.Name()] = v
	return e
}

// DeleteView  deletes any view that exists for the current Element but is not
// displayed.
func (e *Element) DeleteView(name string) *Element {
	v, ok := e.AlternateViews[name]
	if !ok {
		return e
	}
	for _, el := range v.Elements().List {
		detach(el)
	}
	delete(e.AlternateViews, name)
	return e
}

// RetrieveView will return a pointer to a non-displayed view for the current element.
// If the named ViewElements does not exist, nil is returned.
func (e *Element) RetrieveView(name string) *ViewElements {
	v, ok := e.AlternateViews[name]
	if !ok {
		return nil
	}
	return &v
}

// ActivateVIew is used to render the desired named view for a given Element.
func (e *Element) ActivateView(name string) error {
	newview, ok := e.AlternateViews[name]
	if !ok {
		// Support for parameterized views
		if len(e.AlternateViews) != 0 {
			var view ViewElements
			var parameterName string
			for k, v := range e.AlternateViews {
				if strings.HasPrefix(k, ":") {
					parameterName = k
					view = v
					break
				}
			}
			if parameterName != "" {
				if len(parameterName) == 1 {
					return errors.New("Bad view name parameter. Needs to be longer than 0 character.")
				}
				// Now that we have found a matching parameterized view, let's try to retrieve the actual
				// view corresponding to the submitted value "name"
				v, err := view.ApplyParameter(name)
				if err != nil {
					// This parameter does not seem to be accepted.
					return err
				}
				view = *v

				// Let's detach the former view items
				oldview, ok := e.GetUI("activeview")
				oldviewname, ok2 := oldview.(string)
				viewIsParameterized := (oldviewname != e.ActiveView)
				if ok && ok2 && oldviewname != "" && e.Children != nil {
					for _, child := range e.Children.List {
						detach(child)
						if !viewIsParameterized {
							attach(e, child, false)
						}
					}
					if !viewIsParameterized {
						// the view is not parameterized
						e.AlternateViews[oldviewname] = NewViewElements(oldviewname, e.Children.List...)
					}
				}

				// Let's append the new view Elements
				for _, newchild := range view.Elements().List {
					e.AppendChild(newchild)
				}
				e.SetUI("activeview", name, false)
				e.ActiveView = parameterName
				return nil
			}
		}
		return errors.New("View does not exist.")
	}

	// first we detach the current active View and reattach it as an alternative View if non-parameterized
	oldview, ok := e.GetUI("activeview")
	oldviewname, ok2 := oldview.(string)
	viewIsParameterized := (oldviewname != e.ActiveView)
	if ok && ok2 && oldviewname != "" && e.Children != nil {
		for _, child := range e.Children.List {
			detach(child)
			if !viewIsParameterized {
				attach(e, child, false)
			}
		}
		if !viewIsParameterized {
			// the view is not parameterized
			e.AlternateViews[oldviewname] = NewViewElements(oldviewname, e.Children.List...)
		}
	}
	// we attach and activate the desired view
	for _, child := range newview.Elements().List {
		e.AppendChild(child)
	}
	delete(e.AlternateViews, name)
	e.SetUI("activeview", name, false)
	e.ActiveView = name

	return nil
}

// attach will link a child Element to the subtree its target parent belongs to.
// It does not however position it in any view specifically. At this stage,
// the Element can not be rendered as part of the view.
func attach(parent, child *Element, activeview bool) {
	if activeview {
		child.Parent = parent
		child.path.InsertFirst(parent).InsertFirst(parent.path.List...)
	}
	child.root = parent.root // attached once means attached for ever unless attached to a new app *root (imagining several apps can be ran concurrently and can share ui elements)
	child.subtreeRoot = parent.subtreeRoot

	// if the child is not a navigable view(meaning that its alternateViews is nil, then it's viewadress is its parent's)
	// otherwise, it's its own to which is prepended its parent's viewAddress.
	if child.AlternateViews == nil {
		child.ViewAccessPath = parent.ViewAccessPath
	} else {
		child.ViewAccessPath = child.ViewAccessPath.Prepend(parent.ViewAccessPath.nodes...)
	}

	for _, descendant := range child.Children.List {
		attach(child, descendant, true)
	}

	for _, descendants := range child.AlternateViews {
		for _, descendant := range descendants.Elements().List {
			attach(child, descendant, false)
		}
	}
}

// detach will unlink an Element from its parent. If the element was in a view,
// the element is still being rendered until it is removed. However, it should
// not be anle to react to events or mutations. TODO review the latter part.
func detach(e *Element) {
	if e.Parent == nil {
		return
	}
	e.subtreeRoot = e

	// reset e.path to start with the top-most element i.e. "e" in the current case
	index := -1
	for k, ancestor := range e.path.List {
		if ancestor == e.Parent {
			index = k
			break
		}
	}
	if index >= 0 {
		e.path.List = e.path.List[index+1:]
	}

	e.Parent = nil

	// ViewAccessPath handling:
	if e.AlternateViews == nil {
		e.ViewAccessPath = nil
	} else {
		e.ViewAccessPath.nodes = e.ViewAccessPath.nodes[len(e.ViewAccessPath.nodes)-1:]
	}

	// got to update the subtree with the new subtree root and path
	for _, descendant := range e.Children.List {
		attach(e, descendant, true)
	}

	for _, descendants := range e.AlternateViews {
		for _, descendant := range descendants.Elements().List {
			attach(e, descendant, false)
		}
	}
}

// AppendChild appends a new element to the element's children list for the active
// view being rendered.
func (e *Element) AppendChild(child *Element) *Element {
	if e.DocType != child.DocType {
		log.Printf("Doctypes do not macth. Parent has %s while child Element has %s", e.DocType, child.DocType)
		return e
	}
	attach(e, child, true)

	e.Children.InsertLast(child)
	if e.Native != nil {
		e.Native.AppendChild(child)
	}
	return e
}

func (e *Element) Prepend(child *Element) *Element {
	if e.DocType != child.DocType {
		log.Printf("Doctypes do not macth. Parent has %s while child Element has %s", e.DocType, child.DocType)
		return e
	}
	attach(e, child, true)

	e.Children.InsertFirst(child)
	if e.Native != nil {
		e.Native.PrependChild(child)
	}
	return e
}

func (e *Element) InsertChild(child *Element, index int) *Element {
	if e.DocType != child.DocType {
		log.Printf("Doctypes do not macth. Parent has %s while child Element has %s", e.DocType, child.DocType)
		return e
	}
	attach(e, child, true)

	e.Children.Insert(child, index)
	if e.Native != nil {
		e.Native.InsertChild(child, index)
	}
	return e
}

func (e *Element) ReplaceChild(old *Element, new *Element) *Element {
	if e.DocType != new.DocType {
		log.Printf("Doctypes do not macth. Parent has %s while child Element has %s", e.DocType, new.DocType)
		return e
	}
	attach(e, new, true)

	detach(old)

	e.Children.Replace(old, new)
	if e.Native != nil {
		e.Native.ReplaceChild(old, new)
	}
	return e
}

func (e *Element) RemoveChild(child *Element) *Element {
	detach(child)

	if e.Native != nil {
		e.Native.RemoveChild(child)
	}
	return e
}

func (e *Element) RemoveChildren() *Element {
	for _, child := range e.Children.List {
		e.RemoveChild(child)
	}
	return e
}

func (e *Element) Watch(datalabel string, mutationSource *Element, h *MutationHandler) *Element {
	mutationSource.Data.NewWatcher(datalabel, e)
	e.OnMutation.Add(mutationSource.ID+"/"+datalabel, h)
	return e
}
func (e *Element) Unwatch(datalabel string, mutationSource *Element) *Element {
	mutationSource.Data.RemoveWatcher(datalabel, e)
	return e
}

func (e *Element) AddEventListener(event Event, handler *EventHandler, nativebinding NativeEventBridge) *Element {
	e.OnEvent.AddEventHandler(event, handler)
	if nativebinding != nil {
		nativebinding(event, e)
	}
	return e
}
func (e *Element) RemoveEventListener(event Event, handler *EventHandler, native bool) *Element {
	e.OnEvent.RemoveEventHandler(event, handler)
	if native {
		if e.NativeEventUnlisteners.List != nil {
			e.NativeEventUnlisteners.Apply(event)
		}
	}
	return e
}

// Detached returns whether the subtree the current Element belongs to is attached
// to the main tree or not.
func (e *Element) Detached() bool {
	if e.subtreeRoot.Parent == nil && e.subtreeRoot != e.root {
		return true
	}
	return false
}

func (e *Element) Get(label string) (interface{}, bool) {
	return e.Data.Get(label)
}
func (e *Element) Set(label string, value interface{}) {
	e.Data.Set(label, value)
	evt := e.NewMutationEvent(label, value).Data()
	e.OnMutation.DispatchEvent(evt)
}

func (e *Element) GetUI(propName string) (interface{}, bool) {
	return e.UIProperties.Get(propName)
}

func (e *Element) SetUI(propName string, value interface{}, inheritable bool) {
	e.UIProperties.Set(propName, value, inheritable)
	evt := e.NewMutationEvent(propName, value).UI()
	e.OnMutation.DispatchEvent(evt)
}

// Route returns the path to an Element.
// If the path to an Element includes a parameterized view, the returned route is
// parameterized as well.
//
// Important notice: views that are nested within a fixed element use that Element ID for routing.
// In order for links using the routes to these views to not be breaking between refresh/reruns of an app (hard requirement for online link-sharing), the ID of the parent element
// should be generated so as to not change. Using the default PRNG-based ID generator is very likely to not be a good-fit here.
//
// For instance, if we were to create a dynamic view composed of retrieved tweets, we would not use the default ID generator but probably reuse the tweet ID gotten via http call for each Element.
// Building a shareable link toward any of these elements still require that every ID generated in the path is stable across app refresh/re-runs.
func (e *Element) Route() string {
	// TODO if root is window and not app root, might need to implement additional logic to make link creation process stop at app root.
	var Route string
	if e.Detached() {
		return ""
	}
	if e.ViewAccessPath == nil {
		return "/"
	}

	for _, n := range e.path.List {
		rpath := pathSegment(n, e.ViewAccessPath)
		Route = Route + "/" + rpath
	}
	return Route
}

// viewAdjacence determines whether an Element has more than one adjacent
// sibling view.
func (e *Element) viewAdjacence() bool {
	var count int
	if e.AlternateViews != nil {
		count++
	}
	if e.path != nil && len(e.path.List) > 1 {
		firstAncestor := e.path.List[len(e.path.List)-1]
		if firstAncestor.AlternateViews != nil {
			vnode := e.ViewAccessPath.nodes[len(e.ViewAccessPath.nodes)-1]
			for _, c := range vnode.ViewElements.Elements().List {
				if c.AlternateViews != nil {
					count++
				}
			}
			if count > 1 {
				return true
			}
			return false
		}

		for _, c := range firstAncestor.Children.List {
			if c.AlternateViews != nil {
				count++
			}
		}
		if count > 1 {
			return true
		}
		return false
	}
	return false
}

// pathSegment returns true if the path belongs to a View besides returning the
// first degree relative path of an Element.
// If the view holds Elements which are adjecent view objects, t
func pathSegment(p *Element, views *viewNodes) string {
	rp := p.ID
	if views != nil {
		for _, v := range views.nodes {
			if v.Element.ID == rp {
				rp = v.ViewElements.Name()
				if p.viewAdjacence() {
					rp = p.ID + "/" + rp
				}
				return rp
			}
		}
	}
	return rp
}

// MakeToggable is a simple example of conditional rendering.
// It allows an Element to have a single child Element that can be switched with another one.
//
// For example, if we have a toggable button, one for login and one for logout,
// We can implement the switch between login and logout by switching the inner Elements.
//
// This is merely an example as we could implement toggling between more than two
// Elements quite easily.
// Routing will probably be implemented this way, toggling between states
// when a mutationevent such as browser history occurs.
func MakeToggable(conditionName string, e *Element, firstView ViewElements, secondView ViewElements, initialconditionvalue interface{}) *Element {
	e.AddView(firstView).AddView(secondView)

	toggle := NewMutationHandler(func(evt MutationEvent) bool {
		value, ok := evt.NewValue().(bool)
		if !ok {
			value = false
		}
		if value {
			e.ActivateView(firstView.Name())
		}
		e.ActivateView(secondView.Name())
		return true
	})

	e.Watch(conditionName, e, toggle)

	e.Set(conditionName, initialconditionvalue)
	return e
}

// ViewElements defines a type for a named list of children Element that can be appended
// to an Element, constituting as such a "view".
// ViewElements can be parameterized.
type ViewElements struct {
	name         string
	elements     *Elements
	Parameterize func(parameter string, v ViewElements) (*ViewElements, error)
}

func (v ViewElements) Name() string        { return v.name }
func (v ViewElements) Elements() *Elements { return v.elements }
func (v ViewElements) ApplyParameter(paramvalue string) (*ViewElements, error) {
	return v.Parameterize(paramvalue, v)
}

// NewViewElements can be used to create a list of children Elements to append to an element, for display.
// In effect, allowing to create a named view. (note the lower case letter)
// The true definition of a view is: an *Element and a named list of child Elements (ViewElements) constitute a view.
// An example of use would be an empty window that would be filled with different child elements
// upon navigation.
// A parameterized view can be created by using a naming scheme such as ":parameter" (string with a leading colon)
// In the case, the parameter can be retrieve by the router.
func NewViewElements(name string, elements ...*Element) ViewElements {
	for _, el := range elements {
		el.ActiveView = name
	}
	return ViewElements{name, NewElements(elements...), nil}
}

// NewParameterizedView defines a parameterized, named, list of *Element composing a view.
// The Elements can be parameterized by applying a function submitted as argument.
// This function can and probably should implement validation.
// It may for instance be used to verify that the parameter value belongs to a finite
// set of accepted values.
func NewParameterizedView(parametername string, paramFn func(string, ViewElements) (*ViewElements, error), elements ...*Element) ViewElements {
	if !strings.HasPrefix(parametername, ":") {
		parametername = ":" + parametername
	}
	n := NewViewElements(parametername, elements...)
	n.Parameterize = paramFn
	return n
}

type viewNodes struct {
	nodes []viewNode
}

func newViewNodes() *viewNodes {
	return &viewNodes{make([]viewNode, 0)}
}

func (v *viewNodes) Append(nodes ...viewNode) *viewNodes {
	v.nodes = append(v.nodes, nodes...)
	return v
}

func (v *viewNodes) Prepend(nodes ...viewNode) *viewNodes {
	v.nodes = append(nodes, v.nodes...)
	return v
}

type viewNode struct {
	*Element
	ViewElements
}

func newViewNode(e *Element, view ViewElements) viewNode {
	return viewNode{e, view}
}
