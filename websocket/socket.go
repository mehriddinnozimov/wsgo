package websocket

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"math/rand"
	"net"
	"time"

	"github.com/mehriddinnozimov/wsgo/eventemitter"
)

// frame types
// 0x0: "Continuation fragment",                      // handled
// 0x1: "Text fragment",                              // handled
// 0x2: "Binary fragment",                            // ignored
// 0x3: "Reserved for further non-control fragments", // ignored
// 0x4: "Reserved for further non-control fragments", // ignored
// 0x5: "Reserved for further non-control fragments", // ignored
// 0x6: "Reserved for further non-control fragments", // ignored
// 0x7: "Reserved for further non-control fragments", // ignored
// 0x8: "Connection close",                           // handled
// 0x9: "Ping",                                       // ignored
// 0xA: "Pong",                                       // ignored
// 0xB: "Reserved for further control fragments",     // ignored
// 0xC: "Reserved for further control fragments",     // ignored
// 0xD: "Reserved for further control fragments",     // ignored
// 0xE: "Reserved for further control fragments",     // ignored
// 0xF: "Reserved for further control fragments",     // ignored

type OnClose func()
type OnError func(err error)
type OnMessage func(message string)

type Socket struct {
	ID               int
	State            string
	conn             net.Conn
	brw              *bufio.ReadWriter
	ee               *eventemitter.EventEmitter
	writePayloadSize int
}

const finalFragmentOfMessage = 128
const isMasked = 128
const payload0Max = 125
const payloadNext2Byte = 126
const payloadNext8Byte = 127
const byte2MaxValue = math.MaxUint16
const continuationFragment = 0
const textFragment = 1
const connectionCloseFragment = 8
const maxWritePayloadSize = 4096

func NewSocket(conn net.Conn, brw *bufio.ReadWriter) *Socket {
	ee := eventemitter.New()
	id := randomID()
	socket := Socket{id, "Open", conn, brw, &ee, maxWritePayloadSize}

	go socket.reader()

	return &socket
}

func (socket *Socket) Close() {
	closeMessage := byte(finalFragmentOfMessage + connectionCloseFragment)
	message := make([]byte, 2)
	message[0] = closeMessage
	message[1] = 0
	_, err := socket.brw.Write(message)
	if err != nil {
		socket.ee.Emit("error", err)
		return
	}
	socket.brw.Flush()
	socket.ee.Emit("close")
}

func (socket *Socket) _close() {
	socket.State = "Closed"
	socket.ee.Clear()
	socket.conn.Close()
}

func (socket *Socket) OnClose(onClose OnClose) {
	socket.ee.On("close", func(_ ...interface{}) {
		socket._close()
		onClose()
	})
}

func (socket *Socket) OnError(onError OnError) {
	socket.ee.On("error", func(errors ...interface{}) {
		if len(errors) == 0 {
			return
		}

		err := errors[0].(error)
		onError(err)
	})
}

func (socket *Socket) OnMessage(onMessage OnMessage) {
	socket.ee.On("message", func(messages ...interface{}) {
		if len(messages) == 0 {
			return
		}

		message := messages[0].(string)
		onMessage(message)
	})
}

func (socket *Socket) reader() {
	message := []byte{}
	for {
		var payloadLen int = 0

		meta, err := readBytes(socket.brw, 2)
		if err != nil {
			socket.ee.Emit("error", err)
			continue
		}

		endOfMessage := valueOfBit(meta[0], 7)

		fT := fragmentType(meta[0])
		if fT == connectionCloseFragment {
			// closing from client
			socket.ee.Emit("close", nil)
			message = []byte{}
			return
		}

		len0 := int(meta[1])

		if len0 < isMasked {
			socket.ee.Emit("error", errors.New("Socket is not masked"))
			continue
		}

		len0 -= isMasked // remove mask bit
		nextReadLen := 0

		if len0 <= payload0Max {
			payloadLen = int(len0)
		} else if len0 == payloadNext2Byte {
			nextReadLen = 2
		} else {
			nextReadLen = 8
		}

		if nextReadLen > 0 {
			payloadLenInByte, err := readBytes(socket.brw, nextReadLen)
			if err != nil {
				socket.ee.Emit("error", err)
				continue
			}

			if nextReadLen == 2 {
				payloadLen = int(binary.BigEndian.Uint16(payloadLenInByte))
			} else {
				payloadLen = int(binary.BigEndian.Uint64(payloadLenInByte))
			}
		}

		mask, err := readBytes(socket.brw, 4)
		if err != nil {
			socket.ee.Emit("error", err)
			continue
		}
		fragment, err := readBytes(socket.brw, payloadLen)
		if err != nil {
			socket.ee.Emit("error", err)
			continue
		}

		for i := range fragment {
			fragment[i] ^= mask[i%4]
		}

		message = append(message, fragment...)

		if endOfMessage {
			socket.ee.Emit("message", string(message))
			message = []byte{}
		}
	}
}

func (socket *Socket) Send(message string) {
	for _, fragment := range socket.toFragments(message) {
		socket.brw.Write(fragment)
		socket.brw.Flush()
	}
}

func (socket *Socket) toFragments(message string) [][]byte {
	messageByte := []byte(message)

	messageLen := len(messageByte)
	unreadedMessageLen := messageLen

	fragmentCountF := math.Ceil(float64(messageLen) / float64(socket.writePayloadSize))
	fragmentCount := int(fragmentCountF)
	fragments := [][]byte{}

	for currentFragmentIndex := range fragmentCount {
		var firstByteInt uint8 = 0
		var secondByteInt uint8 = 0

		if currentFragmentIndex == fragmentCount-1 {
			firstByteInt += finalFragmentOfMessage
		}
		if currentFragmentIndex == 0 {
			firstByteInt += 1 // set fragment type to text if first fragment otherwise continuation
		}

		fragmentSize := socket.writePayloadSize
		if unreadedMessageLen < socket.writePayloadSize {
			fragmentSize = unreadedMessageLen
		}

		headerSize := 2
		payloadLenSize := 0

		if fragmentSize <= payload0Max {
			secondByteInt = uint8(fragmentSize)
		} else if fragmentSize <= byte2MaxValue {
			secondByteInt = payloadNext2Byte
			payloadLenSize = 2
		} else {
			secondByteInt = payloadNext8Byte
			payloadLenSize = 8
		}

		headerSize += payloadLenSize

		header := make([]byte, headerSize)
		header[0] = byte(firstByteInt)
		header[1] = byte(secondByteInt)
		if payloadLenSize == 2 {
			binary.BigEndian.PutUint16(header[2:], uint16(fragmentSize))
		} else if payloadLenSize == 8 {
			binary.BigEndian.PutUint64(header[2:], uint64(fragmentSize))
		}
		taken := messageByte[messageLen-unreadedMessageLen : messageLen-unreadedMessageLen+fragmentSize]
		fragment := make([]byte, fragmentSize+headerSize)
		for i, b := range header {
			fragment[i] = b
		}

		for i, b := range taken {
			fragment[i+headerSize] = b
		}

		fragments = append(fragments, fragment)
		unreadedMessageLen = unreadedMessageLen - fragmentSize
	}

	return fragments
}

func readBytes(reader *bufio.ReadWriter, n int) ([]byte, error) {
	buf := make([]byte, n)
	_, err := io.ReadFull(reader, buf)
	if err != nil {
		return buf, err
	}
	return buf, nil
}

func valueOfBit(b byte, index int) bool {
	return (b & (1 << index)) != 0
}

func fragmentType(b byte) int {
	last4Bite := extractRight4Bit(b)
	return int(last4Bite)
}

func extractRight4Bit(b byte) byte {
	return (b & 0x0F)
}

func randomID() int {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomInt := r.Intn(1024 * 1024)
	return randomInt
}
