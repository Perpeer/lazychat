//go:build darwin && cgo

package keylayout

/*
#cgo LDFLAGS: -framework ApplicationServices
#include <ApplicationServices/ApplicationServices.h>

static unsigned long long heldFlags(void) {
	return (unsigned long long)CGEventSourceFlagsState(kCGEventSourceStateCombinedSessionState);
}
*/
import "C"

// Held says whether Shift, Option or Control is down on this Mac's keyboard
// now. It reads only the modifier flags, which needs no permission, unlike
// reading keys; Caps Lock and Command are not asked.
func Held() (shift, option, control bool) {
	f := C.heldFlags()
	return f&C.kCGEventFlagMaskShift != 0, f&C.kCGEventFlagMaskAlternate != 0, f&C.kCGEventFlagMaskControl != 0
}
