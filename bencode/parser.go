package bencode

import "io"

func Decode(r io.Reader) (Value, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	val, _ := parseValue(data)
	return val, nil
}

func parseString(s []byte) (Str, []byte) {
	colon := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			colon = i
			break
		}
	}

	length := 0
	for i := 0; i < colon; i++ {
		digit := int(s[i] - '0')
		length = length*10 + digit
	}

	str := make([]byte, length)
	for i := colon + 1; i < colon+1+length; i++ {
		str[i-colon-1] = s[i]
	}
	remaining := make([]byte, len(s)-colon-1-length)
	for i := 0; i < len(remaining); i++ {
		remaining[i] = s[colon+1+length+i]
	}

	return Str(str), remaining

}

func parseInt(s []byte) (Int, []byte) {
	e := 0
	for i := 1; i < len(s); i++ {
		if s[i] == 'e' {
			e = i
			break
		}
	}
	negative := false
	start := 1
	if s[1] == '-' {
		negative = true
		start = 2
	}
	str := 0
	for i := start; i < e; i++ {
		digit := int(s[i] - '0')
		str = str*10 + digit
	}
	if negative {
		str = -str
	}
	remaining := make([]byte, len(s)-e-1)
	for i := 0; i < len(remaining); i++ {
		remaining[i] = s[e+1+i]
	}
	return Int(str), remaining
}

func parseList(s []byte) (List, []byte) {
	s = s[1:]
	var list List
	for s[0] != 'e' {
		val, rest := parseValue(s)
		list = append(list, val)
		s = rest
	}
	return list, s[1:]
}
func parseDict(s []byte) (Dict, []byte) {
	s = s[1:]
	dict := make(Dict)
	for s[0] != 'e' {
		key, rest1 := parseString(s)
		val, rest2 := parseValue(rest1)
		dict[string(key)] = val
		s = rest2
	}
	return dict, s[1:]
}

func parseValue(s []byte) (Value, []byte) {
	switch s[0] {
	case 'i':
		return parseInt(s)
	case 'l':
		return parseList(s)
	case 'd':
		return parseDict(s)
	default:
		return parseString(s)
	}
}

func FindInfoBytes(data []byte) []byte {

	for i := 0; i < len(data)-6; i++ {
		if data[i] == '4' && data[i+1] == ':' && data[i+2] == 'i' && data[i+3] == 'n' && data[i+4] == 'f' && data[i+5] == 'o' {
			start := i + 6
		_, remaining := parseValue(data[start:])
		end := len(data[start:]) - len(remaining)
		return data[start:start+end]
		}
	}
	return nil
}
