package util

import (
	"github.com/skip2/go-qrcode"
)

func GenerateQRCode(data string) string {
	qr, err := qrcode.New(data, qrcode.Medium)
	if err != nil {
		return ""
	}
	return qr.ToSmallString(false)
}
