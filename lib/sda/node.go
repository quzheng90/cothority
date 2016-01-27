package sda

import (
	"errors"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/crypto/abstract"
	"github.com/satori/go.uuid"
	"reflect"
	"sync"
)

/*
Node represents a protocol-instance in a given TreeNode. It is linked to
Overlay where all the tree-structures are stored.
*/

type Node struct {
	overlay *Overlay
	token   *Token
	// cache for the TreeNode this Node is representing
	treeNode *TreeNode
	// channels holds all channels available for the different message-types
	channels map[uuid.UUID]interface{}
	// registered handler-functions for that protocol
	handlers map[uuid.UUID]MsgHandler
	// The protocolInstance belonging to that node
	instance ProtocolInstance
	// aggregate messages in order to dispatch them at once in the protocol
	// instance
	msgQueue map[uuid.UUID][]*SDAData
	// Holds flags that influence the behaviour of the node
	flags uint32
	// done channel
	done chan bool
	// done count: a protocol instance may use another protocol instance inside,
	// that will also call Done(). We want to really erase that node when all
	// the protocols have called Done().
	doneCount int
	// locks associated since we must access it from different go routines
	doneLock *sync.Mutex
}

// Bit-values for different flags
// If BactchMessages is set, every message from the children is sent
// directly through the channel
// https://golang.org/ref/spec#Iota
const (
	BatchMessages = 1 << iota
)

// MsgHandler is called upon reception of a certain message-type
type MsgHandler func([]*interface{})

// NewNode creates a new node
func NewNode(o *Overlay, tok *Token) (*Node, error) {
	n := &Node{overlay: o,
		token:    tok,
		channels: make(map[uuid.UUID]interface{}),
		handlers: make(map[uuid.UUID]MsgHandler),
		msgQueue: make(map[uuid.UUID][]*SDAData),
		treeNode: nil,
		done:     make(chan bool),
		doneLock: &sync.Mutex{},
	}
	go n.waitDone()
	return n, n.protocolInstantiate()
}

// TreeNode gets the treeNode of this node. If there is no TreeNode for the
// Token of this node, the function will return nil
func (n *Node) TreeNode() *TreeNode {
	return n.treeNode
}

// Entity returns our entity
func (n *Node) Entity() *network.Entity {
	return n.treeNode.Entity
}

// Parent returns the parent-TreeNode of ourselves
func (n *Node) Parent() *TreeNode {
	return n.treeNode.Parent
}

// Children returns the children of ourselves
func (n *Node) Children() []*TreeNode {
	return n.treeNode.Children
}

// Root returns the root-node of that tree
func (n *Node) Root() *TreeNode {
	return n.Tree().Root
}

// IsRoot returns whether whether we are at the top of the tree
func (n *Node) IsRoot() bool {
	return n.treeNode.Parent == nil
}

// IsLeaf returns whether whether we are at the bottom of the tree
func (n *Node) IsLeaf() bool {
	return len(n.treeNode.Children) == 0
}

// SendTo sends to a given node
func (n *Node) SendTo(to *TreeNode, msg interface{}) error {
	if to == nil {
		return errors.New("Sent to a nil TreeNode")
	}
	return n.overlay.SendToTreeNode(n.token, to, msg)
}

// Tree returns the tree of that node
func (n *Node) Tree() *Tree {
	return n.overlay.TreeFromToken(n.token)
}

// EntityList returns the entity-list
func (n *Node) EntityList() *EntityList {
	return n.Tree().EntityList
}

// RegisterChannel takes a channel with a struct that contains two
// elements: a TreeNode and a message. It will send every message that are the
// same type to this channel. NOTE: In legacy network library, you had to
// register the message type to the network library. Using this function you
// don't have to anymore, it will automatically do the registration for you.
func (n *Node) RegisterChannel(c interface{}) error {
	cr := reflect.TypeOf(c)
	// Check we have the correct channel-type
	if cr.Kind() != reflect.Chan {
		return errors.New("Input is not channel")
	}
	if cr.Elem().Kind() != reflect.Struct {
		return errors.New("Input is not channel of structure")
	}
	if cr.Elem().NumField() != 2 {
		return errors.New("Input is not channel of structure with 2 elements")
	}
	dbg.Lvl3(cr.Elem().Field(0).Type)
	if cr.Elem().Field(0).Type != reflect.TypeOf(TreeNode{}) {
		return errors.New("Input-channel doesn't have TreeNode as element")
	}
	// Automatic registration of the message to the network library.
	typ := network.RegisterMessageUUID(network.RTypeToUUID(cr.Elem().Field(1).Type),
		cr.Elem().Field(1).Type)
	n.channels[typ] = c
	dbg.Lvl3("Registered channel", typ)
	return nil
}

// ProtocolInstance returns the instance of the running protocol
func (n *Node) ProtocolInstance() ProtocolInstance {
	return n.instance
}

// ProtocolInstantiate creates a new instance of a protocol given by it's name
func (n *Node) protocolInstantiate() error {
	if n.token == nil {
		return errors.New("Hope this is running in test-mode")
	}
	pid := n.token.ProtocolID
	p, ok := protocols[pid]
	if !ok {
		return errors.New("Protocol " + pid.String() + " doesn't exist")
	}
	tree := n.overlay.Tree(n.token.TreeID)
	if tree == nil {
		return errors.New("Tree does not exists")
	}
	if n.overlay.EntityList(n.token.EntityListID) == nil {
		return errors.New("EntityList does not exists")
	}
	var err error
	n.treeNode, err = n.overlay.TreeNodeFromToken(n.token)
	if err != nil {
		return errors.New("We are not represented in the tree")
	}
	n.instance, err = p(n)
	return err
}

func (n *Node) DispatchFunction(msg []*SDAData) error {
	dbg.Fatal("Not implemented for message", msg)
	return nil
}

// DispatchChannel takes a message and sends it to a channel
func (n *Node) DispatchChannel(msgSlice []*SDAData) error {
	for _, msg := range msgSlice {
		dbg.Lvl3("Received message of type:", msg.MsgType)
		out := n.channels[msg.MsgType]

		dbg.Lvl3("Making new", reflect.TypeOf(out))
		m := reflect.Indirect(reflect.New(reflect.TypeOf(out).Elem()))
		tn := n.Tree().GetTreeNode(msg.From.TreeNodeID)
		if tn == nil {
			return errors.New("Didn't find treenode")
		}

		m.Field(0).Set(reflect.ValueOf(*tn))
		m.Field(1).Set(reflect.ValueOf(msg.Msg))
		dbg.Lvl3("Sending", m, "to", n.Entity().Addresses)
		reflect.ValueOf(out).Send(m)
		dbg.Lvl3("Sent")
	}
	return nil
}

// DispatchMsg will dispatch this SDAData to the right instance
func (n *Node) DispatchMsg(sdaMsg *SDAData) error {
	// if message comes from parent, dispatch directly
	// if messages come from children we must aggregate them
	// if we still need to wait for additional messages, we return
	msgType, msgs, done := n.aggregate(sdaMsg)
	if !done {
		return nil
	}

	var err error
	switch {
	case n.channels[msgType] != nil:
		err = n.DispatchChannel(msgs)
	case n.handlers[msgType] != nil:
		err = n.DispatchFunction(msgs)
	default:
		err = n.instance.Dispatch(msgs)
	}
	return err
}

// SetFlag makes sure a given flag is set
func (n *Node) SetFlag(f uint32) {
	n.flags |= f
}

// ClearFlag makes sure a given flag is removed
func (n *Node) ClearFlag(f uint32) {
	n.flags &^= f
}

// HasFlag returns true if the given flag is set
func (n *Node) HasFlag(f uint32) bool {
	return n.flags&f != 0
}

// aggregate store the message for a protocol instance such that a protocol
// instances will get all its children messages at once.
// node is the node the host is representing in this Tree, and sda is the
// message being analyzed.
func (n *Node) aggregate(sdaMsg *SDAData) (uuid.UUID, []*SDAData, bool) {
	mt := sdaMsg.MsgType
	fromParent := !n.IsRoot() && uuid.Equal(sdaMsg.From.TreeNodeID, n.Parent().Id)
	if fromParent || n.HasFlag(BatchMessages) {
		return mt, []*SDAData{sdaMsg}, true
	}
	// store the msg according to its type
	if _, ok := n.msgQueue[mt]; !ok {
		n.msgQueue[mt] = make([]*SDAData, 0)
	}
	msgs := append(n.msgQueue[mt], sdaMsg)
	n.msgQueue[mt] = msgs
	// do we have everything yet or no
	// get the node this host is in this tree
	// OK we have all the children messages
	if len(msgs) == len(n.Children()) {
		// erase
		delete(n.msgQueue, mt)
		return mt, msgs, true
	}
	// no we still have to wait!
	dbg.Lvl3("Number of msgs:", len(msgs), "number of children:", len(n.Children()))
	return mt, nil, false
}

// Start calls the start-method on the protocol which in turn will initiate
// the first message to its children
func (n *Node) Start() error {
	return n.instance.Start()
}

// Done returns a channel that must be given a bool when a protocol instance has
// finished its work.
func (n *Node) Done() chan bool {
	n.doneLock.Lock()
	n.doneCount++
	protoChan := make(chan bool)
	n.doneLock.Unlock()
	go func() {
		// wait until the channel is closed
		for _ = range protoChan {
		}
		n.done <- true

	}()
	return protoChan
}

// waitDone simply waits on the Done channel and erase itself from the overlay
// when it has received n.doneCount values
func (n *Node) waitDone() {
	var count int
	for {
		_ = <-n.done
		n.doneLock.Lock()
		count++
		doneCount := n.doneCount
		n.doneLock.Unlock()
		// everybody called Done() on the node
		if count == doneCount {
			// we erase ourself
			n.overlay.nodeDone(n.token)
			break
		}
	}
	close(n.done)
}

// Private returns the corresponding private key
func (n *Node) Private() abstract.Secret {
	return n.overlay.host.private
}
