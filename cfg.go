package main

import (
	"bytes"
	"errors"
	"hash/crc32"
	"machine"
	"strconv"
	"strings"
)

// 設定(CFG)の入れ物と、Flash への保存・読み込み。
// キーマップとは別の1セクタに、「名前=値」の文字を1行ずつ書く。
// 保存先は、使ってよい Flash 領域の「最後から2番目」の1セクタ(最後の1セクタはキーマップ用。flash.go)。
// 形式: マジック"TKC1"(4) + 版(1) + 文字の長さ(2) + 「名前=値\n」の並び + CRC32(4)。数値は小さい桁が先。
// 知らない名前は、読み込んでも捨てず、保存し直しても残す(ファームを新しくしても、古い設定が消えない)。

const (
	cfgMagic         = "TKC1"
	cfgFormatVersion = 1
	cfgHeaderSize    = 7    // マジック4 + 版1 + 文字の長さ2
	cfgBufSize       = 1024 // 書き込みバッファ。256の倍数(書き込みの単位)にすること
	cfgMaxEntries    = 32   // 設定の最大の個数(知らない名前も含む)
	cfgLnameMax      = 6    // レイヤー名の最大の文字数(仮。段階6で、OLED に実際に出して確かめる)
)

// 1セクタ(4096)に収まらなくなったら、コンパイルエラーにする。
var _ [flashSectorSize - cfgBufSize]byte

var cfgBuf [cfgBufSize]byte

// cfgKind は、設定の値の種類。種類ごとに、使える値が決まる。
type cfgKind uint8

const (
	cfgName    cfgKind = iota // レイヤー名: 英大文字・数字・-・_ で、cfgLnameMax 文字まで
	cfgPercent                // 0〜100 の整数
	cfgWord                   // 英小文字・数字・_ で、1〜12文字(効果の名前)
	cfgColor                  // 6桁の16進(保存するときは大文字にそろえる)
	cfgRotate                 // 0 / 90 / 180 / 270
	cfgTri                    // 空(無効)、または「1,2,3」の形(レイヤー番号3つ)
)

// cfgDef は、ファームが知っている設定1つぶん(名前・初期値・種類)。
type cfgDef struct {
	name string
	def  string
	kind cfgKind
}

// cfgEntry は、RAM や Flash にある、設定1つぶん(名前と値)。
type cfgEntry struct {
	name  string
	value string
}

var (
	cfgDefs   []cfgDef   // ファームが知っている設定の表(buildCfgDefs で作る)
	cfgNow    []cfgEntry // いま使っている設定(RAM)。表の名前が先頭に並び、そのあとに、知らない名前が続く
	cfgSource string     // 起動時に何を使ったか("flash" か "default:理由")。診断用
)

// レイヤーごとの色の初期値(仮。段階8の実機で決め直す)
var lcolorDefaults = [NumLayers]string{"2060FF", "FF8000", "20FF40", "FF2080"}

// buildCfgDefs は、設定の表を作る。名前の並びが、保存するときの並びになる。
func buildCfgDefs() {
	cfgDefs = cfgDefs[:0]
	for l := 0; l < NumLayers; l++ {
		// レイヤー名の初期値は、oled.go の表(L0 = MAC、L1 = EDIT。それ以外は空)
		cfgDefs = append(cfgDefs, cfgDef{"lname" + strconv.Itoa(l), oledLayerNames[l], cfgName})
	}
	cfgDefs = append(cfgDefs,
		cfgDef{"led", "13", cfgPercent}, // 仮。いまの ledLevel(32/255)に近い値
		cfgDef{"led_mode", "layer", cfgWord},
	)
	for l := 0; l < NumLayers; l++ {
		cfgDefs = append(cfgDefs, cfgDef{"lcolor" + strconv.Itoa(l), lcolorDefaults[l], cfgColor})
	}
	cfgDefs = append(cfgDefs,
		cfgDef{"oled", "100", cfgPercent},
		cfgDef{"rotate", "0", cfgRotate},
		cfgDef{"tri0", "", cfgTri},
	)
}

// cfgNormalize は、値が、その種類で使える値かを調べる。使えるなら、保存する形にそろえて返す。
func cfgNormalize(kind cfgKind, v string) (string, bool) {
	switch kind {
	case cfgName:
		if len(v) > cfgLnameMax {
			return "", false
		}
		for i := 0; i < len(v); i++ {
			c := v[i]
			if !(c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
				return "", false
			}
		}
		return v, true

	case cfgPercent:
		n, err := strconv.ParseUint(v, 10, 8)
		if err != nil || n > 100 {
			return "", false
		}
		return strconv.FormatUint(n, 10), true // "007" は "7" にそろえる

	case cfgWord:
		if len(v) == 0 || len(v) > 12 {
			return "", false
		}
		for i := 0; i < len(v); i++ {
			c := v[i]
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
				return "", false
			}
		}
		return v, true

	case cfgColor:
		if len(v) != 6 {
			return "", false
		}
		out := make([]byte, 6)
		for i := 0; i < 6; i++ {
			c := v[i]
			switch {
			case c >= '0' && c <= '9', c >= 'A' && c <= 'F':
				out[i] = c
			case c >= 'a' && c <= 'f':
				out[i] = c - 'a' + 'A' // 大文字にそろえる
			default:
				return "", false
			}
		}
		return string(out), true

	case cfgRotate:
		switch v {
		case "0", "90", "180", "270":
			return v, true
		}
		return "", false

	case cfgTri:
		if v == "" {
			return "", true // 空 = 無効
		}
		parts := strings.Split(v, ",")
		if len(parts) != 3 {
			return "", false
		}
		out := ""
		for i, p := range parts {
			n, err := strconv.ParseUint(p, 10, 8)
			if err != nil || n < 1 || n >= NumLayers { // レイヤー0は常に有効なので、1以上
				return "", false
			}
			if i > 0 {
				out += ","
			}
			out += strconv.FormatUint(n, 10)
		}
		return out, true
	}
	return "", false
}

// cfgDefIndex は、表(cfgDefs)の中の名前の位置を返す。知らない名前なら -1。
func cfgDefIndex(name string) int {
	for i := range cfgDefs {
		if cfgDefs[i].name == name {
			return i
		}
	}
	return -1
}

// cfgEntryIndex は、list の中の名前の位置を返す。なければ -1。
func cfgEntryIndex(list []cfgEntry, name string) int {
	for i := range list {
		if list[i].name == name {
			return i
		}
	}
	return -1
}

// cfgEqual は、2つの設定の並びが、まったく同じかを返す。
func cfgEqual(a, b []cfgEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// cfgBuild は、Flash から読んだ設定(loaded)を、使える形に整える。
// 表の名前は、初期値から始めて、読んだ値が使えるものだけ入れる(使えない値は、初期値のまま)。
// 知らない名前は、そのまま後ろに残す。
func cfgBuild(loaded []cfgEntry) []cfgEntry {
	out := make([]cfgEntry, 0, cfgMaxEntries)
	for _, d := range cfgDefs {
		out = append(out, cfgEntry{d.name, d.def})
	}
	for _, e := range loaded {
		if i := cfgDefIndex(e.name); i >= 0 {
			if v, ok := cfgNormalize(cfgDefs[i].kind, e.value); ok {
				out[i].value = v // out の i 番目は、表の i 番目と同じ名前
			}
			continue
		}
		if len(out) < cfgMaxEntries {
			out = append(out, e) // 知らない名前
		}
	}
	return out
}

// cfgGet は、いまの設定(RAM)の値を返す。知らない名前なら ok=false。
func cfgGet(name string) (string, bool) {
	i := cfgEntryIndex(cfgNow, name)
	if i < 0 {
		return "", false
	}
	return cfgNow[i].value, true
}

// cfgSet は、いまの設定(RAM)を書き換える。Flash には書かない。
// 空の値は、その名前の初期値に戻す。表にない名前や、使えない値は、エラー。
func cfgSet(name, value string) error {
	d := cfgDefIndex(name)
	if d < 0 {
		return errors.New("unknown name")
	}
	if value == "" {
		value = cfgDefs[d].def
	}
	v, ok := cfgNormalize(cfgDefs[d].kind, value)
	if !ok {
		return errors.New("bad value")
	}
	cfgNow[d].value = v
	return nil
}

// cfgFlashOffset は、設定用セクタの位置(Flashデータ領域の先頭からのバイト数)を返す。
// 最後から2番目の1セクタ。flash.go の flashOffset と同じ確認をする。怪しければエラーを返し、Flash には触らない。
func cfgFlashOffset() (int64, error) {
	start := machine.FlashDataStart()
	end := machine.FlashDataEnd()
	if machine.Flash.EraseBlockSize() != flashSectorSize {
		return 0, errors.New("unexpected erase size")
	}
	if end < start+2*flashSectorSize {
		return 0, errors.New("no flash area")
	}
	target := end - 2*flashSectorSize
	off := int64(target - start)
	if target%flashSectorSize != 0 || off%flashSectorSize != 0 {
		return 0, errors.New("unaligned flash")
	}
	if off+flashSectorSize > machine.Flash.Size() {
		return 0, errors.New("flash size mismatch")
	}
	return off, nil
}

// cfgEncode は、設定を保存形式のバイト列にして cfgBuf に入れる。入りきらなければ false。
func cfgEncode(list []cfgEntry) bool {
	b := cfgBuf[:]
	for i := range b {
		b[i] = 0xFF // 使わない部分は、消去済みと同じ値
	}
	p := cfgHeaderSize
	for _, e := range list {
		if p+len(e.name)+1+len(e.value)+1+4 > len(b) { // +4 は CRC32 の分
			return false
		}
		p += copy(b[p:], e.name)
		b[p] = '='
		p++
		p += copy(b[p:], e.value)
		b[p] = '\n'
		p++
	}
	copy(b[0:4], cfgMagic)
	b[4] = cfgFormatVersion
	n := p - cfgHeaderSize
	b[5] = byte(n)
	b[6] = byte(n >> 8)
	putLE32(b[p:], crc32.ChecksumIEEE(b[:p])) // マジックから文字の終わりまでの CRC32
	return true
}

// cfgDecode は、保存形式のバイト列を、設定の並びに戻す。無効ならエラー。
func cfgDecode(b []byte) ([]cfgEntry, error) {
	if len(b) < cfgHeaderSize+4 {
		return nil, errors.New("too short")
	}
	if string(b[0:4]) != cfgMagic {
		return nil, errors.New("bad magic")
	}
	if b[4] != cfgFormatVersion {
		return nil, errors.New("unsupported version")
	}
	n := int(b[5]) | int(b[6])<<8
	end := cfgHeaderSize + n
	if end+4 > len(b) {
		return nil, errors.New("bad length")
	}
	if crc32.ChecksumIEEE(b[:end]) != getLE32(b[end:]) {
		return nil, errors.New("bad crc")
	}
	list := make([]cfgEntry, 0, cfgMaxEntries)
	text := b[cfgHeaderSize:end]
	for len(text) > 0 {
		nl := bytes.IndexByte(text, '\n')
		if nl < 0 {
			return nil, errors.New("bad text")
		}
		line := text[:nl]
		text = text[nl+1:]
		eq := bytes.IndexByte(line, '=')
		if eq <= 0 {
			return nil, errors.New("bad line")
		}
		if len(list) >= cfgMaxEntries {
			return nil, errors.New("too many entries")
		}
		list = append(list, cfgEntry{string(line[:eq]), string(line[eq+1:])})
	}
	return list, nil
}

// cfgLoadFromFlash は、Flash から読んで、有効な設定の並びを返す(整える前のもの)。無効ならエラー。
// (RAM の設定には触らない)
func cfgLoadFromFlash() ([]cfgEntry, error) {
	off, err := cfgFlashOffset()
	if err != nil {
		return nil, err
	}
	if _, err := machine.Flash.ReadAt(cfgBuf[:], off); err != nil {
		return nil, errors.New("read failed")
	}
	return cfgDecode(cfgBuf[:])
}

// cfgSaveToFlash は、設定を Flash に保存する。
// 手順: バイト列を作る → セクタを消去 → 書き込み → 読み戻して確認。
func cfgSaveToFlash(list []cfgEntry) error {
	off, err := cfgFlashOffset()
	if err != nil {
		return err
	}
	if !cfgEncode(list) {
		return errors.New("too large")
	}
	if err := machine.Flash.EraseBlocks(off/flashSectorSize, 1); err != nil {
		return errors.New("erase failed")
	}
	if _, err := machine.Flash.WriteAt(cfgBuf[:], off); err != nil {
		return errors.New("write failed")
	}
	got, err := cfgLoadFromFlash() // 読み戻して、書いた内容と同じか確認する
	if err != nil {
		return errors.New("verify failed: " + err.Error())
	}
	if !cfgEqual(got, list) {
		return errors.New("verify mismatch")
	}
	return nil
}

// initCfg は、起動時に1回だけ呼ばれる。Flash に有効な設定があればそれを、なければ初期値を、RAM に入れる。
func initCfg() {
	buildCfgDefs()
	list, err := cfgLoadFromFlash()
	if err != nil {
		cfgNow = cfgBuild(nil)
		cfgSource = "default:" + err.Error()
		return
	}
	cfgNow = cfgBuild(list)
	cfgSource = "flash"
}
