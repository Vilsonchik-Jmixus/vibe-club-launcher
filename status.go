package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"time"
)

type ServerStatus struct {
	Online  bool   `json:"online"`
	Players int    `json:"players"`
	Max     int    `json:"max"`
	Version string `json:"version"`
	Checked int64  `json:"checked"`
}

var (
	statusMu    sync.Mutex
	statusCache ServerStatus
)

func serverStatus(m *Manifest) ServerStatus {
	statusMu.Lock()
	if time.Since(time.Unix(statusCache.Checked, 0)) < 20*time.Second {
		s := statusCache
		statusMu.Unlock()
		return s
	}
	statusMu.Unlock()
	st := ServerStatus{Checked: time.Now().Unix()}
	var targets []string
	if m != nil && m.Server.Address != "" {
		if _, addrs, err := net.LookupSRV("minecraft", "tcp", m.Server.Address); err == nil {
			for _, a := range addrs {
				targets = append(targets, net.JoinHostPort(trimDot(a.Target), strconv.Itoa(int(a.Port))))
			}
		}
	}
	if m != nil && m.Server.Host != "" {
		targets = append(targets, net.JoinHostPort(m.Server.Host, strconv.Itoa(m.Server.Port)))
	}
	for _, t := range targets {
		if s, err := pingServer(t); err == nil {
			st = s
			st.Checked = time.Now().Unix()
			break
		}
	}
	statusMu.Lock()
	statusCache = st
	statusMu.Unlock()
	return st
}

func trimDot(s string) string {
	if len(s) > 0 && s[len(s)-1] == '.' {
		return s[:len(s)-1]
	}
	return s
}

func writeVarInt(b *bytes.Buffer, v int) {
	u := uint32(v)
	for {
		if u&^0x7F == 0 {
			b.WriteByte(byte(u))
			return
		}
		b.WriteByte(byte(u&0x7F | 0x80))
		u >>= 7
	}
}

func readVarInt(r io.ByteReader) (int, error) {
	var v uint32
	for i := 0; i < 5; i++ {
		c, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		v |= uint32(c&0x7F) << (7 * i)
		if c&0x80 == 0 {
			return int(int32(v)), nil
		}
	}
	return 0, errors.New("varint too long")
}

// pingServer implements the Minecraft Server List Ping (status) protocol.
func pingServer(addr string) (ServerStatus, error) {
	var st ServerStatus
	conn, err := net.DialTimeout("tcp", addr, 4*time.Second)
	if err != nil {
		return st, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(6 * time.Second))
	host, portS, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(portS)

	var hs bytes.Buffer
	writeVarInt(&hs, 0x00)
	writeVarInt(&hs, 767) // 1.21.1
	writeVarInt(&hs, len(host))
	hs.WriteString(host)
	_ = binary.Write(&hs, binary.BigEndian, uint16(port))
	writeVarInt(&hs, 1)
	var pkt bytes.Buffer
	writeVarInt(&pkt, hs.Len())
	pkt.Write(hs.Bytes())
	writeVarInt(&pkt, 1)
	pkt.WriteByte(0x00)
	if _, err := conn.Write(pkt.Bytes()); err != nil {
		return st, err
	}
	br := bufio.NewReader(conn)
	if _, err := readVarInt(br); err != nil {
		return st, err
	}
	if id, err := readVarInt(br); err != nil || id != 0 {
		return st, errors.New("bad packet")
	}
	n, err := readVarInt(br)
	if err != nil || n <= 0 || n > 1<<21 {
		return st, errors.New("bad length")
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(br, buf); err != nil {
		return st, err
	}
	var resp struct {
		Version struct {
			Name string `json:"name"`
		} `json:"version"`
		Players struct {
			Max    int `json:"max"`
			Online int `json:"online"`
		} `json:"players"`
	}
	if err := json.Unmarshal(buf, &resp); err != nil {
		return st, err
	}
	st.Online = true
	st.Players = resp.Players.Online
	st.Max = resp.Players.Max
	st.Version = resp.Version.Name
	return st, nil
}
