package jtt809

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDisconnectInform(t *testing.T) {
	// 1007 主链路 disconnect notification
	// ErrorCode: 0x00 (Main link broken)
	msg := DisconnectInform{
		ErrorCode: DisconnectMainLinkBroken,
	}

	// 1. Check MsgID
	assert.Equal(t, MsgIDDisconnNotify, msg.MsgID())

	// 2. Encode
	data, err := msg.Encode()
	assert.NoError(t, err)
	assert.Equal(t, []byte{0x00}, data)

	// 3. Full Package Encode/Decode
	pkg := Package{
		Header: Header{
			MsgLength:    0, // Will be filled
			MsgSN:        1,
			BusinessType: MsgIDDisconnNotify,
			GNSSCenterID: 12345,
			Version:      Version{Major: 1, Minor: 0, Patch: 0},
			EncryptFlag:  0,
			EncryptKey:   0,
			Timestamp:    time.Unix(1600000000, 0),
		},
		Body: msg,
	}

	encoded, err := EncodePackage(pkg)
	assert.NoError(t, err)

	decodedFrame, err := DecodeFrame(encoded)
	assert.NoError(t, err)
	assert.Equal(t, MsgIDDisconnNotify, decodedFrame.Header.BusinessType)

	decodedMsg, err := ParseDisconnectInform(decodedFrame)
	assert.NoError(t, err)
	assert.Equal(t, DisconnectMainLinkBroken, decodedMsg.ErrorCode)
}

func TestDownDisconnectInform(t *testing.T) {
	// 9007 从链路 disconnect notification
	// ErrorCode: 0x00 (Unable to connect)
	msg := DownDisconnectInform{
		ErrorCode: DisconnectCannotConnectSub,
	}

	// 1. Check MsgID
	assert.Equal(t, MsgIDDownDisconnectInform, msg.MsgID())

	// 2. Encode
	data, err := msg.Encode()
	assert.NoError(t, err)
	assert.Equal(t, []byte{0x00}, data)

	// Test with ErrorCode: 0x01
	msg2 := DownDisconnectInform{ErrorCode: DisconnectSubLinkBroken}
	data2, err := msg2.Encode()
	assert.NoError(t, err)
	assert.Equal(t, []byte{0x01}, data2)

	// 3. Full Package Encode/Decode
	pkg := Package{
		Header: Header{
			MsgLength:    0,
			MsgSN:        2,
			BusinessType: MsgIDDownDisconnectInform,
			GNSSCenterID: 67890,
			Version:      Version{Major: 1, Minor: 0, Patch: 0},
			EncryptFlag:  0,
			EncryptKey:   0,
			Timestamp:    time.Unix(1600001000, 0),
		},
		Body: msg2,
	}

	encoded, err := EncodePackage(pkg)
	assert.NoError(t, err)

	decodedFrame, err := DecodeFrame(encoded)
	assert.NoError(t, err)
	assert.Equal(t, MsgIDDownDisconnectInform, decodedFrame.Header.BusinessType)

	decodedMsg, err := ParseDownDisconnectInform(decodedFrame)
	assert.NoError(t, err)
	assert.Equal(t, DisconnectSubLinkBroken, decodedMsg.ErrorCode)
}
