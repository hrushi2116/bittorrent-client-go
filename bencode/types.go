package bencode
type Value interface{}
type Str string
type Int int64
type List []Value
type Dict map[string]Value
