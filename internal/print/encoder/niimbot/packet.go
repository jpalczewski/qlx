package niimbot

import (
	"errors"
	"fmt"
)

var (
	packetHead = []byte{0x55, 0x55}
	packetTail = []byte{0xAA, 0xAA}
)

type Packet struct {
	Type byte
	Data []byte
}

func (p Packet) checksum() byte {
	cs := p.Type ^ byte(len(p.Data))
	for _, b := range p.Data {
		cs ^= b
	}
	return cs
}

func (p Packet) ToBytes() []byte {
	buf := make([]byte, 0, len(p.Data)+7)
	buf = append(buf, packetHead...)
	buf = append(buf, p.Type, byte(len(p.Data)))
	buf = append(buf, p.Data...)
	buf = append(buf, p.checksum())
	buf = append(buf, packetTail...)
	return buf
}

func ParsePacket(data []byte) (Packet, error) {
	if len(data) < 7 {
		return Packet{}, errors.New("packet too short")
	}
	if data[0] != 0x55 || data[1] != 0x55 {
		return Packet{}, errors.New("bad header")
	}
	if data[len(data)-2] != 0xAA || data[len(data)-1] != 0xAA {
		return Packet{}, errors.New("bad tail")
	}
	typ := data[2]
	dlen := int(data[3])
	if len(data) < dlen+7 {
		return Packet{}, fmt.Errorf("truncated: need %d, got %d", dlen+7, len(data))
	}
	pktData := make([]byte, dlen)
	copy(pktData, data[4:4+dlen])
	pkt := Packet{Type: typ, Data: pktData}
	if data[4+dlen] != pkt.checksum() {
		return Packet{}, fmt.Errorf("checksum mismatch: got %x, want %x", data[4+dlen], pkt.checksum())
	}
	return pkt, nil
}
