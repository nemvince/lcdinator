package main

import (
	"log"
	"sync/atomic"
	"time"

	"go.bug.st/serial"
)

const (
	KEY_HELP  = 0x41
	KEY_LEFT  = 0x42
	KEY_ESC   = 0x43
	KEY_UP    = 0x44
	KEY_ENTER = 0x45
	KEY_DOWN  = 0x46
	KEY_RIGHT = 0x47
)

type KeyHandler struct {
	RequestedScreen *int32
	RedrawChan      chan struct{}
	MenuIndex       *int32 // for menu navigation
	InMenu          *int32 // 0 = not in menu, 1 = in menu
	InDialog        *int32 // 0 = no dialog, 1 = dialog active
	DialogType      *int32 // 0 = none, 1 = shutdown, 2 = reboot
	DialogResult    *int32 // 0 = none, 1 = confirmed, 2 = cancelled
}

func (kh *KeyHandler) Start(port serial.Port) {
	go func() {
		buf := make([]byte, 1)
		for {
			port.SetReadTimeout(100 * time.Millisecond)
			n, _ := port.Read(buf)
			if n == 1 {
				log.Printf("Key pressed: 0x%02X", buf[0])
				changed := kh.handleKey(buf[0])
				if changed {
					select {
					case kh.RedrawChan <- struct{}{}:
					default:
					}
				}
			}
		}
	}()
}

func (kh *KeyHandler) handleKey(key byte) bool {
	changed := false
	curScreen := int(atomic.LoadInt32(kh.RequestedScreen))
	// About overlay logic
	if curScreen == 2 {
		if key == KEY_ESC {
			atomic.StoreInt32(kh.RequestedScreen, 0)
			changed = true
		}
		return changed
	}
	// Global screen cycling (skip About)
	switch key {
	case KEY_LEFT:
		if curScreen == 0 {
			curScreen = 4
		} else if curScreen == 2 {
			curScreen = 1
		} else {
			curScreen--
		}
		if curScreen == 2 {
			curScreen = 1
		}
		atomic.StoreInt32(kh.RequestedScreen, int32(curScreen))
		changed = true
		return changed
	case KEY_RIGHT:
		if curScreen == 4 {
			curScreen = 0
		} else if curScreen == 1 {
			curScreen = 3
		} else {
			curScreen++
		}
		if curScreen == 2 {
			curScreen = 3
		}
		atomic.StoreInt32(kh.RequestedScreen, int32(curScreen))
		changed = true
		return changed
	case KEY_HELP:
		atomic.StoreInt32(kh.RequestedScreen, 2)
		changed = true
		return changed
	case KEY_ESC:
		// Show menu as a real screen
		atomic.StoreInt32(kh.RequestedScreen, 3)
		atomic.StoreInt32(kh.MenuIndex, 0)
		changed = true
		return changed
	}
	// Delegate to current screen's HandleKey
	if curScreen >= 0 && curScreen < len(screens) {
		if screens[curScreen].HandleKey(key) {
			changed = true
		}
	}
	return changed
}
