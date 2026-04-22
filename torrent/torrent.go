package torrent

import (
	"bittorrent/bencode"
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type PeerConn struct {
	Conn       net.Conn
	InfoHash   [20]byte
	PeerId     [20]byte
	Choked     bool
	Unchoked   bool
	Interested bool
}
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
	val, err := bencode.Decode(bytes.NewReader(data))
	if err != nil {
		return TorrentFile{}, err
	}
	dict := val.(bencode.Dict)

	var announce bencode.Str
	if a, ok := dict["announce"]; ok {
		announce = a.(bencode.Str)
	} else if ul, ok := dict["url-list"]; ok {
		list := ul.(bencode.List)
		if len(list) > 0 {
			announce = list[0].(bencode.Str)
		}
	}
	infoDict := dict["info"].(bencode.Dict)

	name := infoDict["name"].(bencode.Str)
	pieceLength := infoDict["piece length"].(bencode.Int)
	Pieces := infoDict["pieces"].(bencode.Str)

	InfoHash := sha1.Sum(rawInfo)

	peerId := [20]byte{}
	copy(peerId[:], "-GO0001-000000000000")

	var Length int64
	if l, ok := infoDict["length"]; ok {
		Length = int64(l.(bencode.Int))
	}

	return TorrentFile{
		Announce: string(announce),
		InfoHash: InfoHash,
		PeerId:   peerId,
		Info: TorrentInfo{
			PieceLength: int64(pieceLength),
			Pieces:      []byte(Pieces),
			Name:        string(name),
			Length:      Length,
		},
	}, nil
}

func GetPeers(tf TorrentFile, peerId [20]byte, port int) ([]string, error) {
	baseURL := tf.Announce

	InfoHashEncoded := url.QueryEscape(string(tf.InfoHash[:]))
	peerIdEncoded := url.QueryEscape(string(peerId[:]))

	query := "info_hash=" + InfoHashEncoded + "&peer_id=" + peerIdEncoded + "&port=6881&uploaded=0&downloaded=0&left=" + fmt.Sprintf("%d", tf.Info.Length) + "&compact=1"

	url := baseURL + "?" + query

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("bad status: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	parsed, err := bencode.Decode(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse error: %v", err)
	}
	dict := parsed.(bencode.Dict)

	peersVal, ok := dict["peers"]
	if !ok {
		return nil, fmt.Errorf("no peers key in response")
	}
	peerList := []string{}

	switch p := peersVal.(type) {
	case bencode.Str:
		peersData := []byte(p)

		for i := 0; i < len(peersData)-5; i += 6 {
			ip := fmt.Sprintf("%d.%d.%d.%d", peersData[i], peersData[i+1], peersData[i+2], peersData[i+3])
			port := int(peersData[i+4])*256 + int(peersData[i+5])
			peerList = append(peerList, ip+":"+fmt.Sprintf("%d", port))
		}
	case bencode.List:
		for _, peer := range p {
			peerDict, ok := peer.(bencode.Dict)
			if !ok {
				continue
			}
			ipVal, ok := peerDict["ip"].(bencode.Str)
			if !ok {
				continue
			}
			ip := string(ipVal)
			portVal, ok := peerDict["port"].(bencode.Int)
			if !ok {
				continue
			}
			port := int(portVal)
			if strings.Contains(ip, ":") {
				peerList = append(peerList, "["+ip+"]:"+fmt.Sprintf("%d", port))
			} else {
				peerList = append(peerList, ip+":"+fmt.Sprintf("%d", port))
			}
		}
	}
	return peerList, nil
}

func ConnectToPeeer(peerAdr string, InfoHash [20]byte, peerId [20]byte) (*PeerConn, error) {
	d := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.Dial("tcp", peerAdr)
	if err != nil {
		return nil, err
	}
	conn.SetDeadline(time.Now().Add(10 * time.Second))

	handshake := make([]byte, 0, 68)
	handshake = append(handshake, byte(19))
	handshake = append(handshake, []byte("BitTorrent protocol")...)
	handshake = append(handshake, make([]byte, 8)...)
	handshake = append(handshake, InfoHash[:]...)
	handshake = append(handshake, peerId[:]...)

	_, err = conn.Write(handshake)
	if err != nil {
		conn.Close()
		return nil, err
	}
	response := make([]byte, 68)
	_, err = io.ReadFull(conn, response)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return &PeerConn{
		Conn:       conn,
		InfoHash:   InfoHash,
		PeerId:     peerId,
		Choked:     true,
		Unchoked:   false,
		Interested: false,
	}, nil
}
