package ws

import (
	"encoding/binary"
	"math/rand"
)

const (
	OpText   = 0x1
	OpBinary = 0x2
	OpClose  = 0x8
)

func Encode(data []byte) []byte {
	length := len(data)
	var header []byte

	if length < 126 {
		header = []byte{0x82, byte(length) | 0x80}
		mask := make([]byte, 4)
		rand.Read(mask)
		header = append(header, mask...)
		masked := make([]byte, length)
		for i := 0; i < length; i++ {
			masked[i] = data[i] ^ mask[i%4]
		}
		return append(header, masked...)
	}
	// simplified for long payloads
	return nil
}

func Decode(buf []byte) (opcode byte, data []byte, n int) {
	if len(buf) < 2 {
		return 0, nil, 0
	}
	opcode = buf[0] & 0x0F
	length := int(buf[1] & 0x7F)
	maskStart := 2

	switch {
	case length == 126:
		if len(buf) < 4 {
			return 0, nil, 0
		}
		length = int(binary.BigEndian.Uint16(buf[2:4]))
		maskStart = 4
	case length == 127:
		// not implemented
		return 0, nil, 0
	}

	if len(buf) < maskStart+4+length {
		return 0, nil, 0
	}
	mask := buf[maskStart : maskStart+4]
	dataStart := maskStart + 4
	payload := make([]byte, length)
	for i := 0; i < length; i++ {
		payload[i] = buf[dataStart+i] ^ mask[i%4]
	}
	return opcode, payload, dataStart + length
}