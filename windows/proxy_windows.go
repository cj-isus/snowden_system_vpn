//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	// HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Internet Settings
	internetSettings = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

	hkeyCurrentUser = 0x80000001
	regTypeDWORD    = 4
	regTypeSZ       = 1

	keyProxyEnable = "ProxyEnable"
	keyProxyServer = "ProxyServer"
)

var (
	modadvapi32           = syscall.NewLazyDLL("advapi32.dll")
	procRegOpenKeyExW     = modadvapi32.NewProc("RegOpenKeyExW")
	procRegSetValueExW    = modadvapi32.NewProc("RegSetValueExW")
	procRegCloseKey       = modadvapi32.NewProc("RegCloseKey")
	procRegFlushKey       = modadvapi32.NewProc("RegFlushKey")

	modwininet  = syscall.NewLazyDLL("wininet.dll")
	procSetOption = modwininet.NewProc("InternetSetOptionW")
)

const (
	internetOptionSettingsChanged = 39
	internetOptionRefresh         = 37
)

// setSystemProxy enables the Windows system HTTP proxy at host (the mixed-in
// address of sing-box, e.g. "127.0.0.1:20808"). It writes ProxyEnable=1 and
// ProxyServer=host to HKCU and notifies WinINet so browsers pick it up live.
func setSystemProxy(host string) error {
	hkey, err := regOpenKey(hkeyCurrentUser, internetSettings)
	if err != nil {
		return fmt.Errorf("RegOpenKeyEx: %w", err)
	}
	defer regCloseKey(hkey)

	if err := regSetDWORD(hkey, keyProxyEnable, 1); err != nil {
		return err
	}
	if err := regSetSZ(hkey, keyProxyServer, host); err != nil {
		return err
	}
	notifySettingsChanged()
	return nil
}

// clearSystemProxy disables the system HTTP proxy and restores direct access.
// Called on VPN stop so the user's browser does not point at a dead proxy.
func clearSystemProxy() error {
	hkey, err := regOpenKey(hkeyCurrentUser, internetSettings)
	if err != nil {
		return fmt.Errorf("RegOpenKeyEx: %w", err)
	}
	defer regCloseKey(hkey)

	if err := regSetDWORD(hkey, keyProxyEnable, 0); err != nil {
		return err
	}
	notifySettingsChanged()
	return nil
}

// --- low-level helpers ---

func regOpenKey(root uintptr, path string) (syscall.Handle, error) {
	var hkey syscall.Handle
	ptr, _ := syscall.UTF16PtrFromString(path)
	r1, _, e := procRegOpenKeyExW.Call(
		uintptr(root),
		uintptr(unsafe.Pointer(ptr)),
		0,
		0xF003F, // KEY_ALL_ACCESS
		uintptr(unsafe.Pointer(&hkey)),
	)
	if r1 != 0 {
		return 0, fmt.Errorf("RegOpenKeyExW error %d: %v", r1, e)
	}
	return hkey, nil
}

func regSetDWORD(hkey syscall.Handle, name string, value uint32) error {
	nptr, _ := syscall.UTF16PtrFromString(name)
	r1, _, e := procRegSetValueExW.Call(
		uintptr(hkey),
		uintptr(unsafe.Pointer(nptr)),
		0,
		uintptr(regTypeDWORD),
		uintptr(unsafe.Pointer(&value)),
		unsafe.Sizeof(value),
	)
	if r1 != 0 {
		return fmt.Errorf("RegSetValueExW(%s) error %d: %v", name, r1, e)
	}
	return nil
}

func regSetSZ(hkey syscall.Handle, name, value string) error {
	nptr, _ := syscall.UTF16PtrFromString(name)
	vptr, _ := syscall.UTF16PtrFromString(value)
	// UTF16 includes a trailing NUL byte; size is (len+1)*2
	size := (len(value) + 1) * 2
	r1, _, e := procRegSetValueExW.Call(
		uintptr(hkey),
		uintptr(unsafe.Pointer(nptr)),
		0,
		uintptr(regTypeSZ),
		uintptr(unsafe.Pointer(vptr)),
		uintptr(size),
	)
	if r1 != 0 {
		return fmt.Errorf("RegSetValueExW(%s) error %d: %v", name, r1, e)
	}
	return nil
}

func regCloseKey(hkey syscall.Handle) {
	procRegCloseKey.Call(uintptr(hkey))
}

// notifySettingsChanged tells WinINet/WinHTTP that proxy settings changed, so
// already-running browsers pick up the new proxy without a restart.
func notifySettingsChanged() {
	procSetOption.Call(0, internetOptionSettingsChanged, 0, 0)
	procSetOption.Call(0, internetOptionRefresh, 0, 0)
}
