// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package p2p implements the Ethereum p2p network protocols.
package p2p

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common/mclock"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/nat"
	"github.com/ethereum/go-ethereum/p2p/netutil"
)

// Main ideas:
// 1. The way protocols are defined will still be the same, by registering a
// 	  p2p.Protocol object. Thus, the transports have to deal with that.
// 2. We have a Peer object that returns the info about a peer. It DOES NOT
// 	  include any transport info to send messages. That is done with protocols.
//	  It does include functions to close the connection.
//

const (
	defaultDialTimeout = 15 * time.Second

	// This is the fairness knob for the discovery mixer. When looking for peers, we'll
	// wait this long for a single source of candidates before moving on and trying other
	// sources.
	discmixTimeout = 5 * time.Second

	// Connectivity defaults.
	defaultMaxPendingPeers = 50
	defaultDialRatio       = 3

	// This time limits inbound connection attempts per source IP.
	inboundThrottleTime = 30 * time.Second

	// Maximum time allowed for reading a complete message.
	// This is effectively the amount of time a connection can be idle.
	frameReadTimeout = 30 * time.Second

	// Maximum amount of time allowed for writing a complete message.
	frameWriteTimeout = 20 * time.Second
)

var errServerStopped = errors.New("server stopped")

// Config holds Server options.
type Config struct {
	// This field must be set to a valid secp256k1 private key.
	PrivateKey *ecdsa.PrivateKey `toml:"-"`

	// MaxPeers is the maximum number of peers that can be
	// connected. It must be greater than zero.
	MaxPeers int

	// MaxPendingPeers is the maximum number of peers that can be pending in the
	// handshake phase, counted separately for inbound and outbound connections.
	// Zero defaults to preset values.
	MaxPendingPeers int `toml:",omitempty"`

	// DialRatio controls the ratio of inbound to dialed connections.
	// Example: a DialRatio of 2 allows 1/2 of connections to be dialed.
	// Setting DialRatio to zero defaults it to 3.
	DialRatio int `toml:",omitempty"`

	// NoDiscovery can be used to disable the peer discovery mechanism.
	// Disabling is useful for protocol debugging (manual topology).
	NoDiscovery bool

	// DiscoveryV5 specifies whether the new topic-discovery based V5 discovery
	// protocol should be started or not.
	DiscoveryV5 bool `toml:",omitempty"`

	// Name sets the node name of this server.
	// Use common.MakeName to create a name that follows existing conventions.
	Name string `toml:"-"`

	// BootstrapNodes are used to establish connectivity
	// with the rest of the network.
	BootstrapNodes []*enode.Node

	// BootstrapNodesV5 are used to establish connectivity
	// with the rest of the network using the V5 discovery
	// protocol.
	BootstrapNodesV5 []*enode.Node `toml:",omitempty"`

	// Static nodes are used as pre-configured connections which are always
	// maintained and re-connected on disconnects.
	StaticNodes []*enode.Node

	// Trusted nodes are used as pre-configured connections which are always
	// allowed to connect, even above the peer limit.
	TrustedNodes []*enode.Node

	// Connectivity can be restricted to certain IP networks.
	// If this option is set to a non-nil value, only hosts which match one of the
	// IP networks contained in the list are considered.
	NetRestrict *netutil.Netlist `toml:",omitempty"`

	// NodeDatabase is the path to the database containing the previously seen
	// live nodes in the network.
	NodeDatabase string `toml:",omitempty"`

	// Protocols should contain the protocols supported
	// by the server. Matching protocols are launched for
	// each peer.
	Protocols []Protocol `toml:"-"`

	// If ListenAddr is set to a non-nil address, the server
	// will listen for incoming connections.
	//
	// If the port is zero, the operating system will pick a port. The
	// ListenAddr field will be updated with the actual address when
	// the server is started.
	ListenAddr string

	// If set to a non-nil value, the given NAT port mapper
	// is used to make the listening port available to the
	// Internet.
	NAT nat.Interface `toml:",omitempty"`

	// If Dialer is set to a non-nil value, the given Dialer
	// is used to dial outbound peer connections.
	// Dialer NodeDialer `toml:"-"`

	// If NoDial is true, the server will not dial any peers.
	NoDial bool `toml:",omitempty"`

	// If EnableMsgEvents is set then the server will emit PeerEvents
	// whenever a message is sent to or received from a peer
	EnableMsgEvents bool

	// Logger is a custom logger to use with the p2p.Server.
	Logger log.Logger `toml:",omitempty"`

	LibP2PPort int64

	clock mclock.Clock
}

// Server manages all peer connections.
type Server struct {
	// Config fields may not be modified while the server is running.
	Config // TODO: THIS INLINE IS UGLY

	// Hooks for testing. These are useful because we can inhibit
	// the whole protocol stack.
	// newTransport func(net.Conn, *ecdsa.PublicKey) transport
	newPeerHook func(*Peer)
	//listenFunc  func(network, addr string) (net.Listener, error)

	lock    sync.Mutex // protects running
	running bool

	//listener     net.Listener
	ourHandshake *protoHandshake
	loopWG       sync.WaitGroup // loop, listenLoop
	peerFeed     event.Feed
	log          log.Logger

	nodedb    *enode.DB
	localnode *enode.LocalNode
	ntab      *discover.UDPv4
	DiscV5    *discover.UDPv5
	discmix   *enode.FairMix
	dialsched *dialScheduler

	// Channels into the run loop.
	closeCh chan struct{}
	//addtrusted    chan *enode.Node // REMOVE
	//removetrusted chan *enode.Node // REMOVE
	//peerOp                  chan peerOpFunc
	//peerOpDone              chan struct{}
	//delpeer                 chan peerDrop // REMOVE
	//checkpointPostHandshake chan *conn // REMOVE
	//checkpointAddPeer       chan *conn // REMOVE

	// State of run loop and listenLoop.
	inboundHistory expHeap

	// Bor

	// new fields
	trusted peerList

	peers        map[enode.ID]*Peer
	peersLock    sync.Mutex
	inboundCount int64

	// new transports
	libp2pTransport *libp2pTransportV2
	devp2pTransport *devp2pTransportV2
}

//type peerOpFunc func(map[enode.ID]*Peer)

type peerDrop struct {
	peer      *Peer
	err       error
	requested bool // true if signaled by the peer
}

// LocalNode returns the local node record.
func (srv *Server) LocalNode() *enode.LocalNode {
	return srv.localnode
}

// Peers returns all connected peers.
func (srv *Server) Peers() []*Peer {
	fmt.Println("=>2")
	srv.peersLock.Lock()
	defer srv.peersLock.Unlock()

	var ps []*Peer
	//srv.doPeerOp(func(peers map[enode.ID]*Peer) {
	for _, p := range srv.peers {
		ps = append(ps, p)
	}
	//})
	return ps
}

// PeerCount returns the number of connected peers.
func (srv *Server) PeerCount() int {
	return srv.lenPeers()
	/*
		var count int
		srv.doPeerOp(func(ps map[enode.ID]*Peer) {
			count = len(ps)
		})
		return count
	*/
}

// AddPeer adds the given node to the static node set. When there is room in the peer set,
// the server will connect to the node. If the connection fails for any reason, the server
// will attempt to reconnect the peer.
func (srv *Server) AddPeer(node *enode.Node) {
	srv.dialsched.addStatic(node)
}

// RemovePeer removes a node from the static node set. It also disconnects from the given
// node if it is currently connected as a peer.
//
// This method blocks until all protocols have exited and the peer is removed. Do not use
// RemovePeer in protocol implementations, call Disconnect on the Peer instead.
func (srv *Server) RemovePeer(node *enode.Node) {
	fmt.Println("=>3")
	srv.peersLock.Lock()

	var (
		ch  chan *PeerEvent
		sub event.Subscription
	)
	// Disconnect the peer on the main loop.
	//srv.doPeerOp(func(peers map[enode.ID]*Peer) {
	srv.dialsched.removeStatic(node)
	if peer, ok := srv.peers[node.ID()]; ok {
		ch = make(chan *PeerEvent, 1)
		sub = srv.peerFeed.Subscribe(ch)
		peer.Disconnect(DiscRequested)
	}

	// THIS, BE CAREFUL
	srv.peersLock.Unlock()

	//})
	// Wait for the peer connection to end.
	if ch != nil {
		defer sub.Unsubscribe()
		for ev := range ch {
			if ev.Peer == node.ID() && ev.Type == PeerEventTypeDrop {
				return
			}
		}
	}
}

// AddTrustedPeer adds the given node to a reserved trusted list which allows the
// node to always connect, even if the slot are full.
func (srv *Server) AddTrustedPeer(node *enode.Node) {
	srv.log.Trace("Adding trusted node", "node", node)
	srv.setTrusted(node.ID(), true)

	/*
		select {
		case srv.addtrusted <- node:
		case <-srv.quit:
		}
	*/
}

// RemoveTrustedPeer removes the given node from the trusted peer set.
func (srv *Server) RemoveTrustedPeer(node *enode.Node) {
	/*
		select {
		case srv.removetrusted <- node:
		case <-srv.quit:
		}
	*/
	// This channel is used by RemoveTrustedPeer to remove a node
	// from the trusted node set.
	srv.log.Trace("Removing trusted node", "node", node)
	srv.setTrusted(node.ID(), false)
}

// SubscribeEvents subscribes the given channel to peer events
func (srv *Server) SubscribeEvents(ch chan *PeerEvent) event.Subscription {
	return srv.peerFeed.Subscribe(ch)
}

// Self returns the local node's endpoint information.
func (srv *Server) Self() *enode.Node {
	srv.lock.Lock()
	ln := srv.localnode
	srv.lock.Unlock()

	if ln == nil {
		return enode.NewV4(&srv.PrivateKey.PublicKey, net.ParseIP("0.0.0.0"), 0, 0)
	}
	return ln.Node()
}

// Stop terminates the server and all active peer connections.
// It blocks until all active connections have been closed.
func (srv *Server) Stop() {
	srv.lock.Lock()
	if !srv.running {
		srv.lock.Unlock()
		return
	}
	srv.running = false
	/*
		if srv.listener != nil {
			// this unblocks listener Accept
			srv.listener.Close()
		}
	*/
	close(srv.closeCh)
	srv.lock.Unlock()
	srv.loopWG.Wait()
	// TODO: This wait breaks some stuff because we are actually not waiting or notifying some stuff
	// Thus, the tests do not Stop the server.
}

// sharedUDPConn implements a shared connection. Write sends messages to the underlying connection while read returns
// messages that were found unprocessable and sent to the unhandled channel by the primary listener.
type sharedUDPConn struct {
	*net.UDPConn
	unhandled chan discover.ReadPacket
}

// ReadFromUDP implements discover.UDPConn
func (s *sharedUDPConn) ReadFromUDP(b []byte) (n int, addr *net.UDPAddr, err error) {
	packet, ok := <-s.unhandled
	if !ok {
		return 0, nil, errors.New("connection was closed")
	}
	l := len(packet.Data)
	if l > len(b) {
		l = len(b)
	}
	copy(b[:l], packet.Data[:l])
	return l, packet.Addr, nil
}

// Close implements discover.UDPConn
func (s *sharedUDPConn) Close() error {
	return nil
}

// Start starts running the server.
// Servers can not be re-used after stopping.
func (srv *Server) Start() (err error) {
	srv.lock.Lock()
	defer srv.lock.Unlock()
	if srv.running {
		return errors.New("server already running")
	}
	srv.running = true
	srv.log = srv.Config.Logger
	if srv.log == nil {
		srv.log = log.Root()
	}
	if srv.clock == nil {
		srv.clock = mclock.System{}
	}
	if srv.NoDial && srv.ListenAddr == "" {
		srv.log.Warn("P2P server will be useless, neither dialing nor listening")
	}

	srv.peers = make(map[enode.ID]*Peer)
	srv.trusted = newPeerList()

	// Put trusted nodes into a map to speed up checks.
	// Trusted peers are loaded on startup or added via AddTrustedPeer RPC.
	for _, n := range srv.TrustedNodes {
		srv.trusted.add(n.ID())
	}

	// static fields
	if srv.PrivateKey == nil {
		return errors.New("Server.PrivateKey must be set to a non-nil key")
	}

	srv.closeCh = make(chan struct{})

	if err := srv.setupLocalNode(); err != nil {
		return err
	}
	if srv.ListenAddr != "" {
		if err := srv.setupListening(); err != nil {
			return err
		}
	}

	if err := srv.setupDiscovery(); err != nil {
		return err
	}
	srv.setupDialScheduler()

	srv.loopWG.Add(1)
	go srv.run()
	return nil
}

func (srv *Server) setupLocalNode() error {
	// Create the devp2p handshake.
	pubkey := crypto.FromECDSAPub(&srv.PrivateKey.PublicKey)
	srv.ourHandshake = &protoHandshake{Version: baseProtocolVersion, Name: srv.Name, ID: pubkey[1:]}
	for _, p := range srv.Protocols {
		srv.ourHandshake.Caps = append(srv.ourHandshake.Caps, p.cap())
	}
	sort.Sort(capsByNameAndVersion(srv.ourHandshake.Caps))

	// Create the local node.
	db, err := enode.OpenDB(srv.Config.NodeDatabase)
	if err != nil {
		return err
	}
	srv.nodedb = db
	srv.localnode = enode.NewLocalNode(db, srv.PrivateKey)
	srv.localnode.SetFallbackIP(net.IP{127, 0, 0, 1})
	// TODO: check conflicts
	for _, p := range srv.Protocols {
		for _, e := range p.Attributes {
			srv.localnode.Set(e)
		}
	}
	switch srv.NAT.(type) {
	case nil:
		// No NAT interface, do nothing.
	case nat.ExtIP:
		// ExtIP doesn't block, set the IP right away.
		ip, _ := srv.NAT.ExternalIP()
		srv.localnode.SetStaticIP(ip)
	default:
		// Ask the router about the IP. This takes a while and blocks startup,
		// do it in the background.
		srv.loopWG.Add(1)
		go func() {
			defer srv.loopWG.Done()
			if ip, err := srv.NAT.ExternalIP(); err == nil {
				srv.localnode.SetStaticIP(ip)
			}
		}()
	}
	return nil
}

func (srv *Server) setupDiscovery() error {
	srv.discmix = enode.NewFairMix(discmixTimeout)

	// Add protocol-specific discovery sources.
	added := make(map[string]bool)
	for _, proto := range srv.Protocols {
		if proto.DialCandidates != nil && !added[proto.Name] {
			srv.discmix.AddSource(proto.DialCandidates)
			added[proto.Name] = true
		}
	}

	// Don't listen on UDP endpoint if DHT is disabled.
	if srv.NoDiscovery && !srv.DiscoveryV5 {
		return nil
	}

	addr, err := net.ResolveUDPAddr("udp", srv.ListenAddr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	realaddr := conn.LocalAddr().(*net.UDPAddr)
	srv.log.Debug("UDP listener up", "addr", realaddr)
	if srv.NAT != nil {
		if !realaddr.IP.IsLoopback() {
			srv.loopWG.Add(1)
			go func() {
				nat.Map(srv.NAT, srv.closeCh, "udp", realaddr.Port, realaddr.Port, "ethereum discovery")
				srv.loopWG.Done()
			}()
		}
	}
	srv.localnode.SetFallbackUDP(realaddr.Port)

	// Discovery V4
	var unhandled chan discover.ReadPacket
	var sconn *sharedUDPConn
	if !srv.NoDiscovery {
		if srv.DiscoveryV5 {
			unhandled = make(chan discover.ReadPacket, 100)
			sconn = &sharedUDPConn{conn, unhandled}
		}
		cfg := discover.Config{
			PrivateKey:  srv.PrivateKey,
			NetRestrict: srv.NetRestrict,
			Bootnodes:   srv.BootstrapNodes,
			Unhandled:   unhandled,
			Log:         srv.log,
		}
		ntab, err := discover.ListenV4(conn, srv.localnode, cfg)
		if err != nil {
			return err
		}
		srv.ntab = ntab
		srv.discmix.AddSource(ntab.RandomNodes())
	}

	// Discovery V5
	if srv.DiscoveryV5 {
		cfg := discover.Config{
			PrivateKey:  srv.PrivateKey,
			NetRestrict: srv.NetRestrict,
			Bootnodes:   srv.BootstrapNodesV5,
			Log:         srv.log,
		}
		var err error
		if sconn != nil {
			srv.DiscV5, err = discover.ListenV5(sconn, srv.localnode, cfg)
		} else {
			srv.DiscV5, err = discover.ListenV5(conn, srv.localnode, cfg)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (srv *Server) Disconnected(disc peerDisconnected) {
	fmt.Println("=>4")
	srv.peersLock.Lock()
	defer srv.peersLock.Unlock()
	fmt.Println("=>4.1")
	peer, ok := srv.peers[disc.Id]
	if !ok {
		log.Error("disconnected peer not found", "id", disc.Id)
		return
	}
	fmt.Println("=>4.2")
	fmt.Println(disc)
	fmt.Println(srv.peers)

	delete(srv.peers, disc.Id)
	srv.log.Debug("Removing p2p peer", "peercount", srv.lenPeers(), "id", disc.Id, "req", disc.RemoteRequested, "err", disc.Error)
	fmt.Println("=>4.3")
	srv.dialsched.peerRemoved(peer)
	if peer.Inbound() {
		srv.delInbound()
	}
	fmt.Println("=>4.4")
}

func (srv *Server) setupDialScheduler() {
	config := dialConfig{
		self:           srv.localnode.ID(),
		maxDialPeers:   srv.maxDialedConns(),
		maxActiveDials: srv.MaxPendingPeers,
		log:            srv.Logger,
		netRestrict:    srv.NetRestrict,
		clock:          srv.clock,
	}
	if srv.ntab != nil {
		config.resolver = srv.ntab
	}
	srv.dialsched = newDialScheduler(config, srv.discmix, srv.SetupConn)
	for _, n := range srv.StaticNodes {
		srv.dialsched.addStatic(n)
	}
	go srv.dialsched.Run()
}

func (srv *Server) maxInboundConns() int {
	return srv.MaxPeers - srv.maxDialedConns()
}

func (srv *Server) maxDialedConns() (limit int) {
	if srv.NoDial || srv.MaxPeers == 0 {
		return 0
	}
	if srv.DialRatio == 0 {
		limit = srv.MaxPeers / defaultDialRatio
	} else {
		limit = srv.MaxPeers / srv.DialRatio
	}
	if limit == 0 {
		limit = 1
	}
	return limit
}

func (srv *Server) setupListening() error {
	/*
		// Launch the listener.
		listener, err := srv.listenFunc("tcp", srv.ListenAddr)
		if err != nil {
			return err
		}
		srv.listener = listener
		srv.ListenAddr = listener.Addr().String()

		// Update the local node record and map the TCP listening port if NAT is configured.
		if tcp, ok := listener.Addr().(*net.TCPAddr); ok {
			srv.localnode.Set(enr.TCP(tcp.Port))
			if !tcp.IP.IsLoopback() && srv.NAT != nil {
				srv.loopWG.Add(1)
				go func() {
					nat.Map(srv.NAT, srv.quit, "tcp", tcp.Port, tcp.Port, "ethereum p2p")
					srv.loopWG.Done()
				}()
			}
		}
	*/

	// BOR: Set the libp2p port in the localNode
	if srv.Config.LibP2PPort != 0 {
		srv.LocalNode().Set(enode.LibP2PEntry(srv.Config.LibP2PPort))
	}

	srv.loopWG.Add(1)
	go srv.listenLoop()
	return nil
}

func (srv *Server) peerExists(id enode.ID) bool {
	fmt.Println("=>5")
	srv.peersLock.Lock()
	defer srv.peersLock.Unlock()
	fmt.Println("=>5.1")

	_, ok := srv.peers[id]
	return ok
}

func (srv *Server) lenPeers() int {
	fmt.Println("=>6")
	srv.peersLock.Lock()
	defer srv.peersLock.Unlock()
	fmt.Println("=>6.1")

	num := len(srv.peers)
	return num
}

func (srv *Server) setTrusted(id enode.ID, trusted bool) {
	fmt.Println("=>7")
	srv.peersLock.Lock()
	defer srv.peersLock.Unlock()

	if trusted {
		srv.trusted.add(id)
	} else {
		srv.trusted.del(id)
	}
	if p, ok := srv.peers[id]; ok {
		p.set(trustedConn, trusted)
	}
}

// run is the main loop of the server.
func (srv *Server) run() {
	// TODO: This should go in the Close function
	srv.log.Info("Started P2P networking", "self", srv.localnode.Node().URLv4())
	defer srv.loopWG.Done()
	defer srv.nodedb.Close()
	defer srv.discmix.Close()
	defer srv.dialsched.Close()

	/*
		srv.peers = make(map[enode.ID]*Peer)

		var (
		//	peers        = make(map[enode.ID]*Peer)
		// inboundCount = 0
		)

		srv.trusted = &sync.Map{}
	*/

	/*
		// Put trusted nodes into a map to speed up checks.
		// Trusted peers are loaded on startup or added via AddTrustedPeer RPC.
		for _, n := range srv.TrustedNodes {
			srv.trusted.Store(n.ID(), true)
		}
	*/

running:
	for {
		select {
		case <-srv.closeCh:
			// The server was stopped. Run the cleanup logic.
			break running

			/*
				case n := <-srv.addtrusted:
					// This channel is used by AddTrustedPeer to add a node
					// to the trusted node set.
					srv.log.Trace("Adding trusted node", "node", n)
					srv.trusted.Store(n.ID(), true)
					srv.setTrusted(n.ID(), true)

					fmt.Println("- is trusted ?")
					fmt.Println(srv.trusted.Load(n.ID()))

				case n := <-srv.removetrusted:
					// This channel is used by RemoveTrustedPeer to remove a node
					// from the trusted node set.
					srv.log.Trace("Removing trusted node", "node", n)
					srv.trusted.Delete(n.ID())
					srv.setTrusted(n.ID(), false)
			*/

			/*
				case op := <-srv.peerOp:
					// This channel is used by Peers and PeerCount.
					panic("DO NOT DO THIS")
					op(peers)
					srv.peerOpDone <- struct{}{}
			*/

			/*
				case c := <-srv.checkpointPostHandshake:
					// A connection has passed the encryption handshake so
					// the remote identity is known (but hasn't been verified yet).
					if _, ok := srv.trusted.Load(c.node.ID()); ok {
						// Ensure that the trusted flag is set before checking against MaxPeers.
						c.flags |= trustedConn
					}
					// TODO: track in-progress inbound node IDs (pre-Peer) to avoid dialing them.
					c.cont <- srv.postHandshakeChecks(c)

				case c := <-srv.checkpointAddPeer:
					// At this point the connection is past the protocol handshake.
					// Its capabilities are known and the remote identity is verified.
					err := srv.addPeerChecks(c)
					if err == nil {
						// The handshakes are done and it passed all checks.
						p := srv.launchPeer(c)
						srv.peerAdd(c.node.ID(), p)
						srv.log.Debug("Adding p2p peer", "peercount", srv.lenPeers(), "id", p.ID(), "conn", c.flags, "addr", p.RemoteAddr(), "name", p.Name())
						srv.dialsched.peerAdded(c)
						if p.Inbound() {
							srv.addInbound()
						}
					}
					c.cont <- err

			*/

			/*
				case pd := <-srv.delpeer:
					fmt.Println("_ delete peer in channel _", pd.ID())

					// A peer disconnected.
					d := common.PrettyDuration(mclock.Now() - pd.created)
					srv.peerDelete(pd.ID())
					srv.log.Debug("Removing p2p peer", "peercount", srv.lenPeers(), "id", pd.ID(), "duration", d, "req", pd.requested, "err", pd.err)
					srv.dialsched.peerRemoved(pd.rw)
					if pd.Inbound() {
						inboundCount--
					}
			*/

		}
	}

	srv.log.Trace("P2P networking is spinning down")

	// Terminate discovery. If there is a running lookup it will terminate soon.
	if srv.ntab != nil {
		srv.ntab.Close()
	}
	if srv.DiscV5 != nil {
		srv.DiscV5.Close()
	}
	// Disconnect all peers.
	for _, p := range srv.peers {
		p.Disconnect(DiscQuitting)
	}
	// Wait for peers to shut down. Pending connections and tasks are
	// not handled here and will terminate soon-ish because srv.quit
	// is closed.
	for len(srv.peers) > 0 {
		// TODO: IMPORTANT

		//p := <-srv.delpeer
		//p.log.Trace("<-delpeer (spindown)")
		//delete(srv.peers, p.ID())
	}
}

func (srv *Server) addInbound() {
	atomic.AddInt64(&srv.inboundCount, 1)
}

func (srv *Server) delInbound() {
	atomic.AddInt64(&srv.inboundCount, -1)
}

func (srv *Server) getInbound() int64 {
	return atomic.LoadInt64(&srv.inboundCount)
}

func (srv *Server) ValidatePreHandshake(peer *Peer) error {
	// A connection has passed the encryption handshake so
	// the remote identity is known (but hasn't been verified yet).
	fmt.Println("QQ1")
	if srv.trusted.contains(peer.node.ID()) {
		// Ensure that the trusted flag is set before checking against MaxPeers.
		peer.set(trustedConn, true)
	}
	fmt.Println("QQ2")
	if !peer.is(trustedConn) && srv.lenPeers() >= srv.MaxPeers {
		fmt.Println("- eign?")
		return DiscTooManyPeers
	}
	fmt.Println("QQ3")
	if !peer.is(trustedConn) && peer.is(inboundConn) && int(srv.getInbound()) >= srv.maxInboundConns() {
		return DiscTooManyPeers
	}
	fmt.Println("QQ4")
	if srv.peerExists(peer.ID()) {
		return DiscAlreadyConnected
	}
	fmt.Println("QQ5")
	if peer.ID() == srv.localnode.ID() {
		return DiscSelf
	}
	return nil
}

func (srv *Server) ValidatePostHandshake(peer *Peer) error {
	// Drop connections with no matching protocols.
	if len(srv.Protocols) > 0 && countMatchingProtocols(srv.Protocols, peer.Caps()) == 0 {
		fmt.Println("no matching protocols")
		fmt.Println(srv.Protocols, peer.Caps())

		return DiscUselessPeer
	}
	return nil
}

// listenLoop runs in its own goroutine and accepts
// inbound connections.
func (srv *Server) listenLoop() {
	// srv.log.Debug("TCP listener up", "addr", srv.listener.Addr())

	/*
		// The slots channel limits accepts of new connections.
		tokens := defaultMaxPendingPeers
		if srv.MaxPendingPeers > 0 {
			tokens = srv.MaxPendingPeers
		}
		slots := make(chan struct{}, tokens)
		for i := 0; i < tokens; i++ {
			slots <- struct{}{}
		}

		// Wait for slots to be returned on exit. This ensures all connection goroutines
		// are down before listenLoop returns.
		defer srv.loopWG.Done()
		defer func() {
			for i := 0; i < cap(slots); i++ {
				<-slots
			}
		}()

		for {
			// Wait for a free slot before accepting.
			<-slots

			var (
				fd      net.Conn
				err     error
				lastLog time.Time
			)
			for {
				fd, err = srv.listener.Accept()
				if netutil.IsTemporaryError(err) {
					if time.Since(lastLog) > 1*time.Second {
						srv.log.Debug("Temporary read error", "err", err)
						lastLog = time.Now()
					}
					time.Sleep(time.Millisecond * 200)
					continue
				} else if err != nil {
					srv.log.Debug("Read error", "err", err)
					slots <- struct{}{}
					return
				}
				break
			}

			remoteIP := netutil.AddrIP(fd.RemoteAddr())
			if err := srv.checkInboundConn(remoteIP); err != nil {
				srv.log.Debug("Rejected inbound connection", "addr", fd.RemoteAddr(), "err", err)
				fd.Close()
				slots <- struct{}{}
				continue
			}
			if remoteIP != nil {
				var addr *net.TCPAddr
				if tcp, ok := fd.RemoteAddr().(*net.TCPAddr); ok {
					addr = tcp
				}
				fd = newMeteredConn(fd, true, addr)
				srv.log.Trace("Accepted connection", "addr", fd.RemoteAddr())
			}
			go func() {
				srv.SetupConn(fd, inboundConn, nil)
				slots <- struct{}{}
			}()
		}
	*/

	// TODO
	// Slot management for incomming connections has been disabled
	// because it is a bit harder to do with two transports at the same time.

	// The transport interface in this case is quite simple, we have an Accept method
	// that will return a Peer object. As with the Dial method, after the Peer is returned, we assume we only have
	// to do some final end checks but the peer is already valid and Protocols are running.
	// This was done as a middleground between devp2p and libp2p implementations, another option would
	// be to have some interface implemetned by the server called AddPeer.

	srv.devp2pTransport = &devp2pTransportV2{
		b: srv,
	}

	// TODO: Nat is disabled
	srv.devp2pTransport.Listen(srv.ListenAddr)

	go func() {
		// devp2p transport
		for {
			peer, err := srv.devp2pTransport.Accept()
			if err != nil {
				panic(err)
			}
			srv.addPeer(peer)
		}
	}()

	if srv.LibP2PPort != 0 {
		// only enabled if libp2pPort flag is set
		srv.libp2pTransport = newLibp2pTransportV2(srv)
		srv.libp2pTransport.init(int(srv.LibP2PPort))

		go func() {
			// libp2p transport
			for {
				peer, err := srv.libp2pTransport.Accept()
				if err != nil {
					panic(err)
				}
				srv.addPeer(peer)
			}
		}()
	}
}

func (srv *Server) checkInboundConn(remoteIP net.IP) error {
	// TODO:

	if remoteIP == nil {
		return nil
	}
	// Reject connections that do not match NetRestrict.
	if srv.NetRestrict != nil && !srv.NetRestrict.Contains(remoteIP) {
		return fmt.Errorf("not in netrestrict list")
	}
	// Reject Internet peers that try too often.
	now := srv.clock.Now()
	srv.inboundHistory.expire(now, nil)
	if !netutil.IsLAN(remoteIP) && srv.inboundHistory.contains(remoteIP.String()) {
		return fmt.Errorf("too many attempts")
	}
	srv.inboundHistory.add(remoteIP.String(), now.Add(inboundThrottleTime))
	return nil
}

// SetupConn runs the handshakes and attempts to add the connection
// as a peer. It returns when the connection has been added as a peer
// or the handshakes have failed.
func (srv *Server) SetupConn(flags connFlag, dialDest *enode.Node) error {
	var peer *Peer
	var err error

	if dialDest.IsMultiAddr() {
		// libp2p transport
		peer, err = srv.libp2pTransport.Dial(dialDest)
	} else {
		// devp2p transport
		peer, err = srv.devp2pTransport.Dial(dialDest)
	}

	if err != nil {
		return err
	}

	srv.addPeer(peer)
	return nil
}

func (srv *Server) addPeer(p *Peer) {
	srv.log.Debug("Adding p2p peer", "peercount", srv.lenPeers(), "id", p.ID(), "conn", p.flags, "addr", p.RemoteAddr(), "name", p.Name())

	fmt.Println("=>1")
	srv.peersLock.Lock()
	defer srv.peersLock.Unlock()

	// add the peer to the set
	srv.peers[p.ID()] = p

	srv.dialsched.peerAdded(p, true)
	if p.Inbound() {
		srv.addInbound()
	}
	fmt.Println("=>1 done")
}

func (srv *Server) LocalPrivateKey() *ecdsa.PrivateKey {
	return srv.Config.PrivateKey
}

func (srv *Server) LocalHandshake() *protoHandshake {
	return srv.ourHandshake
}

func (srv *Server) GetProtocols() []Protocol {
	return srv.Protocols
}

/*
func (srv *Server) setupConn(rawConn net.Conn, flags connFlag, dialDest *enode.Node) error {
	// Prevent leftover pending conns from entering the handshake.
	srv.lock.Lock()
	running := srv.running
	srv.lock.Unlock()
	if !running {
		return errServerStopped
	}

	// clog := srv.log.New("id", c.node.ID(), "addr", c.fd.RemoteAddr(), "conn", c.flags)

	// THIS CONN IS FOR RLPX, HERE JUST CALL RLPX_TRANSPORT_V2
	rrr := &devp2pTransportV2{
		b: srv,
	}
	// override the main conn
	peer, err := rrr.connect(rawConn, flags, dialDest)
	if err != nil {
		return err
	}

	err = srv.checkpointAddPeer(peer)
	if err != nil {
		// clog.Trace("Rejected peer", "err", err)
		return err
	}

	return nil
}
*/

/*
// checkpoint sends the conn to run, which performs the
// post-handshake checks for the stage (posthandshake, addpeer).
func (srv *Server) checkpoint(c *conn, stage chan<- *conn) error {
	select {
	case stage <- c:
	case <-srv.quit:
		return errServerStopped
	}
	return <-c.cont
}
*/

/*
// THIS IS THE PART THAT GOES INTO RLPX
func (srv *Server) launchPeer(c *conn) *Peer {
	p := newPeer(srv.log, c, srv.Protocols)
	if srv.EnableMsgEvents {
		// If message events are enabled, pass the peerFeed
		// to the peer.
		p.events = &srv.peerFeed
	}
	go srv.runPeer(p)
	return p
}
*/

/*
// runPeer runs in its own goroutine for each peer.
func (srv *Server) runPeer(p *Peer) {
	if srv.newPeerHook != nil {
		srv.newPeerHook(p)
	}
	srv.peerFeed.Send(&PeerEvent{
		Type:          PeerEventTypeAdd,
		Peer:          p.ID(),
		RemoteAddress: p.RemoteAddr().String(),
		LocalAddress:  p.LocalAddr().String(),
	})

	// Run the per-peer main loop.
	remoteRequested, err := p.run()

	// Announce disconnect on the main loop to update the peer set.
	// The main loop waits for existing peers to be sent on srv.delpeer
	// before returning, so this send should not select on srv.quit.
	// srv.delpeer <- peerDrop{p, err, remoteRequested}
	srv.oldDelPeer(peerDrop{p, err, remoteRequested})

	// Broadcast peer drop to external subscribers. This needs to be
	// after the send to delpeer so subscribers have a consistent view of
	// the peer set (i.e. Server.Peers() doesn't include the peer when the
	// event is received.
	srv.peerFeed.Send(&PeerEvent{
		Type:          PeerEventTypeDrop,
		Peer:          p.ID(),
		Error:         err.Error(),
		RemoteAddress: p.RemoteAddr().String(),
		LocalAddress:  p.LocalAddr().String(),
	})
}
*/

// NodeInfo represents a short summary of the information known about the host.
type NodeInfo struct {
	ID    string `json:"id"`    // Unique node identifier (also the encryption key)
	Name  string `json:"name"`  // Name of the node, including client type, version, OS, custom data
	Enode string `json:"enode"` // Enode URL for adding this peer from remote peers
	ENR   string `json:"enr"`   // Ethereum Node Record
	IP    string `json:"ip"`    // IP address of the node
	Ports struct {
		Discovery int `json:"discovery"` // UDP listening port for discovery protocol
		Listener  int `json:"listener"`  // TCP listening port for RLPx
	} `json:"ports"`
	ListenAddr string                 `json:"listenAddr"`
	Protocols  map[string]interface{} `json:"protocols"`
}

// NodeInfo gathers and returns a collection of metadata known about the host.
func (srv *Server) NodeInfo() *NodeInfo {
	// Gather and assemble the generic node infos
	node := srv.Self()
	info := &NodeInfo{
		Name:       srv.Name,
		Enode:      node.URLv4(),
		ID:         node.ID().String(),
		IP:         node.IP().String(),
		ListenAddr: srv.ListenAddr,
		Protocols:  make(map[string]interface{}),
	}
	info.Ports.Discovery = node.UDP()
	info.Ports.Listener = node.TCP()
	info.ENR = node.String()

	// Gather all the running protocol infos (only once per protocol type)
	for _, proto := range srv.Protocols {
		if _, ok := info.Protocols[proto.Name]; !ok {
			nodeInfo := interface{}("unknown")
			if query := proto.NodeInfo; query != nil {
				nodeInfo = proto.NodeInfo()
			}
			info.Protocols[proto.Name] = nodeInfo
		}
	}
	return info
}

// PeersInfo returns an array of metadata objects describing connected peers.
func (srv *Server) PeersInfo() []*PeerInfo {
	// Gather all the generic and sub-protocol specific infos
	infos := make([]*PeerInfo, 0, srv.PeerCount())
	for _, peer := range srv.Peers() {
		if peer != nil {
			infos = append(infos, peer.Info())
		}
	}
	// Sort the result array alphabetically by node identifier
	for i := 0; i < len(infos); i++ {
		for j := i + 1; j < len(infos); j++ {
			if infos[i].ID > infos[j].ID {
				infos[i], infos[j] = infos[j], infos[i]
			}
		}
	}
	return infos
}
