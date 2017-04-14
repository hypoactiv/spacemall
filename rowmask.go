package game

import "fmt"

// A RowMask encodes a horizontal run of Locations with boolean values
// attached. It is used for painting tiles with RoomIds
type RowMask struct {
	Left  Location
	mask  []int // run lengths
	width int
	last  bool // the last value added
}

func (rm *RowMask) Reset() {
	rm.width = 0
	rm.mask = rm.mask[:1]
	rm.mask[0] = 0
	rm.last = false
}

func (rm *RowMask) Width() int {
	return rm.width
}

func (rm *RowMask) Append(v bool) {
	if v != rm.last {
		rm.last = v
		rm.mask = append(rm.mask, 0)
	}
	rm.mask[len(rm.mask)-1]++
	rm.width++
}

// Returns the mask value at position i, and the distance to the next mask
// value change (true to false or false to true), or the distance to the end
// of the RowMask if there are no more changes.
func (rm *RowMask) Mask(i int) (bool, int) {
	if i < 0 || i >= rm.width {
		fmt.Println(i, rm.width)
		panic("out of bounds")
	}
	m := rm.mask
	j := 0
	v := false
	for _, runLength := range m {
		j += runLength
		if j > i {
			break
		}
		v = !v
	}
	return v, j - i
	/*for j < len(m) && i >= m[j] {
		i -= m[j]
		j++
		v = !v
	}
	if j < len(m) {
		return v, m[j] - i + 1
	}
	return v, rm.width - i*/
}

func NewRowMask(width int, left Location) *RowMask {
	return &RowMask{
		Left:  left,
		mask:  make([]int, 1, width), // TODO experiment
		width: 0,
		last:  false,
	}
}

func (rm *RowMask) String() (out string) {
	for i := 0; i < rm.Width(); i++ {
		if v, _ := rm.Mask(i); v {
			out += fmt.Sprint("+")
		} else {
			out += fmt.Sprint(".")
		}
	}
	out += fmt.Sprintln()
	return
}
