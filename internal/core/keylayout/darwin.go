//go:build darwin && cgo

package keylayout

/*
#cgo LDFLAGS: -framework Carbon -framework CoreFoundation
#include <Carbon/Carbon.h>

enum { codes = 128, mods = 4, width = 4 };

static TISInputSourceRef source(const char *id) {
	if (id == NULL) return TISCopyCurrentKeyboardLayoutInputSource();
	CFStringRef s = CFStringCreateWithCString(NULL, id, kCFStringEncodingUTF8);
	const void *keys[] = { kTISPropertyInputSourceID };
	const void *vals[] = { s };
	CFDictionaryRef filter = CFDictionaryCreate(NULL, keys, vals, 1, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	CFArrayRef list = TISCreateInputSourceList(filter, true);
	CFRelease(filter);
	CFRelease(s);
	TISInputSourceRef src = NULL;
	if (list != NULL && CFArrayGetCount(list) > 0) src = (TISInputSourceRef)CFRetain(CFArrayGetValueAtIndex(list, 0));
	if (list != NULL) CFRelease(list);
	return src;
}

// layoutKeys fills out with what every key code types under each modifier
// set, width UTF-16 units apiece, and lens with their lengths; 0 when the
// layout cannot be read.
static int layoutKeys(const char *id, UniChar *out, int *lens) {
	static const UInt32 set[mods] = { 0, shiftKey >> 8, optionKey >> 8, (optionKey | shiftKey) >> 8 };
	TISInputSourceRef src = source(id);
	if (src == NULL) return 0;
	CFDataRef data = TISGetInputSourceProperty(src, kTISPropertyUnicodeKeyLayoutData);
	if (data == NULL) {
		CFRelease(src);
		return 0;
	}
	const UCKeyboardLayout *layout = (const UCKeyboardLayout *)CFDataGetBytePtr(data);
	for (int c = 0; c < codes; c++) {
		for (int m = 0; m < mods; m++) {
			UInt32 dead = 0;
			UniCharCount n = 0;
			int i = c * mods + m;
			if (UCKeyTranslate(layout, c, kUCKeyActionDown, set[m], LMGetKbdType(), kUCKeyTranslateNoDeadKeysBit, &dead, width, &n, out + i * width) != noErr) n = 0;
			lens[i] = (int)n;
		}
	}
	CFRelease(src);
	return 1;
}
*/
import "C"

import (
	"unicode/utf16"
	"unsafe"
)

func current() []Key { return layout("") }

// layout reads the layout with the given input source id, the current one
// for "".
func layout(id string) []Key {
	var cid *C.char
	if id != "" {
		cid = C.CString(id)
		defer C.free(unsafe.Pointer(cid))
	}
	var out [C.codes * C.mods * C.width]C.UniChar
	var lens [C.codes * C.mods]C.int
	if C.layoutKeys(cid, &out[0], &lens[0]) == 0 {
		return nil
	}
	text := func(i int) string {
		u := make([]uint16, lens[i])
		for j := range u {
			u[j] = uint16(out[i*C.width+j])
		}
		return string(utf16.Decode(u))
	}
	var keys []Key
	for c := range C.codes {
		if !typing(c) {
			continue
		}
		i := c * C.mods
		keys = append(keys, Key{Plain: text(i), Shift: text(i + 1), Option: text(i + 2), OptionShift: text(i + 3)})
	}
	return keys
}

// typing is a key code of the main block, ISO and JIS keys included. The
// keypad types the same digits as the number row, and Option does not
// change them there, so taking it in would make every digit ambiguous.
func typing(code int) bool { return code < 0x41 || code == 0x5d || code == 0x5e }
