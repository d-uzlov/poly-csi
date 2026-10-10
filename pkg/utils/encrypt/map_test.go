package encrypt_test

import (
	"testing"

	"meoe.io/poly-csi/pkg/utils/encrypt"
)

type EncryptTestData struct {
	Qwe string
	Asd string
}

func TestFullCycle(t *testing.T) {
	private, err := encrypt.GenerateRsa()
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	cleartext := "123"

	wrap, err := encrypt.NewWrapper(private.GetPublicKey())
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	err = wrap.SetData([]byte(cleartext))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	decrypted, err := wrap.Extract(*private)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if cleartext != string(decrypted) {
		t.Errorf("cleartext %#v != decrypted %#v", cleartext, decrypted)
		t.FailNow()
	}
}
