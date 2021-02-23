package main

import (
	"sync"
	"time"

	pb "github.com/vyzo/libp2p-flare-test/pb"
	"github.com/vyzo/libp2p-flare-test/proto"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/libp2p/go-msgio/protoio"
	ma "github.com/multiformats/go-multiaddr"
)

const maxMsgSize = 4096

type Daemon struct {
	sync.Mutex
	secret string
	peers  map[peer.ID]struct{}
	info   map[string]map[peer.ID]*ClientInfo
}

type ClientInfo struct {
	nick string
	pi   peer.AddrInfo
}

func NewDaemon(h host.Host, cfg *Config) *Daemon {
	daemon := &Daemon{
		secret: cfg.Secret,
		peers:  make(map[peer.ID]struct{}),
		info:   make(map[string]map[peer.ID]*ClientInfo),
	}
	h.SetStreamHandler(proto.ProtoID, daemon.handleStream)
	return daemon
}

func (d *Daemon) handleStream(s network.Stream) {
	defer s.Close()

	p := s.Conn().RemotePeer()
	log.Debugf("incoming stream from %s at %s", p, s.Conn().RemoteMultiaddr())

	d.Lock()
	_, active := d.peers[p]
	if !active {
		d.peers[p] = struct{}{}
	}
	d.Unlock()
	if active {
		log.Warnf("already have presence from %s; resetting stream", p)
		s.Reset()
		return
	}

	defer func() {
		d.Lock()
		delete(d.peers, p)
		for _, info := range d.info {
			delete(info, p)
		}
		d.Unlock()
	}()

	var msg pb.FlareMessage
	wr := protoio.NewDelimitedWriter(s)
	rd := protoio.NewDelimitedReader(s, maxMsgSize)

	// Authenticate peer
	s.SetDeadline(time.Now().Add(time.Minute))
	if err := rd.ReadMsg(&msg); err != nil {
		log.Warnf("error reading authen message from %s: %s", p, err)
		s.Reset()
		return
	}

	if t := msg.GetType(); t != pb.FlareMessage_AUTHEN {
		log.Warnf("expected authen message from %s, got %d", p, t)
		s.Reset()
		return
	}

	auth := msg.GetAuthen()
	if auth == nil {
		log.Warnf("missing authentication from %s", p)
		s.Reset()
		return
	}

	proofNonce := auth.GetNonce()
	proof := proto.Proof(d.secret, proofNonce)
	challengeNonce, err := proto.Nonce()
	if err != nil {
		log.Warnf("error generating nonce for %s: %s", p, err)
		s.Reset()
		return
	}

	msg.Reset()
	msg.Type = pb.FlareMessage_CHALLENGE.Enum()
	msg.Challenge = &pb.Challenge{
		Proof: proof,
		Nonce: challengeNonce,
	}
	if err := wr.WriteMsg(&msg); err != nil {
		log.Warnf("error writing challenge message to %s: %s", p, err)
		s.Reset()
		return
	}

	msg.Reset()
	if err := rd.ReadMsg(&msg); err != nil {
		log.Warnf("error reading response message from %s: %s", p, err)
		s.Reset()
		return
	}
	s.SetDeadline(time.Time{})

	if t := msg.GetType(); t != pb.FlareMessage_RESPONSE {
		log.Warnf("expected response message from %s, got %d", p, t)
		s.Reset()
		return
	}

	resp := msg.GetResponse()
	if resp == nil {
		log.Warnf("missing response from %s", p)
		s.Reset()
		return
	}

	proof = resp.GetProof()
	if !proto.Verify(d.secret, challengeNonce, proof) {
		log.Errorf("authentication failure from %s", p)
		s.Reset()
		return
	}

	log.Infof("peer %s successfully authenticated", p)

	// client is authenticated, handle announcements and peer requests
	for {
		msg.Reset()
		if err := rd.ReadMsg(&msg); err != nil {
			log.Warnf("error reading message from %s: %s", p, err)
			s.Reset()
			return
		}

		switch t := msg.GetType(); t {
		case pb.FlareMessage_ANNOUNCE:
			ann := msg.GetAnnounce()
			if ann == nil {
				log.Warnf("missing announce from %s", p)
				s.Reset()
				return
			}

			domain := ann.GetDomain()
			cinfo, err := clientInfoFromPeerInfo(ann.GetPeerInfo())
			if err != nil {
				log.Warnf("malformed announce from %s: %s", p, err)
				s.Reset()
				return
			}

			if cinfo.pi.ID != p {
				log.Warnf("annunce for bogus peer ID %s from %s", cinfo.pi.ID, p)
				s.Reset()
				return
			}

			d.Lock()
			tab, ok := d.info[domain]
			if !ok {
				tab = make(map[peer.ID]*ClientInfo)
				d.info[domain] = tab
			}
			tab[p] = cinfo
			d.Unlock()

		case pb.FlareMessage_GETPEERS:
			getPeers := msg.GetGetPeers()
			if getPeers == nil {
				log.Warnf("missing getPeers from %s", p)
				s.Reset()
				return
			}

			domain := getPeers.GetDomain()

			var pis []*pb.PeerInfo
			d.Lock()
			tab, ok := d.info[domain]
			if ok {
				for _, info := range tab {
					if info.pi.ID == p {
						continue
					}
					pi := peerInfoFromClientInfo(info)
					pis = append(pis, pi)
				}
			}
			d.Unlock()

			msg.Reset()
			msg.Type = pb.FlareMessage_PEERLIST.Enum()
			msg.PeerList = &pb.PeerList{Peers: pis}
			if err := wr.WriteMsg(&msg); err != nil {
				log.Warnf("error writing message to %s: %s", p, err)
				s.Reset()
				return
			}

		default:
			log.Warnf("unexpected message from %s: expected ANNOUNCE or GETPEERS, got %d", p, t)
			s.Reset()
			return
		}
	}
}

func clientInfoFromPeerInfo(pi *pb.PeerInfo) (*ClientInfo, error) {
	result := new(ClientInfo)
	result.nick = pi.GetNick()

	pid, err := peer.IDFromBytes(pi.GetPeerID())
	if err != nil {
		return nil, err
	}
	result.pi.ID = pid

	for _, ab := range pi.GetAddrs() {
		a, err := ma.NewMultiaddrBytes(ab)
		if err != nil {
			return nil, err
		}
		result.pi.Addrs = append(result.pi.Addrs, a)
	}

	return result, nil
}

func peerInfoFromClientInfo(info *ClientInfo) *pb.PeerInfo {
	result := &pb.PeerInfo{
		Nick:   &info.nick,
		PeerID: []byte(info.pi.ID),
	}

	for _, a := range info.pi.Addrs {
		result.Addrs = append(result.Addrs, a.Bytes())
	}

	return result
}
