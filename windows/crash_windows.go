//go:build windows

package main

import (
	"log"
	"syscall"
	"unsafe"
)

var (
	kernel32       = syscall.NewLazyDLL("kernel32.dll")
	procSetHandler = kernel32.NewProc("SetConsoleCtrlHandler")

	// Reuse advapi32 procs from proxy_windows.go:
	// procRegOpenKeyExW, procRegSetValueExW, procRegCloseKey

	procRegQueryValueExW = modadvapi32.NewProc("RegQueryValueExW")
)

// HKEY_CURRENT_USER = 0x80000001 — already defined in proxy_windows.go

// installCrashHandler registers a Windows console control handler that clears
// the system proxy on Ctrl+C / taskkill (without /F) / logoff / shutdown.
// It CANNOT catch taskkill /F or a hard crash — pair with ClearStaleProxyOnStartup.
func installCrashHandler() {
	cb := syscall.NewCallback(func(ctrlType uint32) uintptr {
		// Best-effort: clear the proxy so the user doesn't lose internet.
		_ = clearSystemProxy()
		return 0 // FALSE = let other handlers process too
	})
	_, _, _ = procSetHandler.Call(cb, 1)
	log.Printf("[crash] console control handler installed")
}

// ClearStaleProxyOnStartup checks if the system proxy points to our local port
// (127.0.0.1:20808) and clears it if so. This handles taskkill /F crashes.
func ClearStaleProxyOnStartup() {
	enabled, _ := regGetDWORD(hkeyCurrentUser, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, "ProxyEnable")
	if enabled == 0 {
		return // proxy already off
	}

	server, _ := regGetSZ(hkeyCurrentUser, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, "ProxyServer")

	// If the proxy points to our port, it's stale from a previous crash.
	if server == "127.0.0.1:20808" {
		log.Printf("[startup] detected stale proxy %s — clearing", server)
		_ = clearSystemProxy()
	}
}

// --- low-level registry read helpers ---

func regGetDWORD(root uintptr, path, name string) (uint32, error) {
	var hkey syscall.Handle
	ppath, _ := syscall.UTF16PtrFromString(path)
	r1, _, _ := procRegOpenKeyExW.Call(
		uintptr(root),
		uintptr(unsafe.Pointer(ppath)),
		0,
		0x20019, // KEY_READ
		uintptr(unsafe.Pointer(&hkey)),
	)
	if r1 != 0 {
		return 0, nil
	}
	defer procRegCloseKey.Call(uintptr(hkey))

	pname, _ := syscall.UTF16PtrFromString(name)
	var val uint32
	var bufLen uint32 = 4
	var valType uint32
	r1, _, _ = procRegQueryValueExW.Call(
		uintptr(hkey),
		uintptr(unsafe.Pointer(pname)),
		0,
		uintptr(unsafe.Pointer(&valType)),
		uintptr(unsafe.Pointer(&val)),
		uintptr(unsafe.Pointer(&bufLen)),
	)
	if r1 != 0 {
		return 0, nil
	}
	return val, nil
}

func regGetSZ(root uintptr, path, name string) (string, error) {
	var hkey syscall.Handle
	ppath, _ := syscall.UTF16PtrFromString(path)
	r1, _, _ := procRegOpenKeyExW.Call(
		uintptr(root),
		uintptr(unsafe.Pointer(ppath)),
		0,
		0x20019, // KEY_READ
		uintptr(unsafe.Pointer(&hkey)),
	)
	if r1 != 0 {
		return "", nil
	}
	defer procRegCloseKey.Call(uintptr(hkey))

	pname, _ := syscall.UTF16PtrFromString(name)
	var buf [1024]uint16
	var bufLen uint32 = uint32(len(buf) * 2)
	var valType uint32
	r1, _, _ = procRegQueryValueExW.Call(
		uintptr(hkey),
		uintptr(unsafe.Pointer(pname)),
		0,
		uintptr(unsafe.Pointer(&valType)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&bufLen)),
	)
	if r1 != 0 {
		return "", nil
	}
	// Convert UTF-16 to Go string
	n := bufLen / 2
	for i := uint32(0); i < n; i++ {
		if buf[i] == 0 {
			n = i
			break
		}
	}
	return syscall.UTF16ToString(buf[:n]), nil
}
