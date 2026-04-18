package torrent

import (
	"bittorrent/bencode"
	"crypto/sha1"
	"os"
	"strings"
)

type TorrentFile struct {
	Announce string
	InfoHash [20]byte
	PeerId   [20]byte
	Info     TorrentInfo
}

type TorrentInfo struct {
	PieceLength int64
	Pieces      []byte
	Name        string
	Length      int64
}

func Open(path string) (TorrentFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TorrentFile{}, err
	}
	rawInfo := bencode.FindInfoBytes(data)
	val, err := bencode.Decode(strings.NewReader(string(data)))
	if err != nil {
		return TorrentFile{}, err
	}
	dict := val.(bencode.Dict)
	announce := dict["announce"].(bencode.Str)
	infoDict := dict["info"].(bencode.Dict)

	name := infoDict["name"].(bencode.Str)
	pieceLength := infoDict["piece length"].(bencode.Int)
	Length := infoDict["length"].(bencode.Int)
	Pieces := infoDict["pieces"].(bencode.Str)

	InfoHash := sha1.Sum(rawInfo)

	peerId := [20]byte{}
	copy(peerId[:], "-GO0001-000000000000")

	return TorrentFile{
		Announce: string(announce),
		InfoHash: InfoHash,
		PeerId:   peerId,
		Info: TorrentInfo{
			PieceLength: int64(pieceLength),
			Pieces:      []byte(Pieces),
			Name:        string(name),
			Length:      int64(Length),
		},
	}, nil
}
