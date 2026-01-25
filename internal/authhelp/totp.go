package authhelp

import (
	"bytes"
	"encoding/base64"
	"image/png"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

func GenerateTOTP(accountName string) (*otp.Key, error) {
	return totp.Generate(totp.GenerateOpts{
		Issuer:      "RPSync",
		AccountName: accountName,
	})
}

func ValidateTOTP(passcode, secret string) bool {
	return totp.Validate(passcode, secret)
}

func GenerateQRCode(key *otp.Key) (string, error) {
	var buf bytes.Buffer
	img, err := key.Image(200, 200)
	if err != nil {
		return "", err
	}
	err = png.Encode(&buf, img)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
