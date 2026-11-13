//go:build windows

package cripto

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// Entropia aggiuntiva: lega il dato protetto a Gaster Bless.
var dpapiEntropia = []byte("Gaster Bless-chiave-dati-v1")

func blob(b []byte) *windows.DataBlob {
	if len(b) == 0 {
		return &windows.DataBlob{}
	}
	return &windows.DataBlob{Size: uint32(len(b)), Data: &b[0]}
}

func copiaBlob(d *windows.DataBlob) []byte {
	out := make([]byte, d.Size)
	copy(out, unsafe.Slice(d.Data, d.Size))
	windows.LocalFree(windows.Handle(unsafe.Pointer(d.Data)))
	return out
}

func dpapiCifra(dato []byte) ([]byte, error) {
	var out windows.DataBlob
	err := windows.CryptProtectData(blob(dato), windows.StringToUTF16Ptr("Gaster Bless"), blob(dpapiEntropia), 0, nil,
		windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	if err != nil {
		return nil, err
	}
	return copiaBlob(&out), nil
}

func dpapiDecifra(dato []byte) ([]byte, error) {
	var out windows.DataBlob
	err := windows.CryptUnprotectData(blob(dato), nil, blob(dpapiEntropia), 0, nil,
		windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	if err != nil {
		return nil, err
	}
	return copiaBlob(&out), nil
}
