package bencode

import "io"
func Decode(r io.Reader) (Value, error) {
    data, err := io.ReadAll(r)
    if err != nil {
        return nil, err
    }
    val, _ := parseValue(data)  // pass []byte directly
    return val, nil
}

func parseString(s []byte) (Str, []byte) {
	colon := 0
	for i := 0; i<len(s); i++{
		if s[i] == ':' {
			colon = i 
			break
		}
	}

	length := 0
	for i:=0; i<colon; i++{
		digit := int(s[i] - '0')
		length = length*10 + digit
	}

	str := make([]byte, length)
	for i:= colon +1; i< colon+1+length; i++{
		str[i-colon-1] = s[i]
	}
	remaining := make([]byte, len(s)-colon-1-length)
	for i := 0; i < len(remaining); i++ {
    remaining[i] = s[colon+1+length+i]
	}	

	return Str(str), remaining

}

func parseInt(s []byte) (Int ,[]byte){
	e := 0
	for i:=1; i<len(s); i++ {
		if s[i] == 'e' {
			e = i 
			break
		}
	}
	str := 0
	for i:= 1;i<e;i++ {
		digit := int(s[i] - '0')
		str = str*10 + digit
	}
	remaining := make([]byte, len(s)-e-1)
	for i := 0; i < len(remaining); i++ {
    remaining[i] = s[e+1+i]
	}
	return Int(str) ,remaining
}

func parseList(s []byte) (List , []byte){
	s = s[1:]
	var list List
	for s[0] != 'e' {
		val , rest := parseValue(s)
		list = append(list,val)
		s = rest
	}
	return list , s[1:]
}
func parseDict(s []byte) (Dict ,[]byte) {
	s = s[1:]
	dict := make(Dict)
	for s[0] != 'e' {
		key , rest1 := parseString(s)
		val , rest2 := parseValue(rest1)
		dict[string(key)] = val
		s = rest2
	}
	return dict, s[1:]
}

func parseValue(s []byte) (Value , []byte) {
	switch s[0] {
	case 'i' :
		return parseInt(s)
	case 'l' :
		return parseList(s)
	case 'd' :
		return parseDict(s)
	default :
		return parseString(s)
	}
}