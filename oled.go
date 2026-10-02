package main

// OLED(SSD1306、128x32、I2C)の表示。I2C1 を使う。ピンは config.go の pinSDA(GPIO14)/ pinSCL(GPIO15)。
//
// 今は「配線とコードの確認」用で、tinykeemap からは設定できない(コマンドは作っていない)。
// 確認用の表示: 上の行に「TAKOYAKI8」、その下で四角い印が、左から右へ流れ続ける。
// 印を動かしている間も、キー入力・キーの読み取り・USBシリアル・LED が、これまでどおり動くかを確かめる。
//
// OLED がつながっていないときは、何もしない(キーボードとしては、そのまま動く)。

import (
	"image/color" // 色の型(画面の点を「点灯」にするために使う)
	"machine"

	"tinygo.org/x/drivers/ssd1306" // SSD1306 用のドライバ
)

const (
	oledWidth    = 128 // 画面の幅(ドット)
	oledHeight   = 32  // 画面の高さ(ドット)
	oledBarWidth = 16  // 流れる四角い印の幅
	oledBarTop   = 22  // 印の上端の y
	oledBarBot   = 30  // 印の下端の y(この手前まで塗る)
	oledBarStep  = 4   // 1回の更新で、印が進むドット数
)

var (
	oledDev   *ssd1306.Device // ドライバ(これを通して OLED に表示を送る)
	oledReady bool            // OLED が見つかって、使える状態なら true
	oledTicks int             // メインループが何回まわったか(oledEveryLoops に達したら更新して0に戻す)
	oledPos   int16           // 四角い印の左端の x
)

var oledWhite = color.RGBA{R: 255, G: 255, B: 255, A: 255} // 点灯させる色

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

// initOLED は、I2C1 を使える状態にして、OLED が見つかれば初期化して、最初の表示を出す。
func initOLED() {
	err := machine.I2C1.Configure(machine.I2CConfig{
		SDA:       pinSDA,
		SCL:       pinSCL,
		Frequency: oledFreq,
	})
	if err != nil {
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
	if !oledReady {
		return
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
func oledDraw() {
	oledDev.ClearBuffer()
	oledDrawText(0, 0, "TAKOYAKI8", 2)

	for x := oledPos; x < oledPos+oledBarWidth; x++ {
		for y := int16(oledBarTop); y < oledBarBot; y++ {
			oledDev.SetPixel(x, y, oledWhite)
		}
	}

	oledDev.Display() // バッファ全体(512バイト)を OLED に送る
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
