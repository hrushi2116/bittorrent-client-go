package main

import (
	"bittorrent/torrent"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: go run main.go <torrent_file>")
		return
	}

	tf, err := torrent.Open(os.Args[1])
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("Announce:", tf.Announce)
	fmt.Println("Name:", tf.Info.Name)
	fmt.Println("Size:", tf.Info.Length)
	fmt.Printf("Info Hash: %x\n", tf.InfoHash)

	peers, err := torrent.GetPeers(tf, tf.PeerId, 6881)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("Found %d peers\n", len(peers))

	err = torrent.DownloadParallel(tf, peers)
	if err != nil {
		fmt.Println("Download failed:", err)
		return
	}
	fmt.Println("Download complete!")

}
