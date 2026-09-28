package main

import (
	"machine"
	"time"
)

// Matrix は COL2ROW のマトリクススキャナ。
type Matrix struct {
	cols     []machine.Pin
	rows     []machine.Pin
	stable   []bool  // 確定済みの状態(押されているか)
	count    []uint8 // 確定状態と違う読み取りが何回続いているか
	debounce uint8
}

func NewMatrix(cols, rows []machine.Pin, debounce uint8) *Matrix {
	m := &Matrix{
		cols:     cols,
		rows:     rows,
		stable:   make([]bool, len(cols)*len(rows)),
		count:    make([]uint8, len(cols)*len(rows)),
		debounce: debounce,
	}
	for _, c := range cols {
		c.Configure(machine.PinConfig{Mode: machine.PinOutput})
		c.Low()
	}
	for _, r := range rows {
		r.Configure(machine.PinConfig{Mode: machine.PinInputPulldown})
	}
	return m
}

func (m *Matrix) Scan(buf []KeyEvent) []KeyEvent {
	for c, col := range m.cols {
		col.High()
		time.Sleep(10 * time.Microsecond) // 電圧が落ち着くのを待つ

		for r, row := range m.rows {
			id := r*len(m.cols) + c
			pressed := row.Get()

			if pressed == m.stable[id] {
				m.count[id] = 0 // 変化なし(または一瞬のノイズ)
				continue
			}

			m.count[id]++
			if m.count[id] >= m.debounce {
				m.stable[id] = pressed
				m.count[id] = 0
				buf = append(buf, KeyEvent{ID: uint16(id), Pressed: pressed})
			}
		}

		col.Low()
	}
	return buf
}
