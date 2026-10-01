package main

import (
	"strconv"
	"strings"
)

// tinykeemap との通信プロトコル(tinykeemap の docs/protocol.md、Draft v1)の実装。
// 1行のコマンドを受け取り、応答の1行(末尾の改行なし)を返す。
//
// INFO / GET / SET / DUMP はRAMのキーマップを読み書きする。
// SAVE / LOAD / RESET はRAMとFlashの間の受け渡し(Flashの処理は flash.go)。

// handleCommand は、1行のコマンドを処理して応答を返す。
// 空の行(何も書かれていない行)は、コマンドではないので "" を返す(応答しない)。
func handleCommand(line string) string {
	f := strings.Fields(line) // スペースで区切って単語に分ける
	if len(f) == 0 {
		return ""
	}

	// コマンド名は大文字のみ受け付ける(protocol.md の決まり)。小文字は BADCMD になる。
	switch f[0] {
	case "INFO":
		if len(f) != 1 {
			return usageError("INFO")
		}
		return "OK " + Info()

	case "GET":
		const usage = "GET <layer> <index>"
		if len(f) != 3 {
			return usageError(usage)
		}
		layer, ok1 := parseNumber(f[1])
		index, ok2 := parseNumber(f[2])
		if !ok1 || !ok2 {
			return usageError(usage)
		}
		if layer >= uint32(NumLayers) {
			return rangeError("layer", layer)
		}
		if index >= uint32(NumKeys) {
			return rangeError("index", index)
		}
		return "OK " + hex4(keymap[layer][index])

	case "SET":
		const usage = "SET <layer> <index> <keycode>"
		if len(f) != 4 {
			return usageError(usage)
		}
		layer, ok1 := parseNumber(f[1])
		index, ok2 := parseNumber(f[2])
		code, ok3 := parseKeycode(f[3])
		if !ok1 || !ok2 || !ok3 {
			return usageError(usage)
		}
		if layer >= uint32(NumLayers) {
			return rangeError("layer", layer)
		}
		if index >= uint32(NumKeys) {
			return rangeError("index", index)
		}
		if !validKeycode(code) {
			return "ERR BADARG undefined keycode"
		}
		keymap[layer][index] = code // RAMのキーマップを書き換える。すぐキー入力に反映される
		return "OK"

	case "DUMP":
		if len(f) != 2 {
			return usageError("DUMP <layer>")
		}
		layer, ok := parseNumber(f[1])
		if !ok {
			return usageError("DUMP <layer>")
		}
		if layer >= uint32(NumLayers) {
			return rangeError("layer", layer)
		}
		resp := "OK"
		for i := 0; i < NumKeys; i++ {
			resp += " " + hex4(keymap[layer][i])
		}
		return resp

	case "SAVE":
		if len(f) != 1 {
			return usageError("SAVE")
		}
		// 現在のキーマップをFlashに保存する。消去を伴うので少し時間がかかる。
		if err := saveKeymapToFlash(&keymap); err != nil {
			return "ERR FLASH " + err.Error()
		}
		return "OK"

	case "LOAD":
		if len(f) != 1 {
			return usageError("LOAD")
		}
		// Flashの内容を読み直す。未保存の変更は捨てる。
		// Flashの内容が無効なときは、RAMのキーマップを変えずにエラーを返す。
		km, err := loadKeymapFromFlash()
		if err != nil {
			return "ERR FLASH " + err.Error()
		}
		keymap = km
		return "OK"

	case "RESET":
		if len(f) != 1 {
			return usageError("RESET")
		}
		// 初期キーマップに戻す(RAMのみ)。Flashは SAVE するまで変わらない。
		keymap = defaultKeymap()
		return "OK"
	}

	return "ERR BADCMD unknown command"
}

// usageError は、引数の数や形式が正しくないときの応答を作る。
func usageError(usage string) string {
	return "ERR BADARG usage: " + usage
}

// rangeError は、layer や index が範囲外のときの応答を作る。
// 例: ERR RANGE layer 4 out of range
func rangeError(name string, v uint32) string {
	return "ERR RANGE " + name + " " + strconv.FormatUint(uint64(v), 10) + " out of range"
}

// parseNumber は、10進数の文字列(layer や index)を数値にする。
// 数字以外が混ざっている・大きすぎる場合は ok=false。
func parseNumber(s string) (n uint32, ok bool) {
	v, err := strconv.ParseUint(s, 10, 32)
	return uint32(v), err == nil
}

// parseKeycode は、"0x0004" のような16進の文字列をキーコードにする。
// "0x" で始まり、あとに16進数が1〜4桁続く形だけを受け付ける(a-f は大文字小文字どちらも可)。
func parseKeycode(s string) (code uint16, ok bool) {
	if len(s) < 3 || len(s) > 6 || s[0] != '0' || (s[1] != 'x' && s[1] != 'X') {
		return 0, false
	}
	v, err := strconv.ParseUint(s[2:], 16, 16)
	return uint16(v), err == nil
}

// validKeycode は、このファームが受け付けるキーコードかどうかを返す。
// 「何もしない」「透過」と、MO(n)(n はこのファームのレイヤー数より小さいもの)と、
// toHID で変換できるキーコード(基本キー・修飾キー単体)だけを認める。
// (TG / TO などの種類を足すときは、ここにも足す)
func validKeycode(code uint16) bool {
	if code == KC_NO || code == KC_TRANSPARENT {
		return true
	}
	if _, ok := momentaryLayer(code); ok {
		return true
	}
	_, ok := toHID(code)
	return ok
}

const hexDigits = "0123456789ABCDEF"

// hex4 は、キーコードを "0x0004" の形(0x + 大文字16進4桁)の文字列にする。
func hex4(v uint16) string {
	return "0x" + string([]byte{
		hexDigits[(v>>12)&0xF],
		hexDigits[(v>>8)&0xF],
		hexDigits[(v>>4)&0xF],
		hexDigits[v&0xF],
	})
}
