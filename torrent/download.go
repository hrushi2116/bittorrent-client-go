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
		_, err := io.ReadFull(conn, lengthBuf)
		if err != nil {
			return 0, 0, nil, err
		}
		msgLength := binary.BigEndian.Uint32(lengthBuf)

		if msgLength == 0 {
			continue
		}

		remaining := make([]byte, msgLength)
		_, err = io.ReadFull(conn, remaining)
		if err != nil {
			return 0, 0, nil, err
		}

		switch remaining[0] {
		case 0, 1, 2, 3, 4, 5, 8:
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

func getDownloadedPieces(tf *TorrentFile) ([]bool, error) {
	numPieces := int(tf.Info.Length) / int(tf.Info.PieceLength)
	if tf.Info.Length%int64(tf.Info.PieceLength) != 0 {
		numPieces++
	}
	downloaded := make([]bool, numPieces)

	info, err := os.Stat(tf.Info.Name)
	if err != nil {
		return downloaded, nil
	}

	downloadedSize := info.Size()
	for i := 0; i < numPieces; i++ {
		pieceStart := int64(i) * tf.Info.PieceLength
		if downloadedSize >= pieceStart+tf.Info.PieceLength {
			downloaded[i] = true
		} else if downloadedSize >= pieceStart {
			break
		}
	}

	return downloaded, nil
}

func DownloadParallel(tf TorrentFile, peerAddrs []string) error {
	if len(peerAddrs) == 0 {
		return fmt.Errorf("no peers provided")
	}
	fmt.Printf("Connecting to %d peers...\n", len(peerAddrs))

	type result struct {
		pc   *PeerConn
		addr string
		err  error
	}

	results := make(chan result, len(peerAddrs))
	for _, addr := range peerAddrs {
		addr := addr
		go func() {
			pc, err := ConnectToPeeer(addr, tf.InfoHash, tf.PeerId)
			results <- result{pc, addr, err}
		}()
	}

	peerConns := make([]*PeerConn, 0, len(peerAddrs))
	for i := 0; i < len(peerAddrs); i++ {
		r := <-results
		if r.err != nil {
			continue
		}
		sendIntrested(r.pc.Conn)
		peerConns = append(peerConns, r.pc)
		fmt.Printf("  connected: %s\n", r.addr)
	}

	if len(peerConns) == 0 {
		return fmt.Errorf("could not connect to any peers")
	}
	fmt.Printf("Connected to %d peers\n", len(peerConns))

	activeConns := len(peerConns)
	numPieces := int(tf.Info.Length) / int(tf.Info.PieceLength)
	if tf.Info.Length%int64(tf.Info.PieceLength) != 0 {
		numPieces++
	}
	const blockSize = 16384

	downloaded, _ := getDownloadedPieces(&tf)
	startPiece := 0
	for i := 0; i < numPieces; i++ {
		if !downloaded[i] {
			startPiece = i
			break
		}
	}
	fmt.Printf("Resuming from piece %d\n", startPiece)

	for i := startPiece; i < numPieces; i++ {
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

			peerConns[peerIdx].Conn.SetDeadline(time.Now().Add(120 * time.Second))
			err := sendRequest(peerConns[peerIdx].Conn, uint32(i), uint32(offset), uint32(reqLen))
			if err != nil {
				return err
			}
			peerConns[peerIdx].Conn.SetDeadline(time.Now().Add(120 * time.Second))
			_, _, data, err := readPiece(peerConns[peerIdx].Conn)
			if err != nil {
				return err
			}
			pieceData = append(pieceData, data...)
			fmt.Printf("Got block: piece=%d, begin=%d, len=%d\n", i, offset, len(data))
		}

		if !verifyPiece(&tf, uint32(i), pieceData) {
			return fmt.Errorf("piece %d failed verification", i)
		}
		fmt.Printf("piece %d complete (%d bytes) - verified!\n", i, len(pieceData))
		err := writePiece(&tf, uint32(i), pieceData)
		if err != nil {
			return err
		}
	}

	for _, pc := range peerConns {
		pc.Conn.Close()
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
		if payload[0] == 1 {
			break
		}
	}
	numPieces := int(tf.Info.Length) / int(tf.Info.PieceLength)
	if tf.Info.Length%int64(tf.Info.PieceLength) != 0 {
		numPieces++
	}
	const blockSize = 16384

	downloaded, err := getDownloadedPieces(&tf)
	if err != nil {
		return err
	}
	startPiece := 0
	for i := 0; i < numPieces; i++ {
		if !downloaded[i] {
			startPiece = i
			break
		}
	}

	fmt.Printf("Resuming from piece %d\n", startPiece)

	for i := startPiece; i < numPieces; i++ {
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

			pc.Conn.SetDeadline(time.Now().Add(120 * time.Second))
			err = sendRequest(pc.Conn, uint32(i), uint32(offset), uint32(reqLen))
			if err != nil {
				return err
			}
			pc.Conn.SetDeadline(time.Now().Add(120 * time.Second))
			_, _, data, err := readPiece(pc.Conn)
			if err != nil {
				return err
			}
			pieceData = append(pieceData, data...)
			fmt.Printf("Got block: piece=%d, begin=%d, len=%d\n", i, offset, len(data))
		}

		if !verifyPiece(&tf, uint32(i), pieceData) {
			return fmt.Errorf("piece %d failed verification", i)
		}
		fmt.Printf("piece %d complete (%d bytes) - verified!\n", i, len(pieceData))
		err = writePiece(&tf, uint32(i), pieceData)
		if err != nil {
			return err
		}
	}
	return nil
}
