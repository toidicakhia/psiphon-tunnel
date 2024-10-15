package wire

import (
	"bytes"

	"github.com/toidicakhia/psiphon-tunnel/psiphon/common/quic/gquic-go/internal/protocol"
)

// A Frame in QUIC
type Frame interface {
	Write(b *bytes.Buffer, version protocol.VersionNumber) error
	Length(version protocol.VersionNumber) protocol.ByteCount
}
