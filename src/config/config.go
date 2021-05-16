/*
The config package contains structures related to the configuration of an
Yggdrasil node.

The configuration contains, amongst other things, encryption keys which are used
to derive a node's identity, information about peerings and node information
that is shared with the network. There are also some module-specific options
related to TUN, multicast and the admin socket.

In order for a node to maintain the same identity across restarts, you should
persist the configuration onto the filesystem or into some configuration storage
so that the encryption keys (and therefore the node ID) do not change.

Note that Yggdrasil will automatically populate sane defaults for any
configuration option that is not provided.
*/
package config

import (
	"encoding/hex"
	"sync"

	"github.com/yggdrasil-network/yggdrasil-go/src/crypto"
	"github.com/yggdrasil-network/yggdrasil-go/src/defaults"
)

// NodeState represents the active and previous configuration of an Yggdrasil
// node. A NodeState object is returned when starting an Yggdrasil node. Note
// that this structure and related functions are likely to disappear soon.
type NodeState struct {
	Current  NodeConfig
	Previous NodeConfig
	Mutex    sync.RWMutex
}

// Current returns the active node configuration.
func (s *NodeState) GetCurrent() NodeConfig {
	s.Mutex.RLock()
	defer s.Mutex.RUnlock()
	return s.Current
}

// Previous returns the previous node configuration.
func (s *NodeState) GetPrevious() NodeConfig {
	s.Mutex.RLock()
	defer s.Mutex.RUnlock()
	return s.Previous
}

// Replace the node configuration with new configuration.
func (s *NodeState) Replace(n NodeConfig) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	s.Previous = s.Current
	s.Current = n
}

// NodeConfig is the main configuration structure, containing configuration
// options that are necessary for an Yggdrasil node to run. You will need to
// supply one of these structs to the Yggdrasil core when starting a node.
type NodeConfig struct {
	Peers               []string               `comment:"List of connection strings for outbound peer connections in URI format,\ne.g. tcp://a.b.c.d:e or socks://a.b.c.d:e/f.g.h.i:j. These connections\nwill obey the operating system routing table, therefore you should\nuse this section when you may connect via different interfaces."`
	InterfacePeers      map[string][]string    `comment:"List of connection strings for outbound peer connections in URI format,\narranged by source interface, e.g. { \"eth0\": [ tcp://a.b.c.d:e ] }.\nNote that SOCKS peerings will NOT be affected by this option and should\ngo in the \"Peers\" section instead."`
	Listen              []string               `comment:"Listen addresses for incoming connections. You will need to add\nlisteners in order to accept incoming peerings from non-local nodes.\nMulticast peer discovery will work regardless of any listeners set\nhere. Each listener should be specified in URI format as above, e.g.\ntcp://0.0.0.0:0 or tcp://[::]:0 to listen on all interfaces."`
	AdminListen         string                 `comment:"Listen address for admin connections. Default is to listen for local\nconnections either on TCP/9001 or a UNIX socket depending on your\nplatform. Use this value for yggdrasilctl -endpoint=X. To disable\nthe admin socket, use the value \"none\" instead."`
	MulticastInterfaces []string               `comment:"Regular expressions for which interfaces multicast peer discovery\nshould be enabled on. If none specified, multicast peer discovery is\ndisabled. The default value is .* which uses all interfaces."`
	AllowedPublicKeys   []string               `comment:"List of peer encryption public keys to allow incoming TCP peering\nconnections from. If left empty/undefined then all connections will\nbe allowed by default. This does not affect outgoing peerings, nor\ndoes it affect link-local peers discovered via multicast."`
	PublicKey           string                 `comment:"Your public signing key. Your peers may ask you for this to put\ninto their AllowedPublicKeys configuration."`
	PrivateKey          string                 `comment:"Your private signing key. DO NOT share this with anyone!"`
	LinkLocalTCPPort    uint16                 `comment:"The port number to be used for the link-local TCP listeners for the\nconfigured MulticastInterfaces. This option does not affect listeners\nspecified in the Listen option. Unless you plan to firewall link-local\ntraffic, it is best to leave this as the default value of 0. This\noption cannot currently be changed by reloading config during runtime."`
	IfName              string                 `comment:"Local network interface name for TUN adapter, or \"auto\" to select\nan interface automatically, or \"none\" to run without TUN."`
	IfMTU               uint64                 `comment:"Maximum Transmission Unit (MTU) size for your local TUN interface.\nDefault is the largest supported size for your platform. The lowest\npossible value is 1280."`
	SessionFirewall     SessionFirewall        `comment:"The session firewall controls who can send/receive network traffic\nto/from. This is useful if you want to protect this node without\nresorting to using a real firewall. This does not affect traffic\nbeing routed via this node to somewhere else. Rules are prioritised as\nfollows: blacklist, whitelist, always allow outgoing, direct, remote."`
	NodeInfoPrivacy     bool                   `comment:"By default, nodeinfo contains some defaults including the platform,\narchitecture and Yggdrasil version. These can help when surveying\nthe network and diagnosing network routing problems. Enabling\nnodeinfo privacy prevents this, so that only items specified in\n\"NodeInfo\" are sent back if specified."`
	NodeInfo            map[string]interface{} `comment:"Optional node info. This must be a { \"key\": \"value\", ... } map\nor set as null. This is entirely optional but, if set, is visible\nto the whole network on request."`
}

// SessionFirewall controls the session firewall configuration.
type SessionFirewall struct {
	Enable              bool     `comment:"Enable or disable the session firewall. If disabled, network traffic\nfrom any node will be allowed. If enabled, the below rules apply."`
	AllowFromDirect     bool     `comment:"Allow network traffic from directly connected peers."`
	AllowFromRemote     bool     `comment:"Allow network traffic from remote nodes on the network that you are\nnot directly peered with."`
	AlwaysAllowOutbound bool     `comment:"Allow outbound network traffic regardless of AllowFromDirect or\nAllowFromRemote. This does allow a remote node to send unsolicited\ntraffic back to you for the length of the session."`
	WhitelistPublicKeys []string `comment:"List of public keys from which network traffic is always accepted,\nregardless of AllowFromDirect or AllowFromRemote."`
	BlacklistPublicKeys []string `comment:"List of public keys from which network traffic is always rejected,\nregardless of the whitelist, AllowFromDirect or AllowFromRemote."`
}

// Generates default configuration and returns a pointer to the resulting
// NodeConfig. This is used when outputting the -genconf parameter and also when
// using -autoconf.
func GenerateConfig() *NodeConfig {
	// Generate encryption keys.
	spub, spriv := crypto.NewSigKeys()
	// Create a node configuration and populate it.
	cfg := NodeConfig{}
	cfg.Listen = []string{}
	cfg.AdminListen = defaults.GetDefaults().DefaultAdminListen
	cfg.PublicKey = hex.EncodeToString(spub[:])
	cfg.PrivateKey = hex.EncodeToString(spriv[:])
	cfg.Peers = []string{}
	cfg.InterfacePeers = map[string][]string{}
	cfg.AllowedPublicKeys = []string{}
	cfg.MulticastInterfaces = defaults.GetDefaults().DefaultMulticastInterfaces
	cfg.IfName = defaults.GetDefaults().DefaultIfName
	cfg.IfMTU = defaults.GetDefaults().DefaultIfMTU
	cfg.SessionFirewall.Enable = false
	cfg.SessionFirewall.AllowFromDirect = true
	cfg.SessionFirewall.AllowFromRemote = true
	cfg.SessionFirewall.AlwaysAllowOutbound = true
	cfg.NodeInfoPrivacy = false

	return &cfg
}

// NewSigningKeys replaces the signing keypair in the NodeConfig with a new
// signing keypair. The signing keys are used by the switch to derive the
// structure of the spanning tree.
func (cfg *NodeConfig) NewKeys() {
	spub, spriv := crypto.NewSigKeys()
	cfg.PublicKey = hex.EncodeToString(spub[:])
	cfg.PrivateKey = hex.EncodeToString(spriv[:])
}
