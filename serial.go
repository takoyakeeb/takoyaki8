package main

import "machine"

// USBシリアル(CDC)から1行ずつコマンドを受け取り、応答を返す部分。
// コマンドの中身の処理は protocol.go(handleCommand)が担当する。

// 1行の最大長(改行を含まない)。
// いちばん長い正しいコマンドは "SET 3 7 0x00A4" で14文字。64あれば十分。
const maxLineLen = 64

// lineState は、1バイト受け取った結果どうなったかを表す。
type lineState int

const (
	lineContinue lineState = iota // まだ1行の途中
	lineComplete                  // 1行が完成した
	lineTooLong                   // 長すぎる行が終わった(内容は捨てた)
)

// lineReader は、1バイトずつ届く文字を「1行」にまとめる。
type lineReader struct {
	buf      [maxLineLen]byte // 溜めている途中の文字
	n        int              // buf に溜まっている文字数
	overflow bool             // 長すぎる行を読み捨てている最中か
}

// feed は1バイトを受け取る。
// 行の終わり(\n または \r)が来たら、1行が完成したか(lineComplete)、
// 長すぎて捨てたか(lineTooLong)を返す。それ以外は lineContinue。
//
// \r も行の終わりとして扱うので、\r\n で終わる端末でも、\r だけの端末でも動く。
// (\r\n だと空の行がもう1つできるが、空の行は無視するので問題ない)
func (r *lineReader) feed(b byte) (string, lineState) {
	if b == '\n' || b == '\r' {
		tooLong := r.overflow
		line := ""
		if !tooLong {
			line = string(r.buf[:r.n])
		}
		r.n = 0
		r.overflow = false
		if tooLong {
			return "", lineTooLong
		}
		return line, lineComplete
	}

	if r.overflow {
		return "", lineContinue // 長すぎる行の残りは捨てる
	}
	if r.n >= maxLineLen {
		r.overflow = true // 上限を超えた。次の行の終わりまで捨てる
		return "", lineContinue
	}
	r.buf[r.n] = b
	r.n++
	return "", lineContinue
}

var reader lineReader

// pollSerial は、シリアルに届いている文字をすべて読み、
// 1行できるたびにコマンドとして処理して応答を返す。
// メインのループから毎回呼ぶ(待たずにすぐ戻る)。
func pollSerial() {
	for machine.Serial.Buffered() > 0 {
		b, err := machine.Serial.ReadByte()
		if err != nil {
			return
		}
		line, state := reader.feed(b)
		switch state {
		case lineComplete:
			// 空の行には応答しない(handleCommand が "" を返す)
			if resp := handleCommand(line); resp != "" {
				writeLine(resp)
			}
		case lineTooLong:
			writeLine("ERR BADARG line too long")
		}
	}
}

// writeLine は、1行(末尾に \n を付けて)シリアルに書く。
func writeLine(s string) {
	machine.Serial.Write([]byte(s + "\n"))
}
