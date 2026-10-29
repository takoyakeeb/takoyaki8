# takoyaki8 配線表

配線を変えたら、このファイルと `config.go` のピン定義を、同じコミットで更新する。

## 概要

- マイコン: Waveshare RP2040-Zero
- スイッチ: 8キー(2行×4列のマトリクス)。ダイオードは各スイッチに1本で、COL2ROW
- LED: WS2812B-V5 を4個。OLED: 0.91 インチ、128×32、I2C

## ピン割り当て

シリーズ共通ルール: マトリクスは GPIO0 から順に使う。周辺機器のピンは固定。

| 役割 | GPIO |
|---|---|
| Row0 / Row1 | GPIO0 / GPIO1 |
| Col0 / Col1 / Col2 / Col3 | GPIO2 / GPIO3 / GPIO4 / GPIO5 |
| LED データ | GPIO26 |
| OLED SDA / SCL(I2C1) | GPIO14 / GPIO15 |
| 予備 | GPIO27 / GPIO28 |
| 使用不可 | GPIO29(基板から引き出されていない)、GPIO16(オンボード LED 専用) |

コード上のピン定義は `config.go` にある(LED = `pinLED`、OLED = `pinSDA` / `pinSCL`)。

## 配線表

形式: Col GPIO ---SW--- D(アノード→カソード) --- Row GPIO

| キー | 位置 | キー番号(ID) | 配線 |
|---|---|---|---|
| SW1 | R0C0 | 0 | GPIO2 ---SW1--- D1 --- GPIO0 |
| SW2 | R0C1 | 1 | GPIO3 ---SW2--- D2 --- GPIO0 |
| SW3 | R0C2 | 2 | GPIO4 ---SW3--- D3 --- GPIO0 |
| SW4 | R0C3 | 3 | GPIO5 ---SW4--- D4 --- GPIO0 |
| SW5 | R1C0 | 4 | GPIO2 ---SW5--- D5 --- GPIO1 |
| SW6 | R1C1 | 5 | GPIO3 ---SW6--- D6 --- GPIO1 |
| SW7 | R1C2 | 6 | GPIO4 ---SW7--- D7 --- GPIO1 |
| SW8 | R1C3 | 7 | GPIO5 ---SW8--- D8 --- GPIO1 |

キー番号は行優先:

```
Row0:  0  1  2  3
Row1:  4  5  6  7
```

共通線:

- Col0(GPIO2): SW1 と SW5 の片足
- Col1(GPIO3): SW2 と SW6
- Col2(GPIO4): SW3 と SW7
- Col3(GPIO5): SW4 と SW8
- Row0(GPIO0): D1〜D4 のカソード(黒線側)
- Row1(GPIO1): D5〜D8 のカソード(黒線側)

## 部品の向き

- ダイオードは、黒線側がカソード。COL2ROW なので、黒線側をすべて Row 側に向ける。
- スイッチの足は2本。どちらを Col につないでも構わない。

## 基板 rev1.0 の構成

回路図と基板の接続は、この配線表と `config.go` に一致している。

- マトリクス: 8キー(Choc v2、ホットスワップ)。D1〜D8(1N4148W、COL2ROW)。SW1〜SW8 = キー番号 0〜7
- RP2040-Zero: 基板の表面に、部品面を下にして直接ハンダ付け(USB-C は上辺側)
- LED: WS2812B-V5 を4個(LED1〜LED4)。GPIO26 → R1(0Ω、直列抵抗のパッド)→ LED1 の DIN。LED1〜LED4 は数珠つなぎで、LED4 の DOUT は未接続。電源は 5V(USB の VBUS)、データ信号は 3.3V のまま。LED ごとに、100nF のコンデンサ(C1〜C4)。ファームは `NumLEDs = 4`、明るさの上限は `ledLevel = 32`(0〜255)
- OLED: J1(1×4 のピンヘッダー)に差す。並びは GND / 3V3 / SCL(GPIO15)/ SDA(GPIO14)。I2C1、アドレス 0x3C、通信速度 400kHz。外付けのプルアップ抵抗なしで動く(基板にも、ない)
- OLED の向き: 縦置き(ピンが基板の下辺側)。USB-C を上にして置くと、読める
- 更新間隔(動作確認時の値): LED は約 20ms、OLED は約 100ms

注意:

- OLED の VCC は、3V3 につなぐ。5V にすると、モジュール側のプルアップで SDA / SCL が 5V まで上がり、5V に対応していない RP2040 の GPIO に入る。
- LED のデータ線は、R1(0Ω)で直結する。信号の 3.3V は、WS2812B の仕様(電源 5V のとき約 3.5V 以上が「1」)の下限をやや下回るので、ちらつくときは、R1 を 330Ω か 470Ω にする。470Ω を入れると光らない個体があった(ブレッドボードの白いテープ。黒いテープは、470Ω でも光った)。
