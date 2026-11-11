// mkicon writes a Windows COFF resource object (.syso) with an application icon.
// Usage: go run ./tools/mkicon out.syso icon16.png icon32.png ... (PNG-compressed icon images)
package main

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"os"
)

func main() {
	out := os.Args[1]
	var imgs [][]byte
	var sizes []int
	for _, p := range os.Args[2:] {
		b, err := os.ReadFile(p)
		if err != nil {
			panic(err)
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(b))
		if err != nil {
			panic(err)
		}
		imgs = append(imgs, b)
		sizes = append(sizes, cfg.Width)
	}
	n := len(imgs)
	le := binary.LittleEndian

	// group icon data
	var grp bytes.Buffer
	binary.Write(&grp, le, uint16(0))
	binary.Write(&grp, le, uint16(1))
	binary.Write(&grp, le, uint16(n))
	for i, b := range imgs {
		w := sizes[i]
		if w >= 256 {
			w = 0
		}
		grp.Write([]byte{byte(w), byte(w), 0, 0})
		binary.Write(&grp, le, uint16(1))
		binary.Write(&grp, le, uint16(32))
		binary.Write(&grp, le, uint32(len(b)))
		binary.Write(&grp, le, uint16(i+1))
	}

	// layout of the .rsrc section
	dirSize := func(entries int) int { return 16 + 8*entries }
	off := 0
	root := off
	off += dirSize(2)
	iconL2 := off
	off += dirSize(n)
	grpL2 := off
	off += dirSize(1)
	iconL3 := make([]int, n)
	for i := range iconL3 {
		iconL3[i] = off
		off += dirSize(1)
	}
	grpL3 := off
	off += dirSize(1)
	iconDE := make([]int, n)
	for i := range iconDE {
		iconDE[i] = off
		off += 16
	}
	grpDE := off
	off += 16
	dataOff := make([]int, n)
	for i, b := range imgs {
		off = (off + 7) &^ 7
		dataOff[i] = off
		off += len(b)
	}
	off = (off + 7) &^ 7
	grpData := off
	off += grp.Len()
	sec := make([]byte, (off+7)&^7)

	putDir := func(at, entries int) {
		le.PutUint16(sec[at+14:], uint16(entries))
	}
	putEnt := func(at, idx int, id uint32, target uint32) {
		p := at + 16 + 8*idx
		le.PutUint32(sec[p:], id)
		le.PutUint32(sec[p+4:], target)
	}
	const sub = 0x80000000
	putDir(root, 2)
	putEnt(root, 0, 3, sub|uint32(iconL2))
	putEnt(root, 1, 14, sub|uint32(grpL2))
	putDir(iconL2, n)
	for i := 0; i < n; i++ {
		putEnt(iconL2, i, uint32(i+1), sub|uint32(iconL3[i]))
		putDir(iconL3[i], 1)
		putEnt(iconL3[i], 0, 0x0409, uint32(iconDE[i]))
		le.PutUint32(sec[iconDE[i]:], uint32(dataOff[i]))
		le.PutUint32(sec[iconDE[i]+4:], uint32(len(imgs[i])))
		copy(sec[dataOff[i]:], imgs[i])
	}
	putDir(grpL2, 1)
	putEnt(grpL2, 0, 1, sub|uint32(grpL3))
	putDir(grpL3, 1)
	putEnt(grpL3, 0, 0x0409, uint32(grpDE))
	le.PutUint32(sec[grpDE:], uint32(grpData))
	le.PutUint32(sec[grpDE+4:], uint32(grp.Len()))
	copy(sec[grpData:], grp.Bytes())

	relocs := append(append([]int{}, iconDE...), grpDE)

	var f bytes.Buffer
	const hdr = 20 + 40
	rawPtr := hdr
	relPtr := rawPtr + len(sec)
	symPtr := relPtr + 10*len(relocs)
	// file header
	binary.Write(&f, le, uint16(0x8664))
	binary.Write(&f, le, uint16(1))
	binary.Write(&f, le, uint32(0))
	binary.Write(&f, le, uint32(symPtr))
	binary.Write(&f, le, uint32(1))
	binary.Write(&f, le, uint16(0))
	binary.Write(&f, le, uint16(0))
	// section header
	f.Write([]byte(".rsrc\x00\x00\x00"))
	binary.Write(&f, le, uint32(0))
	binary.Write(&f, le, uint32(0))
	binary.Write(&f, le, uint32(len(sec)))
	binary.Write(&f, le, uint32(rawPtr))
	binary.Write(&f, le, uint32(relPtr))
	binary.Write(&f, le, uint32(0))
	binary.Write(&f, le, uint16(len(relocs)))
	binary.Write(&f, le, uint16(0))
	binary.Write(&f, le, uint32(0x40000040))
	f.Write(sec)
	for _, r := range relocs {
		binary.Write(&f, le, uint32(r))
		binary.Write(&f, le, uint32(0))
		binary.Write(&f, le, uint16(3)) // IMAGE_REL_AMD64_ADDR32NB
	}
	// symbol table: .rsrc section symbol
	f.Write([]byte(".rsrc\x00\x00\x00"))
	binary.Write(&f, le, uint32(0))
	binary.Write(&f, le, int16(1))
	binary.Write(&f, le, uint16(0))
	f.WriteByte(3)
	f.WriteByte(0)
	binary.Write(&f, le, uint32(4)) // empty string table
	if err := os.WriteFile(out, f.Bytes(), 0o644); err != nil {
		panic(err)
	}
}
