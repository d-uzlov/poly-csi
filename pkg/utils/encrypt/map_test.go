package encrypt_test

import (
	"encoding/json"
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

	storage := map[string]string{}

	secretData := EncryptTestData{
		Qwe: "123",
		Asd: "456",
	}

	err = encrypt.InjectSecretIntoMap(storage, private.GetPublicKey(), secretData)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	decryptedBytes, err := encrypt.ExtractSecretFromMap(storage, *private)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	var decrypted EncryptTestData
	err = json.Unmarshal(decryptedBytes, &decrypted)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if secretData != decrypted {
		t.Errorf("secretData %#v != decrypted %#v", secretData, decrypted)
		t.FailNow()
	}
}
