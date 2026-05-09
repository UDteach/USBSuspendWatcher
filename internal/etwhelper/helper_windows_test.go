package etwhelper

import "testing"

func TestUSBProvidersUsePlainPowerEnableParameters(t *testing.T) {
	t.Setenv("USB_SUSPEND_WATCH_ETW_RUNDOWN", "")
	providers := providers()
	if len(providers) != 3 {
		t.Fatalf("providers length = %d, want 3", len(providers))
	}
	for _, provider := range providers {
		if provider.GUID.IsZero() {
			t.Fatalf("%s has a zero GUID", provider.Name)
		}
		if provider.EnableLevel != 0xff {
			t.Fatalf("%s EnableLevel = %#x, want 0xff", provider.Name, provider.EnableLevel)
		}
		if provider.MatchAnyKeyword != 0x8 {
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
