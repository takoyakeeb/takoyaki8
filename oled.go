package main

// OLED(SSD1306、128x32、I2C)の表示。I2C1 を使う。ピンは model_takoyaki8.go の pinSDA(GPIO14)/ pinSCL(GPIO15)。
//
// 表示するもの: いま「いちばん上」の有効なレイヤーの番号(大きく)と、短い名前(layer.go の currentLayer)。
// 向きは、設定 rotate(0 / 90 / 180 / 270)で決める。ボードを、USB-C が上の状態から、時計回りに何度回して置くか。
//   0 = USB-C が上(縦長。初期値。いままでの向き)、90 = USB-C が右(横長。画面そのままの向き)、
//   180 = USB-C が下(縦長)、270 = USB-C が左(横長)。
// 描くときは、見える向きの画面(縦長なら幅32 x 高さ128、横長なら幅128 x 高さ32)として座標を考え、
// oledPixel で実際の画面の座標に直す。
// レイヤーの名前は、設定の lname0〜(cfg.go の layerName)から読む。CFG SET で変えると、すぐ画面に出る。
// 明るさは、設定の oled(0〜100%)。0 は画面を消す(oledApplyLevel)。
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
	// 画面の大きさ(oledWidth、oledHeight)は、model_takoyaki8.go。

	// 表示の位置と大きさ(縦長の画面の座標。幅32 x 高さ128。文字は、左右の中央にそろえる)
	oledLabelY     = 8  // 「LAYER」の上端の y
	oledDigitY     = 34 // レイヤーの番号の上端の y
	oledDigitScale = 5  // レイヤーの番号の拡大(5x7 の文字を5倍 = 幅25 x 高さ35)
	oledNameY      = 88 // 短い名前の上端の y

	// 横長の画面(幅128 x 高さ32)の表示。左に番号(大きく)、右に「LAYER」と名前。
	oledLandDigitX     = 6  // 番号の左端の x
	oledLandDigitY     = 2  // 番号の上端の y(5x7 の文字を4倍 = 幅20 x 高さ28)
	oledLandDigitScale = 4  // 番号の拡大
	oledLandTextX      = 32 // 右側の文字を置く場所の左端の x(幅は 128 - 32 = 96)
	oledLandTextW      = 96 // 右側の文字を置く場所の幅
	oledLandLabelY     = 3  // 「LAYER」の上端の y(名前があるとき)
	oledLandNameY      = 11 // 名前の上端の y(3倍 = 高さ21。5文字で幅89)
	oledLandNameScale  = 3  // 名前の拡大

	// SSD1306 の「コントラスト」の命令(次に、0〜255 の値を1つ送る)。明るさは、これで変える。
	oledCmdContrast = 0x81

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

	oledLevelSent = -1 // 最後に OLED へ送った明るさ(%)。-1 は、まだ送っていない
	oledRotate    = 0  // いま使っている向き(設定 rotate。画面を描くたびに読み直す)
)

var oledWhite = color.RGBA{R: 255, G: 255, B: 255, A: 255} // 点灯させる色

// ===== 診断用(OLED が止まったときの原因調べのために残す。通常は false) =====
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
// レイヤー名に使える文字(英大文字・数字・-・_。cfg.go の cfgName)と、スペースが、すべて入っている。
// 表にない文字は、スペースとして描く(oledDrawText)。
var oledGlyphs = map[rune][5]byte{
	' ': {0x00, 0x00, 0x00, 0x00, 0x00},
	'-': {0x08, 0x08, 0x08, 0x08, 0x08},
	'_': {0x40, 0x40, 0x40, 0x40, 0x40},
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
	'B': {0x7F, 0x49, 0x49, 0x49, 0x36},
	'C': {0x3E, 0x41, 0x41, 0x41, 0x22},
	'D': {0x7F, 0x41, 0x41, 0x22, 0x1C},
	'E': {0x7F, 0x49, 0x49, 0x49, 0x41},
	'F': {0x7F, 0x09, 0x09, 0x09, 0x01},
	'G': {0x3E, 0x41, 0x49, 0x49, 0x7A},
	'H': {0x7F, 0x08, 0x08, 0x08, 0x7F},
	'I': {0x00, 0x41, 0x7F, 0x41, 0x00},
	'J': {0x20, 0x40, 0x41, 0x3F, 0x01},
	'K': {0x7F, 0x08, 0x14, 0x22, 0x41},
	'L': {0x7F, 0x40, 0x40, 0x40, 0x40},
	'M': {0x7F, 0x02, 0x0C, 0x02, 0x7F},
	'N': {0x7F, 0x04, 0x08, 0x10, 0x7F},
	'O': {0x3E, 0x41, 0x41, 0x41, 0x3E},
	'P': {0x7F, 0x09, 0x09, 0x09, 0x06},
	'Q': {0x3E, 0x41, 0x51, 0x21, 0x5E},
	'R': {0x7F, 0x09, 0x19, 0x29, 0x46},
	'S': {0x46, 0x49, 0x49, 0x49, 0x31},
	'T': {0x01, 0x01, 0x7F, 0x01, 0x01},
	'U': {0x3F, 0x40, 0x40, 0x40, 0x3F},
	'V': {0x1F, 0x20, 0x40, 0x20, 0x1F},
	'W': {0x3F, 0x40, 0x38, 0x40, 0x3F},
	'X': {0x63, 0x14, 0x08, 0x14, 0x63},
	'Y': {0x07, 0x08, 0x70, 0x08, 0x07},
	'Z': {0x61, 0x51, 0x49, 0x45, 0x43},
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
	// 明るさを合わせる。0%(画面を消している)なら、画面は送らない(送る間のループの停止も省ける)
	if !oledApplyLevel() {
		return
	}

	l := currentLayer()
	oledRotate = cfgInt("rotate") // 向きを変えても、次の更新で、画面全体が描き直される

	oledDev.ClearBuffer()
	if oledRotate == 90 || oledRotate == 270 {
		oledDrawLandscape(l)
	} else {
		oledDrawPortrait(l)
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

// oledApplyLevel は、設定 oled(0〜100%)が前回送った値から変わっていたら、OLED に明るさを送る。
// 0% は画面を消す(コントラスト 0 でも、SSD1306 はうっすら光るため)。1〜100% は、コントラストを 0〜255 に比例させる。
// 画面を表示してよい(1%以上)なら true を返す。
func oledApplyLevel() bool {
	p := cfgInt("oled")
	if p != oledLevelSent {
		if p == 0 {
			_ = oledDev.Sleep(true) // 画面を消す
		} else {
			if oledLevelSent <= 0 {
				_ = oledDev.Sleep(false) // 消していた画面(または最初)を、つける
			}
			oledDev.Command(oledCmdContrast)
			oledDev.Command(uint8(p * 255 / 100))
		}
		oledLevelSent = p
	}
	return p > 0
}

// oledPixel は、見える向きの画面の点(lx, ly)を、実際の画面(幅128 x 高さ32)の点に直して、点灯させる。
// 実際の画面は、USB-C を右にしたときに読める向き(rotate = 90 と同じ)。
func oledPixel(lx, ly int16) {
	switch oledRotate {
	case 90: // 横長。画面そのまま
		oledDev.SetPixel(lx, ly, oledWhite)
	case 270: // 横長。180°回す
		oledDev.SetPixel(oledWidth-1-lx, oledHeight-1-ly, oledWhite)
	case 180: // 縦長(幅32 x 高さ128)。USB-C が下
		oledDev.SetPixel(ly, oledHeight-1-lx, oledWhite)
	default: // 0: 縦長(幅32 x 高さ128)。USB-C が上
		oledDev.SetPixel(oledWidth-1-ly, lx, oledWhite)
	}
}

// oledDrawPortrait は、縦長の画面(rotate = 0 / 180)に、「LAYER」・番号・名前を描く。
func oledDrawPortrait(l int) {
	oledDrawCentered(oledLabelY, "LAYER", 1)
	oledDrawCentered(oledDigitY, string(rune('0'+l)), oledDigitScale) // NumLayers が10以上になったら、直すこと
	if name := layerName(l); name != "" {
		oledDrawCentered(oledNameY, name, 1)
	}
}

// oledDrawLandscape は、横長の画面(rotate = 90 / 270)に描く。左に番号、右に「LAYER」と名前。
// 名前が空のときは、「LAYER」だけを、2倍の大きさで、上下の中央に置く。
func oledDrawLandscape(l int) {
	oledDrawText(oledLandDigitX, oledLandDigitY, string(rune('0'+l)), oledLandDigitScale)
	if name := layerName(l); name != "" {
		oledDrawCenteredIn(oledLandTextX, oledLandTextW, oledLandLabelY, "LAYER", 1)
		oledDrawCenteredIn(oledLandTextX, oledLandTextW, oledLandNameY, name, oledLandNameScale)
	} else {
		oledDrawCenteredIn(oledLandTextX, oledLandTextW, (oledHeight-14)/2, "LAYER", 2)
	}
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

// oledDrawCenteredIn は、文字列を、x0 から幅 areaW の場所の中央にそろえて書く。
// 文字は ASCII だけ(1文字 = 1バイト)。
func oledDrawCenteredIn(x0, areaW, y int16, s string, scale int16) {
	w := int16(len(s))*6*scale - scale // 最後の文字の右の空きは数えない
	oledDrawText(x0+(areaW-w)/2, y, s, scale)
}

// oledDrawCentered は、文字列を、縦長の画面(幅32)の左右の中央にそろえて書く。
func oledDrawCentered(y int16, s string, scale int16) {
	oledDrawCenteredIn(0, oledHeight, y, s, scale)
}
