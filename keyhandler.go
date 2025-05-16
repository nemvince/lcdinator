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
	if atomic.LoadInt32(kh.InDialog) == 1 {
		switch key {
		case KEY_ENTER:
			atomic.StoreInt32(kh.DialogResult, 1) // confirmed
			atomic.StoreInt32(kh.InDialog, 0)
			atomic.StoreInt32(kh.InMenu, 0)
			changed = true
		case KEY_ESC:
			atomic.StoreInt32(kh.DialogResult, 2) // cancelled
			atomic.StoreInt32(kh.InDialog, 0)
			changed = true
		}
		return changed
	}
	if atomic.LoadInt32(kh.InMenu) == 1 {
		switch key {
		case KEY_UP:
			if atomic.LoadInt32(kh.MenuIndex) > 0 {
				atomic.AddInt32(kh.MenuIndex, -1)
				changed = true
			}
		case KEY_DOWN:
			if atomic.LoadInt32(kh.MenuIndex) < 1 { // 2 menu items for now
				atomic.AddInt32(kh.MenuIndex, 1)
				changed = true
			}
		case KEY_ENTER:
			// Show confirmation dialog for selected menu item
			atomic.StoreInt32(kh.InDialog, 1)
			if atomic.LoadInt32(kh.MenuIndex) == 0 {
				atomic.StoreInt32(kh.DialogType, 1) // shutdown
			} else if atomic.LoadInt32(kh.MenuIndex) == 1 {
				atomic.StoreInt32(kh.DialogType, 2) // reboot
			}
			changed = true
		case KEY_ESC:
			atomic.StoreInt32(kh.InMenu, 0)
			changed = true
		}
		return changed
	}
	switch key {
	case KEY_ENTER: // ok
		if atomic.LoadInt32(kh.RequestedScreen) != 0 {
			atomic.StoreInt32(kh.RequestedScreen, 0)
			changed = true
		}
	case KEY_HELP:
		if atomic.LoadInt32(kh.RequestedScreen) != 1 {
			atomic.StoreInt32(kh.RequestedScreen, 1)
			changed = true
		}
	case KEY_LEFT:
		if atomic.LoadInt32(kh.RequestedScreen) > 0 {
			atomic.AddInt32(kh.RequestedScreen, -1)
			changed = true
		}
	case KEY_RIGHT:
		if atomic.LoadInt32(kh.RequestedScreen) < 2 {
			atomic.AddInt32(kh.RequestedScreen, 1)
			changed = true
		}
	case KEY_ESC:
		// Enter menu
		atomic.StoreInt32(kh.InMenu, 1)
		atomic.StoreInt32(kh.MenuIndex, 0)
		changed = true
	}
	return changed
}
