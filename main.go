package main

import (
    "fmt"
    "os"
    "strings"
    "bittorrent/bencode"
)

func main() {
    if len(os.Args) < 2 {
        fmt.Println("usage: go run main.go <torrent_file>")
        return
    }

    data, err := os.ReadFile(os.Args[1])
    if err != nil {
        fmt.Println("error reading file:", err)
        return
    }

    val , err := bencode.Decode(strings.NewReader(string(data)))
    if err != nil {
        fmt.Println("error parsing:" ,err)
        return
    }
    fmt.Printf("%#v\n", val)
}