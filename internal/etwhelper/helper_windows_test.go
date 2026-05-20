package etwhelper

import (
	"os"
	"testing"
)

func TestUSBProvidersUsePlainPowerEnableParameters(t *testing.T) {
	t.Setenv("USB_SUSPEND_WATCH_ETW_RUNDOWN", "")
	providers := providers()
	if len(providers) != 5 {
		t.Fatalf("providers length = %d, want 5", len(providers))
	}
	for i, provider := range providers {
		if provider.GUID.IsZero() {
			t.Fatalf("%s has a zero GUID", provider.Name)
		}
		if provider.EnableLevel != 0xff {
			t.Fatalf("%s EnableLevel = %#x, want 0xff", provider.Name, provider.EnableLevel)
		}
		wantKeyword := uint64(0x8)
		if i >= 3 {
			wantKeyword = 0xFFFFFFFF
		}
		if provider.MatchAnyKeyword != wantKeyword {
			t.Fatalf("%s MatchAnyKeyword = %#x, want Power", provider.Name, provider.MatchAnyKeyword)
		}
		if provider.MatchAllKeyword != 0 {
			t.Fatalf("%s MatchAllKeyword = %#x, want 0", provider.Name, provider.MatchAllKeyword)
		}
		if provider.EnableProperties != 0 {
			t.Fatalf("%s EnableProperties = %#x, want 0", provider.Name, provider.EnableProperties)
		}
	}
}

func TestUSBProviderKeywordsCanIncludeRundown(t *testing.T) {
	t.Setenv("USB_SUSPEND_WATCH_ETW_RUNDOWN", "1")
	if got := usbProviderKeywords(); got != 0x8008 {
		t.Fatalf("usbProviderKeywords() = %#x, want Power|Rundown", got)
	}
}

func TestParentWatchCanObserveCurrentProcess(t *testing.T) {
	watch, err := openParentWatch(os.Getpid())
	if err != nil {
		t.Fatalf("openParentWatch returned error: %v", err)
	}
	defer watch.Close()
	if watch.Exited() {
		t.Fatalf("current process should not be reported as exited")
	}
}
