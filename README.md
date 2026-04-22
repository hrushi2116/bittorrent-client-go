# BitTorrent Client in Go

A BitTorrent client built from scratch in Go — no libraries for the protocol itself, just the Go standard library.

![demo](assets/first.gif)

---

## What it does

- Parses `.torrent` files and bencode encoding from scratch
- Calculates SHA1 info hash for torrent identification
- Connects to HTTP trackers and retrieves real peer lists
- Establishes TCP connections with peers using the BitTorrent wire protocol
- Downloads file pieces concurrently from multiple peers using goroutines
- Verifies each piece with SHA1 hashing before writing to disk
- Live terminal UI showing download progress, speed, peers, and ETA

---

## Demo

```
Pieces:  50 / 8711
Speed:   14.9 KB/s
Peers:   5
ETA:     2478m 17s
Peer:    [2001:41d0:1004:265d::1]:31812
```

---

## Usage

**Build:**
```bash
git clone https://github.com/yourusername/bittorrent-client-go
cd bittorrent-client-go
go build .
```

**Run:**
```bash
go run main.go yourfile.torrent
```

**Requirements:**
- Go 1.21+
- A `.torrent` file with an HTTP tracker (UDP trackers not yet supported)

---

## Project Structure

```
bittorrent/
├── main.go              # Entry point and error handling
├── bencode/
│   ├── types.go         # Value, Str, Int, List, Dict types
│   └── parser.go        # Recursive bencode decoder + FindInfoBytes
└── torrent/
    ├── torrent.go       # Torrent file parsing, tracker communication
    └── download.go      # Peer wire protocol, piece downloading
```

---

## How it works

**1. Parse the `.torrent` file**

The `.torrent` file is bencode encoded — a simple serialization format with 4 types: strings, integers, lists, and dicts. I wrote a recursive descent parser from scratch that handles all 4 types and binary data correctly using `[]byte` throughout.

**2. Calculate the info hash**

The info hash is a SHA1 hash of the raw bytes of the `info` dictionary in the torrent file. This uniquely identifies the torrent. Getting this wrong means every tracker rejects you — I learned this the hard way when naive depth counting in `FindInfoBytes` produced the same wrong hash for every torrent.

**3. Talk to the tracker**

Send an HTTP GET request to the tracker URL with the info hash, peer ID, port, and download stats as query parameters. The tracker responds with a bencoded list of peers — their IP addresses and ports.

**4. Connect to peers**

Establish TCP connections to peers and perform the BitTorrent handshake:
```
1 byte   - 19 (length of protocol string)
19 bytes - "BitTorrent protocol"
8 bytes  - reserved (zeros)
20 bytes - info hash
20 bytes - peer ID
```
If the peer's info hash doesn't match — drop the connection immediately.

**5. Download pieces**

After handshake, exchange interested/unchoke messages, then request file pieces in 16KB blocks (protocol maximum). Each completed piece is SHA1 verified against the hash from the torrent file before writing to disk.

---

## What I learned

- How the BitTorrent protocol actually works at the byte level
- Go concurrency — goroutines and channels for parallel peer connections
- Binary protocol parsing — why `[]byte` matters and when `string` breaks things
- Real-world debugging — 25 bugs documented with root cause analysis
- Network programming — TCP connections, timeouts, peer state management

---

## Known limitations

- UDP trackers not yet supported (HTTP only)
- Single file torrents only (multi-file in progress)
- Download only, no seeding yet

---

## Bugs & fixes

Every bug I hit while building this is documented in [`BUGS_AND_FIXES.md`](BUGS_AND_FIXES.md) with root cause analysis and what I learned from each one. 25 bugs total ranging from binary encoding issues to peer protocol edge cases.

---

## Built with

- Go standard library — `net`, `net/http`, `crypto/sha1`, `bytes`, `net/url`
