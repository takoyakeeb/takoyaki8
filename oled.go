package main

// OLED(SSD1306、128x32、I2C)の表示。I2C1 を使う。ピンは config.go の pinSDA(GPIO14)/ pinSCL(GPIO15)。
//
// 表示するもの: いま「いちばん上」の有効なレイヤーの番号(大きく)と、短い名前(layer.go の currentLayer)。
// 基板では縦置き(USB-C を上にして置く)なので、画面を90°回して、縦に見たときに読める向きで描く。
// 描くときは、縦長の画面(幅32 x 高さ128)として座標を考え、oledPixel で実際の画面の座標に直す。
// tinykeemap からは設定できない(コマンドは作っていない)。レイヤーの名前は、このファームの中の固定の表。
//
// 画面は、変化がなくても oledEveryLoops ごとに全体を送る(fw 0.4.1 までの流れる印と、通信の量は同じ)。
// 送るのに失敗したときの対策(下の oledMaxFails など)も、そのまま。
//
// OLED がつながっていないとき、通信に失敗し続けるときは、OLED の更新をやめる。
// キー入力を邪魔しないことを、OLED の表示よりも優先する。

import (
	"image/color" // 色の型(画面の点を「点灯」にするために使う)
	"machine"
	"time" // 起動直後の待ち時間と、診断用の時間の計測に使う

	"tinygo.org/x/drivers/ssd1306" // SSD1306 用のドライバ
)

const (
	oledWidth  = 128 // 画面の幅(ドット。実際の画面)
	oledHeight = 32  // 画面の高さ(ドット。実際の画面)

	// 縦長の画面として考えたときの大きさ(USB-C を上にして置いたとき)
	oledLogicalW = oledHeight // 幅 32(高さは oledWidth の 128)

	// 表示の位置と大きさ(縦長の画面の座標。文字は、左右の中央にそろえる)
	oledLabelY     = 8  // 「LAYER」の上端の y
	oledDigitY     = 34 // レイヤーの番号の上端の y
	oledDigitScale = 5  // レイヤーの番号の拡大(5x7 の文字を5倍 = 幅25 x 高さ35)
	oledNameY      = 88 // 短い名前の上端の y

	// 画面を、180°回して表示する。上下が逆に見えたら true にする。
	oledFlip = false

	// 起動してから、OLED を使い始めるまでの待ち時間。
	// 待っている間も、キーボードとしては動く(メインループは止めない)。
	oledStartDelay = 1500 * time.Millisecond

	// 画面を送るのに、続けて何回失敗したら、OLED の更新をやめるか。
	// 1回の失敗で約0.4秒、メインループが止まるため、あまり増やさない。
	oledMaxFails = 3
)

var (
	oledDev     *ssd1306.Device // ドライバ(これを通して OLED に表示を送る)
	oledStartAt time.Time       // この時刻を過ぎたら、OLED を使い始める
	oledStarted bool            // 使い始める処理(oledStart)を、もう実行したか
	oledReady   bool            // OLED が見つかって、使える状態なら true
	oledFails   int             // 画面を送るのに、続けて失敗した回数(成功したら 0 に戻る)
	oledTicks   int             // メインループが何回まわったか(oledEveryLoops に達したら更新して0に戻す)
)

var oledWhite = color.RGBA{R: 255, G: 255, B: 255, A: 255} // 点灯させる色

// ===== 診断用(原因を調べ終わったら、このブロックと、呼び出している所を削除する) =====
// true にすると、約1秒ごとに、USBシリアルへ「# oled ...」の行を出す(キーイベントと同じ「# 」始まり)。
// 値は、起動してからの合計・最大値。
// 初期値は false。true にすると診断の行が tinykeemap の応答に混ざるので、tinykeemap を使うときは false のままにする。
const oledDiag = false

var (
	oledCount     int           // 画面を送った回数
	oledErrs      int           // そのうち、エラーが返った回数
	oledResets    int           // 失敗のあとに、I2C を設定し直した回数
	oledMaxDur    time.Duration // 画面1回を送るのにかかった最長の時間
	oledMaxGap    time.Duration // tickOLED が呼ばれる間隔(= メインループ1周ぶん)の最長
	oledLastErr   string        // 最後のエラーの内容
	oledPrevTick  time.Time     // 前回 tickOLED が呼ばれた時刻
	oledPrevPrint time.Time     // 前回、診断の行を出した時刻
)

// oledRecord は、画面を送った結果(かかった時間・エラー)を記録する。
func oledRecord(start time.Time, err error) {
	d := time.Since(start)
	oledCount++
	if d > oledMaxDur {
		oledMaxDur = d
	}
	if err != nil {
		oledErrs++
		oledLastErr = err.Error()
	}
}

// oledDiagPrint は、約1秒ごとに、診断の行を出す。
func oledDiagPrint() {
	if time.Since(oledPrevPrint) < time.Second {
		return
	}
	oledPrevPrint = time.Now()
	println("# oled ready", oledReady, "n", oledCount, "err", oledErrs, "resets", oledResets,
		"maxsend_us", d2us(oledMaxDur), "maxloop_us", d2us(oledMaxGap), "last", oledLastErr)
}

func d2us(d time.Duration) int64 { return int64(d / time.Microsecond) }

// 5x7 の文字の形(1文字 = 5列。1列は7ビットで、いちばん下のビットが画面の上端)。
// 「LAYER」・レイヤーの名前・番号に使う文字だけ入れてある。名前を変えるときは、必要な文字を足す。
var oledGlyphs = map[rune][5]byte{
	' ': {0x00, 0x00, 0x00, 0x00, 0x00},
	'0': {0x3E, 0x51, 0x49, 0x45, 0x3E},
	'1': {0x00, 0x42, 0x7F, 0x40, 0x00},
	'2': {0x42, 0x61, 0x51, 0x49, 0x46},
	'3': {0x21, 0x41, 0x45, 0x4B, 0x31},
	'4': {0x18, 0x14, 0x12, 0x7F, 0x10},
	'5': {0x27, 0x45, 0x45, 0x45, 0x39},
	'6': {0x3C, 0x4A, 0x49, 0x49, 0x30},
	'7': {0x01, 0x71, 0x09, 0x05, 0x03},
	'8': {0x36, 0x49, 0x49, 0x49, 0x36},
	'9': {0x06, 0x49, 0x49, 0x29, 0x1E},
	'A': {0x7E, 0x11, 0x11, 0x11, 0x7E},
	'C': {0x3E, 0x41, 0x41, 0x41, 0x22},
	'D': {0x7F, 0x41, 0x41, 0x22, 0x1C},
	'E': {0x7F, 0x49, 0x49, 0x49, 0x41},
	'I': {0x00, 0x41, 0x7F, 0x41, 0x00},
	'L': {0x7F, 0x40, 0x40, 0x40, 0x40},
	'M': {0x7F, 0x02, 0x0C, 0x02, 0x7F},
	'R': {0x7F, 0x09, 0x19, 0x29, 0x46},
	'T': {0x01, 0x01, 0x7F, 0x01, 0x01},
	'Y': {0x07, 0x08, 0x70, 0x08, 0x07},
}

// oledLayerNames は、レイヤーごとに画面に出す短い名前(5文字まで。幅32に収まる長さ)。
// 空("")なら、名前は出さない。初期キーマップ(keymap.go)に合わせた固定の名前。
// tinykeemap でキーマップを書き換えても、名前は変わらない(名前を編集できるようにするのは、将来)。
var oledLayerNames = [NumLayers]string{
	0: "MAC",
	1: "EDIT",
}

// initOLED は、起動時に1回だけ呼ばれる。ここでは、使い始める時刻を決めるだけで、OLED とは通信しない。
// (起動した直後は、USB の接続処理が忙しい時間帯なので、少し待ってから使い始める)
func initOLED() {
	oledStartAt = time.Now().Add(oledStartDelay)
}

// i2cSetup は、I2C1 を、GPIO14/15 で使える状態にする(設定し直すときにも使う)。
func i2cSetup() error {
	return machine.I2C1.Configure(machine.I2CConfig{
		SDA:       pinSDA,
		SCL:       pinSCL,
		Frequency: oledFreq,
	})
}

// oledStart は、OLED が見つかれば初期化して、最初の表示を出す。使い始める時刻になったときに、1回だけ呼ばれる。
func oledStart() {
	oledStarted = true

	if err := i2cSetup(); err != nil {
		return
	}

	// OLED がつながっているか確かめる(1バイト送って、返事(ACK)があるか)。
	// つながっていなければ、ここで終わる(oledReady は false のまま)。
	if err := machine.I2C1.Tx(oledAddress, []byte{0x00}, nil); err != nil {
		return
	}

	oledDev = ssd1306.NewI2C(machine.I2C1)
	oledDev.Configure(ssd1306.Config{
		Address: oledAddress,
		Width:   oledWidth,
		Height:  oledHeight,
	})
	oledReady = true
	oledDraw()
}

// tickOLED は、メインループから毎回呼ばれる。
// oledEveryLoops 回に1回だけ、画面を更新する。
func tickOLED() {
	if oledDiag {
		oledDiagPrint()
	}

	// まだ使い始める時刻になっていなければ、何もしない
	if !oledStarted {
		if time.Now().Before(oledStartAt) {
			return
		}
		oledStart()
	}
	if !oledReady {
		return
	}

	if oledDiag {
		now := time.Now()
		if !oledPrevTick.IsZero() {
			if gap := now.Sub(oledPrevTick); gap > oledMaxGap {
				oledMaxGap = gap
			}
		}
		oledPrevTick = now
	}

	oledTicks++
	if oledTicks < oledEveryLoops {
		return
	}
	oledTicks = 0

	oledDraw()
}

// oledDraw は、画面のメモリ(バッファ)を作り直して、OLED に送る。
// 送っている間(約12ms)は、メインループが止まる。
// 送るのに失敗したときは、I2C を設定し直す。続けて oledMaxFails 回失敗したら、OLED の更新をやめる。
func oledDraw() {
	l := currentLayer()

	oledDev.ClearBuffer()
	oledDrawCentered(oledLabelY, "LAYER", 1)
	oledDrawCentered(oledDigitY, string(rune('0'+l)), oledDigitScale) // NumLayers が10以上になったら、直すこと
	if name := oledLayerNames[l]; name != "" {
		oledDrawCentered(oledNameY, name, 1)
	}

	start := time.Now()
	err := oledDev.Display() // バッファ全体(512バイト)を OLED に送る
	if oledDiag {
		oledRecord(start, err)
	}

	if err == nil {
		oledFails = 0
		return
	}

	oledFails++
	if oledFails >= oledMaxFails {
		oledReady = false // 以後、OLED とは通信しない(キー入力を邪魔しないため)
		return
	}
	// 通信をやり直せる状態に戻す(次の更新で、もう一度送る)
	_ = i2cSetup()
	oledResets++
}

// oledPixel は、縦長の画面(幅32 x 高さ128。USB-C を上にして置いたとき)の点(lx, ly)を、
// 実際の画面(幅128 x 高さ32)の点に直して、点灯させる。
// 実際の画面は、USB-C を右にしたときに読める向き。そこから、時計回りに90°回して使う。
func oledPixel(lx, ly int16) {
	if oledFlip {
		oledDev.SetPixel(ly, oledHeight-1-lx, oledWhite)
		return
	}
	oledDev.SetPixel(oledWidth-1-ly, lx, oledWhite)
}

// oledDrawText は、バッファに文字を書く。scale 倍に拡大する。座標は縦長の画面のもの。
// 画面に出すには、あとで Display() を呼ぶ。
func oledDrawText(x, y int16, s string, scale int16) {
	for _, r := range s {
		g, ok := oledGlyphs[r]
		if !ok {
			g = oledGlyphs[' ']
		}
		for col := int16(0); col < 5; col++ {
			for row := int16(0); row < 7; row++ {
				if g[col]&(1<<uint(row)) == 0 {
					continue
				}
				// 1ドットを scale x scale の四角にして塗る
				for dx := int16(0); dx < scale; dx++ {
					for dy := int16(0); dy < scale; dy++ {
						oledPixel(x+col*scale+dx, y+row*scale+dy)
					}
				}
			}
		}
		x += 6 * scale // 5列 + 文字の間の1列
	}
}

// oledDrawCentered は、文字列を、縦長の画面の左右の中央にそろえて書く。
// 文字は ASCII だけ(1文字 = 1バイト)。
func oledDrawCentered(y int16, s string, scale int16) {
	w := int16(len(s))*6*scale - scale // 最後の文字の右の空きは数えない
	oledDrawText((oledLogicalW-w)/2, y, s, scale)
}
