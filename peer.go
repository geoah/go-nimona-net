package net

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"golang.org/x/crypto/openpgp"
)

var (
	ErrorCannotSign = errors.New("Peer cannot sign")
)

type Peer struct {
	ID        string   `json:"id"`
	Addresses []string `json:"addresses"`
	entity    *openpgp.Entity
}

func (p *Peer) Verify(target, signature []byte) (bool, error) {
	keyring := openpgp.EntityList{
		p.entity,
	}
	btarget := bytes.NewBuffer(target)
	bsignature := bytes.NewBuffer(signature)
	entity, err := openpgp.CheckDetachedSignature(keyring, btarget, bsignature)
	if err != nil {
		return false, nil
	}

	if entity == nil {
		return false, nil
	}

	return true, nil
}

func (p *Peer) Sign(data []byte) ([]byte, error) {
	if p.entity.PrivateKey == nil {
		return nil, ErrorCannotSign
	}

	if p.entity.PrivateKey.CanSign() == false {
		return nil, ErrorCannotSign
	}

	out := bytes.NewBuffer(nil)
	message := bytes.NewBuffer(data)
	if err := openpgp.DetachSign(out, p.entity, message, nil); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

func NewPeerFromArmorFile(path string) (*Peer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	els, err := openpgp.ReadArmoredKeyRing(f)
	if err != nil {
		return nil, err
	}

	return NewPeer(els[0])
}

func NewPeerFromArmor(armor []byte) (*Peer, error) {
	buf := bytes.NewBuffer(armor)
	els, err := openpgp.ReadArmoredKeyRing(buf)
	if err != nil {
		return nil, err
	}

	return NewPeer(els[0])

}

func NewPeer(ent *openpgp.Entity) (*Peer, error) {
	return &Peer{
		ID:     fmt.Sprintf("%x", ent.PrimaryKey.Fingerprint),
		entity: ent,
	}, nil
}
