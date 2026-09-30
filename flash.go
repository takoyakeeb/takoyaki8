package main

import (
	"errors"
	"hash/crc32"
	"machine"
)

// キーマップのFlash保存(tinykeemap の docs/protocol.md §4 の形式)。
//
// 保存先は、TinyGoが使ってよいと教えてくれるFlash領域の「最後の1セクタ(4096バイト)」。
// ファームの大きさが変わっても位置が動かない。ファーム本体には重ならない。
// 形式: マジック"TKM1"(4) + バージョン(1) + レイヤー数(1) + キー数(2) + keycode配列 + CRC32(4)
// 数値はリトルエンディアン(小さい桁が先)。

const (
	flashSectorSize  = 4096 // 消去できる最小単位
	flashPageSize    = 256  // 書き込みの単位
	flashHeaderSize  = 8
	flashPayloadSize = 2 * NumLayers * NumKeys
	flashDataSize    = flashHeaderSize + flashPayloadSize + 4 // 末尾の4はCRC32

	// 書き込みバッファの大きさ。flashDataSize を256の倍数に切り上げたもの。
	flashBufSize = (flashDataSize + flashPageSize - 1) / flashPageSize * flashPageSize

	flashMagic         = "TKM1"
	flashFormatVersion = 1
)

// キーマップが大きくなって1セクタに収まらなくなったら、ここでコンパイルエラーにする。
var _ [flashSectorSize - flashBufSize]byte

// 読み書きに使うバッファ。Flashに書くデータはRAM上に置く必要があるので、グローバル変数にしている。
var flashBuf [flashBufSize]byte

// flashOffset は、保存先のセクタの位置(Flashデータ領域の先頭からのバイト数)を返す。
// ここで「ファームと重ならない」「4096の境界にそろっている」ことを確認する。
// 少しでも怪しければエラーを返し、Flashには一切触らない。
func flashOffset() (int64, error) {
	start := machine.FlashDataStart() // ファーム末尾の直後
	end := machine.FlashDataEnd()     // 使ってよいFlashの終端
	if machine.Flash.EraseBlockSize() != flashSectorSize {
		return 0, errors.New("unexpected erase size")
	}
	if end < start+flashSectorSize {
		return 0, errors.New("no flash area")
	}
	target := end - flashSectorSize // 最後の1セクタ(絶対アドレス)
	off := int64(target - start)
	if target%flashSectorSize != 0 || off%flashSectorSize != 0 {
		return 0, errors.New("unaligned flash")
	}
	if off+flashSectorSize > machine.Flash.Size() {
		return 0, errors.New("flash size mismatch")
	}
	return off, nil
}

func putLE32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

func getLE32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

// encodeKeymap は、キーマップを保存形式のバイト列にして flashBuf に入れる。
// 使わない部分は 0xFF(消去済みの状態と同じ値)で埋める。
func encodeKeymap(km *Keymap) {
	b := flashBuf[:]
	for i := range b {
		b[i] = 0xFF
	}
	copy(b[0:4], flashMagic)
	b[4] = flashFormatVersion
	b[5] = NumLayers
	b[6] = byte(NumKeys)
	b[7] = byte(NumKeys >> 8)
	p := flashHeaderSize
	for l := 0; l < NumLayers; l++ {
		for k := 0; k < NumKeys; k++ {
			code := km[l][k]
			b[p] = byte(code)
			b[p+1] = byte(code >> 8)
			p += 2
		}
	}
	putLE32(b[p:], crc32.ChecksumIEEE(b[:p])) // マジックからkeycode配列までのCRC32
}

// decodeKeymap は、保存形式のバイト列をキーマップに戻す。
// protocol.md §4 の「無効」の条件のどれかに当てはまればエラーを返す。
func decodeKeymap(b []byte) (Keymap, error) {
	var km Keymap
	if string(b[0:4]) != flashMagic {
		return km, errors.New("bad magic")
	}
	if b[4] != flashFormatVersion {
		return km, errors.New("unsupported version")
	}
	if b[5] != NumLayers || int(b[6])|int(b[7])<<8 != NumKeys {
		return km, errors.New("layout mismatch")
	}
	end := flashHeaderSize + flashPayloadSize
	if crc32.ChecksumIEEE(b[:end]) != getLE32(b[end:]) {
		return km, errors.New("bad crc")
	}
	p := flashHeaderSize
	for l := 0; l < NumLayers; l++ {
		for k := 0; k < NumKeys; k++ {
			code := uint16(b[p]) | uint16(b[p+1])<<8
			if !validKeycode(code) { // CRCが合っていても、このファームが知らないkeycodeは無効
				return km, errors.New("undefined keycode")
			}
			km[l][k] = code
			p += 2
		}
	}
	return km, nil
}

// loadKeymapFromFlash は、Flashから読んで有効なキーマップを返す。無効ならエラー。
// (RAMのキーマップには触らない)
func loadKeymapFromFlash() (Keymap, error) {
	off, err := flashOffset()
	if err != nil {
		return Keymap{}, err
	}
	if _, err := machine.Flash.ReadAt(flashBuf[:], off); err != nil {
		return Keymap{}, errors.New("read failed")
	}
	return decodeKeymap(flashBuf[:])
}

// saveKeymapToFlash は、キーマップをFlashに保存する。
// 手順: バイト列を作る → セクタを消去 → 書き込み → 読み戻して確認。
// 消去中は時間がかかる(数十ms程度)。
func saveKeymapToFlash(km *Keymap) error {
	off, err := flashOffset()
	if err != nil {
		return err
	}
	encodeKeymap(km)
	if err := machine.Flash.EraseBlocks(off/flashSectorSize, 1); err != nil {
		return errors.New("erase failed")
	}
	if _, err := machine.Flash.WriteAt(flashBuf[:], off); err != nil {
		return errors.New("write failed")
	}
	got, err := loadKeymapFromFlash() // 読み戻して、書いた内容と同じか確認する
	if err != nil {
		return errors.New("verify failed: " + err.Error())
	}
	if got != *km {
		return errors.New("verify mismatch")
	}
	return nil
}
