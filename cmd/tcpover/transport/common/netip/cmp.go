package netip

func Compare(x, y int) int {
	if x < y {
		return -1
	}
	if x > y {
		return +1
	}

	return 0
}

func CompareUint16(x, y uint16) int {
	if x < y {
		return -1
	}
	if x > y {
		return +1
	}

	return 0
}