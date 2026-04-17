import (
	"crypto/sha1"
	"os"
	"bittorrent/bencode"
)

type TorrentFile struct {
	Announce string 
	InfoHash [20]byte
	PeerId   [20]byte
	Info     TorrentFile
}

type TorrentFile struct {
	PieceLength int64
	Pieces      []byte
	Name        string
	Length      int64
}