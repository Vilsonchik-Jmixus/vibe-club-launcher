//go:build windows

package main

import (
	"errors"
	"runtime"
	"syscall"
	"unsafe"
)

// Modern Explorer-style "Select folder" dialog (IFileOpenDialog with FOS_PICKFOLDERS).
// Unlike the old tree dialog it has an address bar and a "Folder:" box, so a copied
// path can simply be pasted with Ctrl+V.

type guid struct {
	d1 uint32
	d2 uint16
	d3 uint16
	d4 [8]byte
}

var (
	clsidFileOpenDialog = guid{0xDC1C5A9C, 0xE88A, 0x4DDE, [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7}}
	iidIFileOpenDialog  = guid{0xD57C7288, 0xD4AD, 0x4768, [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60}}
	iidIShellItem       = guid{0x43826D1E, 0xE718, 0x42EE, [8]byte{0xBC, 0x55, 0xA1, 0xE2, 0x61, 0xC3, 0x7B, 0xFE}}

	ole32                       = syscall.NewLazyDLL("ole32.dll")
	procCoInitializeEx          = ole32.NewProc("CoInitializeEx")
	procCoUninitialize          = ole32.NewProc("CoUninitialize")
	procCoCreateInstance        = ole32.NewProc("CoCreateInstance")
	procCoTaskMemFree           = ole32.NewProc("CoTaskMemFree")
	shell32                     = syscall.NewLazyDLL("shell32.dll")
	procSHCreateItemFromParsing = shell32.NewProc("SHCreateItemFromParsingName")
	procGetForegroundWindow     = user32.NewProc("GetForegroundWindow")
)

const (
	fosPickFolders     = 0x20
	fosForceFileSystem = 0x40
	fosNoChangeDir     = 0x8
	sigdnFileSysPath   = 0x80058000
	hrCancelled        = 0x800704C7
)

// Vtable slots (IUnknown → IModalWindow → IFileDialog / IShellItem).
const (
	vtRelease         = 2
	vtShow            = 3
	vtSetOptions      = 9
	vtGetOptions      = 10
	vtSetFolder       = 12
	vtSetTitle        = 17
	vtSetOkLabel      = 18
	vtSetFileNameLbl  = 19
	vtGetResult       = 20
	vtItemDisplayName = 5
)

func comCall(obj uintptr, slot int, args ...uintptr) uintptr {
	vtbl := *(*uintptr)(unsafe.Pointer(obj))
	fn := *(*uintptr)(unsafe.Pointer(vtbl + uintptr(slot)*unsafe.Sizeof(uintptr(0))))
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{obj}, args...)...)
	return r
}

func comRelease(obj uintptr) {
	if obj != 0 {
		comCall(obj, vtRelease)
	}
}

var errDialogFailed = errors.New("folder dialog failed")

// pickFolderModern returns ("", nil) when the user cancels.
func pickFolderModern(initial string) (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hr, _, _ := procCoInitializeEx.Call(0, 0x2|0x4) // APARTMENTTHREADED | DISABLE_OLE1DDE
	if int32(hr) >= 0 {
		defer procCoUninitialize.Call()
	}

	var dlg uintptr
	hr, _, _ = procCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsidFileOpenDialog)), 0, 1, /*CLSCTX_INPROC_SERVER*/
		uintptr(unsafe.Pointer(&iidIFileOpenDialog)), uintptr(unsafe.Pointer(&dlg)))
	if int32(hr) < 0 || dlg == 0 {
		return "", errDialogFailed
	}
	defer comRelease(dlg)

	var opts uint32
	comCall(dlg, vtGetOptions, uintptr(unsafe.Pointer(&opts)))
	comCall(dlg, vtSetOptions, uintptr(opts|fosPickFolders|fosForceFileSystem|fosNoChangeDir))
	if t, err := syscall.UTF16PtrFromString("Выбери папку для сборки Vibe Club — путь можно вставить в адресную строку (Ctrl+V)"); err == nil {
		comCall(dlg, vtSetTitle, uintptr(unsafe.Pointer(t)))
	}
	if t, err := syscall.UTF16PtrFromString("Выбрать эту папку"); err == nil {
		comCall(dlg, vtSetOkLabel, uintptr(unsafe.Pointer(t)))
	}
	if t, err := syscall.UTF16PtrFromString("Папка или путь:"); err == nil {
		comCall(dlg, vtSetFileNameLbl, uintptr(unsafe.Pointer(t)))
	}
	if start := existingParent(initial); start != "" {
		if p, err := syscall.UTF16PtrFromString(start); err == nil {
			var item uintptr
			hr, _, _ = procSHCreateItemFromParsing.Call(uintptr(unsafe.Pointer(p)), 0, uintptr(unsafe.Pointer(&iidIShellItem)), uintptr(unsafe.Pointer(&item)))
			if int32(hr) >= 0 && item != 0 {
				comCall(dlg, vtSetFolder, item)
				comRelease(item)
			}
		}
	}

	owner, _, _ := procGetForegroundWindow.Call()
	hr = comCall(dlg, vtShow, owner)
	if uint32(hr) == hrCancelled {
		return "", nil
	}
	if int32(hr) < 0 {
		return "", errDialogFailed
	}
	var res uintptr
	hr = comCall(dlg, vtGetResult, uintptr(unsafe.Pointer(&res)))
	if int32(hr) < 0 || res == 0 {
		return "", errDialogFailed
	}
	defer comRelease(res)
	var name *uint16
	hr = comCall(res, vtItemDisplayName, sigdnFileSysPath, uintptr(unsafe.Pointer(&name)))
	if int32(hr) < 0 || name == nil {
		return "", errDialogFailed
	}
	defer procCoTaskMemFree.Call(uintptr(unsafe.Pointer(name)))
	return utf16PtrToString(name), nil
}

func utf16PtrToString(p *uint16) string {
	var s []uint16
	for ptr := unsafe.Pointer(p); ; ptr = unsafe.Add(ptr, 2) {
		c := *(*uint16)(ptr)
		if c == 0 {
			break
		}
		s = append(s, c)
	}
	return syscall.UTF16ToString(s)
}

var (
	folderIDDownloads        = guid{0x374DE290, 0x123F, 0x4565, [8]byte{0x91, 0x64, 0x39, 0xC4, 0x92, 0x5E, 0x46, 0x7B}}
	procSHGetKnownFolderPath = shell32.NewProc("SHGetKnownFolderPath")
)

// platformDownloadDir is the real "Downloads" folder, also when the player moved it to another drive.
func platformDownloadDir() string {
	var p *uint16
	r, _, _ := procSHGetKnownFolderPath.Call(uintptr(unsafe.Pointer(&folderIDDownloads)), 0, 0, uintptr(unsafe.Pointer(&p)))
	if r != 0 || p == nil {
		return ""
	}
	defer procCoTaskMemFree.Call(uintptr(unsafe.Pointer(p)))
	return utf16PtrToString(p)
}
