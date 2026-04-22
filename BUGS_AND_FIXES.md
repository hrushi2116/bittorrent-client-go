# BitTorrent Client — Bugs & Fixes Log

> A personal record of every mistake made and fixed while building this project.
> Keep this. Read it before interviews. These are your talking points.

---

## Bug 1 — Wrong Input Type: `string` instead of `[]byte`

### Where
`bencode/parser.go` — `Decode` function input
`torrent/torrent.go` — file reading and passing data to decoder

### What Was Wrong
```go
// WRONG
val, err := bencode.Decode(strings.NewReader(string(data)))
```
Raw `.torrent` file bytes were being converted to `string` before parsing.

### Why It's a Problem
Go strings are UTF-8. Real `.torrent` files contain raw binary data — especially
the `pieces` field which is concatenated SHA1 hashes. Converting binary bytes to
a UTF-8 string corrupts the data or causes encoding errors.

### Fix
```go
// CORRECT
val, err := bencode.Decode(bytes.NewReader(data))
```
Use `bytes.NewReader` for `[]byte` input instead of `strings.NewReader`.

### Lesson
Always use `[]byte` for binary data — network packets, file bytes, protocol data.
Only convert to `string` when you explicitly know the data is valid text.

---

## Bug 2 — Unused Variables Causing Compile Error

### Where
`bencode/parser.go` — `parseInt` function

### What Was Wrong
```go
negative := false
start := 1
if s[1] == '-' {
    negative = true
    start = 2
}
// variables declared but never actually used below
```

### Why It's a Problem
Go strictly forbids declaring variables without using them. Code won't compile.

### Fix
```go
num := 0
for i := start; i < e; i++ {  // use start here
    digit := int(s[i] - '0')
    num = num*10 + digit
}
if negative {                   // use negative here
    num = -num
}
```

### Lesson
Go's strict unused variable rule is a feature — it forces clean code.
Always trace every declared variable to where it's actually used.

---

## Bug 3 — `parseInt` Doesn't Handle Negative Numbers

### Where
`bencode/parser.go` — `parseInt` function

### What Was Wrong
```go
// only handles positive numbers
for i := 1; i < e; i++ {
    digit := int(s[i] - '0')  // '-' character gives garbage value here
    str = str*10 + digit
}
```

### Why It's a Problem
Bencode represents `-42` as `i-42e`. The `-` character is not a digit.
`'-' - '0'` produces a wrong numeric value silently.

### Fix
```go
negative := false
start := 1
if s[1] == '-' {
    negative = true
    start = 2
}
num := 0
for i := start; i < e; i++ {
    digit := int(s[i] - '0')
    num = num*10 + digit
}
if negative {
    num = -num
}
```

### Lesson
Always think about edge cases — negative numbers, empty strings, zero values.
What does your parser do with unexpected input?

---

## Bug 4 — `FindInfoBytes` Using Naive Depth Counting (FIXED WITH PARSER)

### Original Bug Code
```go
// counting raw 'd' and 'e' bytes to track dict depth
if data[i] == 'd' {
    depth++
} else if data[i] == 'e' {
    depth--
}
```

### Why It's a Problem
Bencode strings can *contain* the letters `d` and `e` in their content.
For example `5:dance` contains a `d` — the depth counter increments wrongly.

### Fix - Using parseValue (correct way)
```go
func FindInfoBytes(data []byte) []byte {
    // Find "4:info" key
    idx := -1
    for i := 0; i < len(data)-6; i++ {
        if data[i] == '4' && data[i+1] == ':' &&
            data[i+2] == 'i' && data[i+3] == 'n' &&
            data[i+4] == 'f' && data[i+5] == 'o' {
            idx = i
            break
        }
    }
    if idx == -1 {
        return nil
    }

    // Start after "4:info" (6 bytes)
    valueStart := idx + 6

    // Use parseValue to find where info dict ends
    _, remaining := parseValue(data[valueStart:])
    
    // Calculate end position
    valueEnd := valueStart + (len(data) - valueStart - len(remaining))

    // Return info dict value
    return data[valueStart:valueEnd]
}
```

### Explanation of the parseValue Fix
- `parseValue()` returns: (parsedValue, remainingData)
- We ignore the parsed value with `_`
- We use remaining data length to calculate where value ends
- valueLength = totalLength - remainingLength
- This properly handles nested strings containing 'd' and 'e'

### Lesson
Use the parser instead of manual byte scanning! The parser understands the format correctly.

---

## Bug 4b — Same Issue: Depth Counting in General

### Where
`bencode/parser.go` — `FindInfoBytes` function

### What Was Wrong
```go
// counting raw 'd' and 'e' bytes to track dict depth
if data[i] == 'd' {
    depth++
} else if data[i] == 'e' {
    depth--
}
```

### Why It's a Problem
Bencode strings can *contain* the letters `d` and `e` in their content.
For example `5:dance` contains a `d` — the depth counter increments wrongly.
This caused `FindInfoBytes` to return wrong bytes, producing an incorrect
info hash. The same wrong info hash appeared for every torrent file.

### Fix
Use the actual bencode parser to skip over values instead of scanning bytes:
```go
func FindInfoBytes(data []byte) []byte {
    for i := 0; i < len(data)-6; i++ {
        if data[i] == '4' && data[i+1] == ':' &&
            data[i+2] == 'i' && data[i+3] == 'n' &&
            data[i+4] == 'f' && data[i+5] == 'o' {
            start := i + 6
            _, remaining := parseValue(data[start:])
            end := len(data[start:]) - len(remaining)
            return data[start : start+end]
        }
    }
    return nil
}
```

### Lesson
Never scan binary/structured data character by character when you have a proper
parser available. Use the parser — it already understands the format correctly.

### Impact
This was the most critical bug. It caused every torrent to produce the same
wrong info hash (`8ba71c3cc8b00bebae90d87c4bca83d8f1d750ba`) and every
tracker to reject the request with "not authorized".

---

## Bug 5 — Manual URL Encoding Instead of Standard Library

### Where
`torrent/torrent.go` — `GetPeers` function

### What Was Wrong
```go
// manual percent encoding
infoHashEncoded := ""
for _, b := range tf.InfoHash {
    infoHashEncoded += "%" + fmt.Sprintf("%02x", b)
}
```

### Why It's a Problem
Manual encoding works but is fragile — easy to miss edge cases.
Go's standard library already handles this correctly and safely.

### Fix
```go
import "net/url"

infoHashEncoded := url.QueryEscape(string(tf.InfoHash[:]))
peerIdEncoded := url.QueryEscape(string(peerId[:]))
```

### Lesson
Don't reinvent what the standard library already does. Know your tools.
`net/url`, `crypto/sha1`, `bytes`, `encoding/binary` — learn what's available.

---

## Bug 6 — Assuming Compact Peer Format Only

### Where
`torrent/torrent.go` — `GetPeers` peer parsing

### What Was Wrong
```go
// only handled compact format (6 bytes per peer)
peersData := []byte(peersVal.(bencode.Str))
for i := 0; i < len(peersData); i += 6 {
    // parse 4 bytes IP + 2 bytes port
}
```

### Why It's a Problem
BitTorrent trackers can return peers in two formats:
- **Compact** — binary string, 6 bytes per peer (4 IP + 2 port)
- **Dictionary** — list of dicts with `ip`, `port`, `peer id` keys

The Ubuntu tracker returned dictionary format. The code panicked with:
`interface conversion: bencode.Value is bencode.List, not bencode.Str`

### Fix
```go
switch p := peersVal.(type) {
case bencode.Str:
    // compact format
    peersData := []byte(p)
    for i := 0; i < len(peersData)-5; i += 6 {
        ip := fmt.Sprintf("%d.%d.%d.%d",
            peersData[i], peersData[i+1],
            peersData[i+2], peersData[i+3])
        port := int(peersData[i+4])*256 + int(peersData[i+5])
        peerList = append(peerList, ip+":"+fmt.Sprintf("%d", port))
    }
case bencode.List:
    // dictionary format
    for _, peer := range p {
        peerDict := peer.(bencode.Dict)
        ip := string(peerDict["ip"].(bencode.Str))
        port := int(peerDict["port"].(bencode.Int))
        peerList = append(peerList, ip+":"+fmt.Sprintf("%d", port))
    }
}
```

### Lesson
Always check the protocol spec for all valid response formats.
Real world data doesn't always match the happy path you coded for.

---

## Bug 7 — IPv6 Addresses Formatted Without Brackets

### Where
`torrent/torrent.go` — `GetPeers` dictionary format parsing

### What Was Wrong
```go
// produces: 2a01:4f8:c17:116d::2:6953
// port looks like part of the IPv6 address
address := ip + ":" + fmt.Sprintf("%d", port)
```

### Why It's a Problem
IPv6 addresses contain colons. Appending `:port` directly makes it
ambiguous — is `6953` the port or part of the IPv6 address?
Standard format requires brackets around IPv6: `[2a01:4f8::2]:6953`

### Fix
```go
if strings.Contains(ip, ":") {
    peerList = append(peerList, "["+ip+"]:"+fmt.Sprintf("%d", port))
} else {
    peerList = append(peerList, ip+":"+fmt.Sprintf("%d", port))
}
```

### Lesson
IPv6 addresses in URLs and host:port strings always need brackets.
Network programming has many format conventions — read them carefully.

---

## Bug 8 — Using Wrong Torrent File for Testing

### What Happened
Tested with an Arch Linux torrent that had no `announce` field —
only a `url-list` field containing mirror download URLs, not tracker URLs.
The code sent a BitTorrent tracker request to a web mirror which
responded with an HTML page instead of a bencoded peer list.

### Symptoms
```
tracker response start: "<!DOCTYPE html>\n<html class=\"no-js\">\n"
panic: makeslice: len out of range
```

### Fix
Always verify the `Announce` field before testing:
- Should contain `announce` in the URL
- Should look like: `http://tracker.opentrackr.org:1337/announce`
- Should NOT be a mirror URL like `https://mirror.aarnet.edu.au/...`

Use Ubuntu or Debian torrents for testing — they always have standard
public tracker URLs.

### Lesson
Test with the right input. A bug that looks like a code problem is
sometimes just wrong test data. Always validate your inputs first.

---

## Summary Table

| # | Bug | Impact | Root Cause |
|---|-----|--------|------------|
| 1 | `string` instead of `[]byte` | Binary data corruption | Not understanding Go string encoding |
| 2 | Unused variables | Compile error | Go strict variable rules |
| 3 | No negative number handling | Wrong parsed values | Missing edge case |
| 4 | Naive `d`/`e` depth counting | Wrong info hash for every torrent | Not using parser for structured data |
| 5 | Manual URL encoding | Fragile code | Not using standard library |
| 6 | Only compact peer format | Panic on real tracker response | Not reading full protocol spec |
| 7 | IPv6 without brackets | Wrong peer addresses | Network format convention |
| 8 | Wrong test torrent | False code bugs | Bad test data |
| 9 | Duplicate peer entries | Same IP printed twice | Adding to list twice |
| 10 | Type assertion without ok check | Panic when keys missing | Not checking if dict key exists |

---

## Bug 9 — Duplicate Peer Entries

### Where
`torrent/torrent.go` - GetPeers dictionary format parsing

### What Was Wrong
```go
peerList = append(peerList, ip+":"+fmt.Sprintf("%d", port))  // First append
if strings.Contains(ip, ":") {
    peerList = append(peerList, "["+ip+"]:"+fmt.Sprintf("%d", port))  // Second append!
}
```

### Why It's a Problem
Each peer IP was being added twice to the list - once before the if/else block and once inside. Results in duplicate addresses in output.

### Fix
Remove the first append, keep only the conditional version:
```go
if strings.Contains(ip, ":") {
    peerList = append(peerList, "["+ip+"]:"+fmt.Sprintf("%d", port))
} else {
    peerList = append(peerList, ip+":"+fmt.Sprintf("%d", port))
}
```

### Result
Now each peer appears only once in the list.

---

## Bug 10 — Type Assertion Without Ok Check

### Where
`torrent/torrent.go` - Parsing dict keys like "announce", "ip", "port"

### What Was Wrong
```go
announce := dict["announce"].(bencode.Str)  // panics if key missing!
ip := peerDict["ip"].(bencode.Str)         // panics if key missing!
port := peerDict["port"].(bencode.Int)    // panics if key missing!
```

### Why It's a Problem
If the key doesn't exist in the dictionary, Go panics with "interface conversion" error. Some torrents use different keys.

### Fix
Use the "comma ok" idiom:
```go
ipVal, ok := peerDict["ip"].(bencode.Str)
if !ok {
    continue  // skip this peer if no IP
}
ip := string(ipVal)

portVal, ok := peerDict["port"].(bencode.Int)
if !ok {
    continue  // skip this peer if no port
}
port := int(portVal)
```

### Alternative - Handle multiple keys
```go
var announce bencode.Str
if a, ok := dict["announce"]; ok {
    announce = a.(bencode.Str)
} else if ul, ok := dict["url-list"]; ok {
    // Some torrents use url-list instead
    list := ul.(bencode.List)
    if len(list) > 0 {
        announce = list[0].(bencode.Str)
    }
}
```

---

## Bug 11 — Peer Connection Handshake Implementation

### Where
`torrent/torrent.go` - Missing `ConnectToPeer` function for establishing TCP connections and performing BitTorrent handshake

### What Was Missing
No implementation for establishing TCP connections to peers and performing the BitTorrent protocol handshake, which is required to communicate with peers and download pieces.

### Why It's Needed
After getting peers from the tracker, the client must:
1. Establish TCP connection to each peer
2. Perform BitTorrent protocol handshake to verify identity and torrent match
3. Exchange messages (interested, unchoke, request, piece) to download data

### Fix - Implemented ConnectToPeer Function
```go
func ConnectToPeer(peerAddr string, infoHash [20]byte, peerId [20]byte) (*PeerConn, error) {
    // Build handshake: pstrlen(1) + pstr(19) + reserved(8) + info_hash(20) + peer_id(20) = 68 bytes
    handshake := make([]byte, 0, 68)
    handshake = append(handshake, byte(19))                                       // pstrlen
    handshake = append(handshake, []byte("BitTorrent protocol")...)               // pstr
    handshake = append(handshake, make([]byte, 8)...)                             // reserved
    handshake = append(handshake, infoHash[:]...)                                 // info_hash
    handshake = append(handshake, peerId[:]...)                                   // peer_id

    // Establish TCP connection
    conn, err := net.Dial("tcp", peerAddr)
    if err != nil {
        return nil, err
    }

    // Send handshake
    _, err = conn.Write(handshake)
    if err != nil {
        conn.Close()
        return nil, err
    }

    // Read handshake response (exactly 68 bytes)
    response := make([]byte, 68)
    _, err = io.ReadFull(conn, response)
    if err != nil {
        conn.Close()
        return nil, err
    }

    // Validate response
    if response[0] != 19 {
        conn.Close()
        return nil, fmt.Errorf("invalid pstrlen: %d", response[0])
    }
    if string(response[1:20]) != "BitTorrent protocol" {
        conn.Close()
        return nil, fmt.Errorf("invalid protocol string")
    }
    remoteInfoHash := response[28:48]
    if !bytes.Equal(remoteInfoHash, infoHash[:]) {
        conn.Close()
        return nil, fmt.Errorf("info hash mismatch")
    }

    // Return connection state
    return &PeerConn{
        Conn:       conn,
        InfoHash:   infoHash,
        PeerId:     peerId,
        Choked:     true,        // Start choked (peer won't send data unless we're interested)
        Unchoked:   false,
        Interested: false,
    }, nil
}
```

### Key Implementation Details:
- **Handshake Structure**: 1+19+8+20+20 = 68 bytes total
- **Validation**: Checks pstrlen, pstr, and info_hash match
- **State Tracking**: Tracks choked/unchoked and interested states
- **Error Handling**: Properly closes connection on any error

### Lessons:
- The BitTorrent handshake is strict - both peers must send identical 68-byte messages
- Proper validation prevents connecting to wrong torrents or malicious peers
- State tracking is essential for the message exchange phase that follows handshake

---

## Bug 12 — Missing PeerConn Struct Definition

### Where
`torrent/torrent.go` - Missing PeerConn struct definition at package level

### What Was Missing
The PeerConn struct was initially defined inside the ConnectToPeer function, making it inaccessible to other functions.

### Why It's Needed
The PeerConn struct needs to be accessible throughout the package to track connection state during message exchange.

### Fix - Defined PeerConn Struct at Package Level:
```go
type PeerConn struct {
    Conn   net.Conn
    InfoHash [20]byte
    PeerId   [20]byte
    Choked   bool
    Unchoked bool
    Interested bool
}
```

### Lesson:
Structs that need to be used across multiple functions must be defined at package level, not inside functions.

---

## What To Say In An Interview

> "I built a BitTorrent client in Go from scratch. The most interesting bug
> I hit was in my `FindInfoBytes` function — I was naively scanning for `d`
> and `e` bytes to track dictionary depth, but bencode strings can contain
> those characters in their content. Every torrent produced the same wrong
> info hash and every tracker rejected me. The fix was to use my actual
> bencode parser to properly skip over values instead of scanning raw bytes."

That's a real answer about a real bug you actually fixed. That's gold in an interview.

---

## Bug 13 — Requesting Full Piece Instead of Blocks

### Where
`torrent/download.go` - Download function

### What Was Wrong
```go
// Requesting full piece at once (e.g., 256KB)
err = sendRequest(pc.Conn, uint32(i), 0, uint32(pieceLength))
```

### Why It's a Problem
BitTorrent protocol doesn't support requesting full pieces. Maximum block size is 16KB (16384 bytes). Peer rejects or ignores oversized requests.

### Fix
```go
const blockSize = 16384
for offset := 0; offset < pieceLength; offset += blockSize {
    reqLen := blockSize
    if offset+reqLen > pieceLength {
        reqLen = pieceLength - offset
    }
    sendRequest(pc.Conn, uint32(i), uint32(offset), uint32(reqLen))
    // receive block and assemble
}
```

---

## Bug 14 — readPiece Missing Begin Offset Check

### Where
`torrent/download.go` - readPiece function

### What Was Wrong
Piece response includes begin offset but we returned data without tracking it. Data could be misaligned if peer sends blocks out of order.

### Fix
```go
func readPiece(conn net.Conn) (uint32, uint32, []byte, error) {
    // Returns: pieceIndex, begin, data, error
}
```

---

## Bug 15 — Duplicate Functions Across Files

### Where
`torrent/torrent.go` and `torrent/download.go`

### What Was Wrong
Download functions (sendIntrested, sendRequest, readPiece, etc.) were copied to both files, causing "redeclared in this block" compile errors.

### Fix
Kept only Open, GetPeers, ConnectToPeeer in torrent.go. All download functions in download.go only.

---

## Bug 16 — Multi-file Torrent Panic

### Where
`torrent/torrent.go` - Open function

### What Was Wrong
```go
Length := infoDict["length"].(bencode.Int)  // panics for multi-file torrents!
```

### Why It's a Problem
Multi-file torrents have "files" dict instead of "length". Code crashed.

### Fix
```go
var Length int64
if l, ok := infoDict["length"]; ok {
    Length = int64(l.(bencode.Int))
}
```

---

## Bug 17 — Bencode Parser Bounds Errors

### Where
`bencode/parser.go` - parseString, parseList, parseDict, parseValue

### What Was Wrong
No bounds checking - crashed on certain torrents with "makeslice: len out of range"

### Fix
Added length checks before make():
```go
if colon+1+length > len(s) {
    return Str(""), s
}
```

---

## Bug 18 — Peer IP Parsing Index Error

### Where
`torrent/torrent.go` - compact peer parsing

### What Was Wrong
```go
// peersData[i] used twice instead of incrementing
ip := fmt.Sprintf("%d.%d.%d.%d", peersData[i], peersData[i], ...)
```

### Fix
```go
ip := fmt.Sprintf("%d.%d.%d.%d", peersData[i], peersData[i+1], peersData[i+2], peersData[i+3])
```

---

## Bug 19 — Handshake Missing peer_id

### Where
`torrent/torrent.go` - ConnectToPeeer function

### What Was Wrong
Handshake included info_hash but not peer_id. Peers rejected connection.

### Fix
```go
handshake = append(handshake, InfoHash[:]...)
handshake = append(handshake, peerId[:]...)  // Added this
```

---

## Bug 20 — SetReadDeadline Causing Premature Failure

### Where
`torrent/download.go` - Download function

### What Was Wrong
Single 120-second deadline at start expired even while downloading. Caused EOF errors mid-download.

### Fix
Removed timeout for now - can add per-read timeout later:
```go
// pc.Conn.SetReadDeadline(time.Now().Add(120 * time.Second))
```

---

## Bug 21 — readPiece Only Reads from First Peer

### Where
`torrent/download.go` - DownloadParallel function

### What Was Wrong
```go
gotPiece, _, data, err := readPiece(peerConns[0].conn)  // Always peer 0!
```

### Why It's a Problem
Even when connected to 5 peers, only peer[0] was used for reading. Other peers sat idle.

### Fix
Round-robin reads:
```go
peerIdx := piece % len(peerConns)
err := sendRequest(peerConns[peerIdx].conn, uint32(piece), uint32(offset), uint32(reqLen))
_, _, data, _ := readPiece(peerConns[peerIdx].conn)
```

---

## Summary Table (Updated)

| # | Bug | Impact |
|---|-----|--------|
| 13 | Full piece request | Peer ignores requests |
| 14 | No begin offset tracking | Data misalignment |
| 15 | Duplicate functions | Compile errors |
| 16 | Multi-file torrent panic | Crash on certain torrents |
| 17 | Parser bounds errors | Crash on malformed input |
| 18 | Peer IP indexing | Wrong peer addresses |
| 19 | Missing peer_id in handshake | Peer rejects connection |
| 20 | Single timeout | Premature failure |
| 21 | readPiece only from peer[0] | Parallel peers not utilized |
| 22 | No DownloadParallel | Single peer only |
| 23 | strings.NewReader for tracker | Binary parse errors |
| 24 | Type assertion without ok | Panics on bad data |
| 25 | No network timeouts | Infinite hang |
| 26 | Peer failure = full restart | Re-downloading pieces |
| 27 | Sequential peer connection | Slow connect |
| 28 | readPiece no error handling | Panic/crash |
| 29 | verifyPiece type mismatch | Compile error |
| 30 | Stale deadline | Mid-download timeout |

---

## Bug 22 (Updated) — DownloadParallel Function

### Where
`torrent/download.go` - New function added

### What Was Missing
Only single-peer download existed. No parallel downloading from multiple peers.

### Fix - Implemented DownloadParallel Function
```go
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

	// Connect to all peers, send interested, wait for unchoke
	// Download pieces using round-robin: piece % activeConns
}
```

### Lesson
Parallel downloads require connecting to multiple peers, waiting for unchoke from each, then distributing work using modulo to ensure each piece comes from one peer.

---

## Bug 23 — Tracker Response Using strings.NewReader

### Where
`torrent/torrent.go` — GetPeers function

### What Was Wrong
```go
parsed, err := bencode.Decode(strings.NewReader(string(body)))
```

### Why It's a Problem
Body is binary data from tracker. Converting to string corrupts it. Use bytes.Reader instead.

### Fix
```go
parsed, err := bencode.Decode(bytes.NewReader(body))
```

---

## Bug 24 — Type Assertion Without OK Check (Peer Dict Format)

### Where
`torrent/torrent.go` — GetPeers dictionary format parsing

### What Was Wrong
```go
peerDict := peer.(bencode.Dict)
ip := string(peerDict["ip"].(bencode.Str))  // panics if key missing!
port := int(peerDict["port"].(bencode.Int))
```

### Why It's a Problem
If key missing or wrong type, panics. Some torrents use different keys or incomplete peer dicts.

### Fix
```go
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
```

---

## Bug 25 — Unresponsive Peers Cause Hang

### Where
`torrent/torrent.go` — ConnectToPeeer and download.go

### What Was Wrong
No timeouts set. Peers that accept TCP but don't respond to BitTorrent handshake cause infinite hang.

### Why It's a Problem
Client hangs forever waiting for peer response. Can't move to next peer.

### Fix - Connect Timeout
```go
import "time"

d := &net.Dialer{Timeout: 5 * time.Second}
conn, err := d.Dial("tcp", peerAdr)
if err != nil {
    return nil, err
}
conn.SetDeadline(time.Now().Add(10 * time.Second))
```

### Fix - Read Timeout in Download
```go
pc.Conn.SetDeadline(time.Now().Add(120 * time.Second))
```

### Result
- 5s connect timeout → dial fails fast
- 10s read timeout → handshake times out
- 120s per-read timeout during download
- Main.go automatically tries next peer on timeout

### Lesson
Always set timeouts for network operations. Real-world peers may be unreachable or malicious. Don't let one bad peer block the whole download.

---

## Bug 26 — Peer Failure Causes Full Restart

### Where
`torrent/download.go` — Download function

### What Was Wrong
When a peer failed, the download restarted from piece 0, re-downloading all already completed pieces.

### Why It's a Problem
The 582MB file had 3+ pieces already downloaded but when peer failed at piece 2, it restarted from piece 0 on the next peer, wasting bandwidth and time.

### Fix - Resume Download Logic
```go
func getDownloadedPieces(tf *TorrentFile) ([]bool, error) {
	info, err := os.Stat(tf.Info.Name)
	if err != nil {
		return downloaded, nil
	}
	downloadedSize := info.Size()
	for i := 0; i < numPieces; i++ {
		pieceStart := int64(i) * tf.Info.PieceLength
		if downloadedSize >= pieceStart+tf.Info.PieceLength {
			downloaded[i] = true
		}
	}
	return downloaded, nil
}
```

### Result
Now resumes from `Resuming from piece 5` instead of always piece 0.

---

## Bug 27 — Sequential Peer Connection Is Slow

### Where
`torrent/download.go` — DownloadParallel

### What Was Wrong
Connecting to 30 peers sequentially took forever - each peer had 10s timeout.

### Fix - Parallel Connection with Goroutines
```go
results := make(chan result, len(peerAddrs))
for _, addr := range peerAddrs {
	addr := addr
	go func() {
		pc, err := ConnectToPeeer(addr, tf.InfoHash, tf.PeerId)
		results <- result{pc, addr, err}
	}()
}
```

### Result
- 30 peers connect in parallel (seconds, not minutes)
- Only working peers are kept
- Dead peers fail fast

---

## Bug 28 — readPiece Has No Error Handling

### Where
`torrent/download.go` — readPiece function

### What Was Wrong
```go
io.ReadFull(conn, lengthBuf)  // ignored errors
io.ReadFull(conn, remaining) // ignored errors
```

### Fix
```go
_, err := io.ReadFull(conn, lengthBuf)
if err != nil {
    return 0, 0, nil, err
}
```

---

## Bug 29 — verifyPiece Type Mismatch

### Where
`torrent/download.go` — verifyPiece

### What Was Wrong
```go
hash := sha1.Sum(data)
return hash == expected  // can't compare [20]byte with []byte
```

### Fix
```go
hash := sha1.Sum(data)
return bytes.Equal(hash[:], expected)
```

---

## Bug 30 — Stale Deadline Causing Timeouts Mid-Download

### Where
`torrent/download.go` — DownloadParallel

### What Was Wrong
The 10s deadline from ConnectToPeeer expired during download, causing all requests to fail.

### Fix - Refresh Deadline Before Each Operation
```go
peerConns[peerIdx].Conn.SetDeadline(time.Now().Add(120 * time.Second))
err := sendRequest(...)
peerConns[peerIdx].Conn.SetDeadline(time.Now().Add(120 * time.Second))
_, _, data, err := readPiece(peerConns[peerIdx].Conn)
```

### Result
- 120s deadline refreshed before each request
- Download continues uninterrupted
