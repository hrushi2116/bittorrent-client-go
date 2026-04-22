package torrent

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

func sendIntrested(conn net.Conn) error {
	msg := make([]byte, 5)
	binary.BigEndian.PutUint32(msg[:4], 1)
	msg[4] = 2

	_, err := conn.Write(msg)
	return err
}
func sendRequest(conn net.Conn, index uint32, begin uint32, length uint32) error {
	msg := make([]byte, 17)
	binary.BigEndian.PutUint32(msg[0:4], 13)
	msg[4] = 6
	binary.BigEndian.PutUint32(msg[5:9], index)
	binary.BigEndian.PutUint32(msg[9:13], begin)
	binary.BigEndian.PutUint32(msg[13:17], length)
	_, err := conn.Write(msg)
	return err
}
func readPiece(conn net.Conn) (uint32, uint32, []byte, error) {
	for {
		lengthBuf := make([]byte, 4)
		io.ReadFull(conn, lengthBuf)
		msgLength := binary.BigEndian.Uint32(lengthBuf)

		if msgLength == 0 {
			continue
		}

		remaining := make([]byte, msgLength)
		io.ReadFull(conn, remaining)

		switch remaining[0] {
		case 0, 1, 2, 3, 4, 5, 8:
			fmt.Println("readPiece: msg id:", remaining[0])
			continue
		case 7:
			pieceIndex := binary.BigEndian.Uint32(remaining[1:5])
			begin := binary.BigEndian.Uint32(remaining[5:9])
			pieceData := remaining[9:]
			return pieceIndex, begin, pieceData, nil
		default:
			return 0, 0, nil, fmt.Errorf("unknown message id: %d", remaining[0])
		}
	}
}
func verifyPiece(tf *TorrentFile, pieceIndex uint32, data []byte) bool {
	expected := tf.Info.Pieces[pieceIndex*20 : pieceIndex*20+20]
	hash := sha1.Sum(data)
	return bytes.Equal(hash[:], expected)
}
func writePiece(tf *TorrentFile, pieceIndex uint32, data []byte) error {
	offset := int64(pieceIndex) * tf.Info.PieceLength
	f, err := os.OpenFile(tf.Info.Name, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	_, err = f.WriteAt(data, offset)
	f.Close()
	return err
}
func DownloadParallel(tf TorrentFile, peerAddrs []string) error {
	if len(peerAddrs) == 0 {
		return fmt.Errorf("no peers provided")
	}
	if len(peerAddrs) > 5 {
		peerAddrs = peerAddrs[:5]
	}

	peerConns := make([]*PeerConn, len(peerAddrs))
	for i, addr := range peerAddrs {
		pc, err := ConnectToPeeer(addr, tf.InfoHash, tf.PeerId)
		if err != nil {
			continue
		}
		peerConns[i] = pc
	}

	activeConns := 0
	for _, pc := range peerConns {
		if pc != nil {
			activeConns++
			sendIntrested(pc.Conn)
		}
	}
	if activeConns == 0 {
		return fmt.Errorf("could not connect to any peers")
	}

	for _, pc := range peerConns {
		if pc == nil {
			continue
		}
		for {
			msg := make([]byte, 4)
			io.ReadFull(pc.Conn, msg)
			length := binary.BigEndian.Uint32(msg)
			if length == 0 {
				continue
			}
			payload := make([]byte, length)
			io.ReadFull(pc.Conn, payload)
			if payload[0] == 1 {
				break
			}
		}
	}

	numPieces := int(tf.Info.Length) / int(tf.Info.PieceLength)
	if tf.Info.Length%int64(tf.Info.PieceLength) != 0 {
		numPieces++
	}
	const blockSize = 16384

	for i := 0; i < numPieces; i++ {
		pieceLength := int(tf.Info.PieceLength)
		if i == numPieces-1 && tf.Info.Length%int64(tf.Info.PieceLength) != 0 {
			pieceLength = int(tf.Info.Length % int64(tf.Info.PieceLength))
		}

		pieceData := make([]byte, 0, pieceLength)

		for offset := 0; offset < pieceLength; offset += blockSize {
			reqLen := blockSize
			if offset+reqLen > pieceLength {
				reqLen = pieceLength - offset
			}

			peerIdx := i % activeConns
			for peerConns[peerIdx] == nil {
				peerIdx = (peerIdx + 1) % activeConns
			}

			err := sendRequest(peerConns[peerIdx].Conn, uint32(i), uint32(offset), uint32(reqLen))
			if err != nil {
				return err
			}
			_, _, data, err := readPiece(peerConns[peerIdx].Conn)
			if err != nil {
				return err
			}
			pieceData = append(pieceData, data...)
		}

		if !verifyPiece(&tf, uint32(i), pieceData) {
			return fmt.Errorf("piece %d failed verification", i)
		}
		err := writePiece(&tf, uint32(i), pieceData)
		if err != nil {
			return err
		}
	}

	for _, pc := range peerConns {
		if pc != nil {
			pc.Conn.Close()
		}
	}
	return nil
}

func Download(tf TorrentFile, peerAddr string) error {
	pc, err := ConnectToPeeer(peerAddr, tf.InfoHash, tf.PeerId)
	if err != nil {
		return err
	}
	pc.Conn.SetDeadline(time.Now().Add(120 * time.Second))
	defer pc.Conn.Close()
	fmt.Println("Connected to peer")
	err = sendIntrested(pc.Conn)
	if err != nil {
		return err
	}
	fmt.Println("Sent interested")
	for {
		msg := make([]byte, 4)
		_, err := io.ReadFull(pc.Conn, msg)
		if err != nil {
			return err
		}
		length := binary.BigEndian.Uint32(msg)
		if length == 0 {
			continue
		}
		payload := make([]byte, length)
		_, err = io.ReadFull(pc.Conn, payload)
		if err != nil {
			return err
		}
		fmt.Printf("Unchoke loop: msg id=%d, len=%d\n", payload[0], length)
		if payload[0] == 1 {
			break
		}
	}
	numPieces := int(tf.Info.Length) / int(tf.Info.PieceLength)
	if tf.Info.Length%int64(tf.Info.PieceLength) != 0 {
		numPieces++
	}
	const blockSize = 16384

	for i := 0; i < numPieces; i++ {
		pieceLength := int(tf.Info.PieceLength)
		if i == numPieces-1 && tf.Info.Length%int64(tf.Info.PieceLength) != 0 {
			pieceLength = int(tf.Info.Length % int64(tf.Info.PieceLength))
		}

		pieceData := make([]byte, 0, pieceLength)

		for offset := 0; offset < pieceLength; offset += blockSize {
			reqLen := blockSize
			if offset+reqLen > pieceLength {
				reqLen = pieceLength - offset
			}

			err = sendRequest(pc.Conn, uint32(i), uint32(offset), uint32(reqLen))
			if err != nil {
				return err
			}
			_, begin, data, err := readPiece(pc.Conn)
			fmt.Printf("Got block: piece=%d, begin=%d, len=%d\n", i, begin, len(data))
			if err != nil {
				return err
			}
			pieceData = append(pieceData, data...)
		}

		fmt.Printf("Piece %d complete (len=%d)\n", i, len(pieceData))
		if !verifyPiece(&tf, uint32(i), pieceData) {
			return fmt.Errorf("piece %d failed verification", i)
		}
		err = writePiece(&tf, uint32(i), pieceData)
		if err != nil {
			return err
		}
	}
	return nil
}
