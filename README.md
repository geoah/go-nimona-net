# Nimona Network

Network allows peers to communicate via various user-defined protocols,
it tries to abstracts peer discovery, connection, stream multiplexing, etc.

* Protocol selection is handled by `multiformats/go-multistream`.
* Mutliplexing is handled by `nimona/go-nimona-mux`.
* Peer discovery will be handled by `nimona/go-kad-dht`.
* Peers are managed by an internal peerstore so the dht and other parts can 
  add/remove peers.

Once a TCP connection is established, the `/nimux/v1.0.0` will be selected.
After that, both client and server will be able to open new streams using mux
and negotiate any protocol they support without having to open new TCP or 
other connections.

Only TCP transport is currently supported.