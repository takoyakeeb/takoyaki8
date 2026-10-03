package main

// OLED(SSD1306、128x32、I2C)の表示。I2C1 を使う。ピンは config.go の pinSDA(GPIO14)/ pinSCL(GPIO15)。
//
// 今は「配線とコードの確認」用で、tinykeemap からは設定できない(コマンドは作っていない)。
// 確認用の表示: 上の行に「TAKOYAKI8」、その下で四角い印が、左から右へ流れ続ける。
// 印を動かしている間も、キー入力・キーの読み取り・USBシリアル・LED が、これまでどおり動くかを確かめる。
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
	oledWidth    = 128 // 画面の幅(ドット)
	oledHeight   = 32  // 画面の高さ(ドット)
	oledBarWidth = 16  // 流れる四角い印の幅
	oledBarTop   = 22  // 印の上端の y
	oledBarBot   = 30  // 印の下端の y(この手前まで塗る)
	oledBarStep  = 4   // 1回の更新で、印が進むドット数

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
	oledPos     int16           // 四角い印の左端の x
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
// 「TAKOYAKI8」に使う文字だけ入れてある。
var oledGlyphs = map[rune][5]byte{
	' ': {0x00, 0x00, 0x00, 0x00, 0x00},
	'8': {0x36, 0x49, 0x49, 0x49, 0x36},
	'A': {0x7E, 0x11, 0x11, 0x11, 0x7E},
	'I': {0x00, 0x41, 0x7F, 0x41, 0x00},
	'K': {0x7F, 0x08, 0x14, 0x22, 0x41},
	'O': {0x3E, 0x41, 0x41, 0x41, 0x3E},
	'T': {0x01, 0x01, 0x7F, 0x01, 0x01},
	'Y': {0x07, 0x08, 0x70, 0x08, 0x07},
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
// oledEveryLoops 回に1回だけ、四角い印を少し進めて、画面を更新する。
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

	// 印を右へ進める(右端に着いたら、左端へ戻る)
	oledPos += oledBarStep
	if oledPos+oledBarWidth > oledWidth {
		oledPos = 0
	}
	oledDraw()
}

// oledDraw は、画面のメモリ(バッファ)を作り直して、OLED に送る。
// 送っている間(約12ms)は、メインループが止まる。
// 送るのに失敗したときは、I2C を設定し直す。続けて oledMaxFails 回失敗したら、OLED の更新をやめる。
func oledDraw() {
	oledDev.ClearBuffer()
	oledDrawText(0, 0, "TAKOYAKI8", 2)

	for x := oledPos; x < oledPos+oledBarWidth; x++ {
		for y := int16(oledBarTop); y < oledBarBot; y++ {
			oledDev.SetPixel(x, y, oledWhite)
		}
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

// oledDrawText は、バッファに文字を書く。scale 倍に拡大する。
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
						oledDev.SetPixel(x+col*scale+dx, y+row*scale+dy, oledWhite)
					}
				}
			}
		}
		x += 6 * scale // 5列 + 文字の間の1列
	}
}
