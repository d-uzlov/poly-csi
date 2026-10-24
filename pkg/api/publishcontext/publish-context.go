package publishcontext

import (
	"crypto/hpke"
	"encoding/json"
	"fmt"
	"reflect"

	"meoe.io/poly-csi/pkg/api/blockspec"
	"meoe.io/poly-csi/pkg/api/fsspec"
	"meoe.io/poly-csi/pkg/api/volumecontext"
	"meoe.io/poly-csi/pkg/utils/secrets"
	"meoe.io/poly-csi/pkg/utils/serialmaps"
	"meoe.io/poly-csi/pkg/utils/typeregistry"
)

type PublishContext struct {
	BlockDevices map[string]Block `map:"block"`
	Filesystem   fsspec.Data      `map:"fs"`
}

type Block struct {
	ConnectorId     string         `map:"connectorId"`
	Data            blockspec.Data `map:"data"`
	ConnectorPreset string         `map:"preset"`
	CgroupPreset    string         `map:"cgroupPreset"`
}

type Serialized struct {
	Enc   []byte               `map:"enc"`
	Block []SerializedBlock    `map:"block"`
	Fs    SerializedFilesystem `map:"fs"`
}

type SerializedBlock struct {
	ConnectorId     string         `map:":connectorId"`
	Name            string         `map:":name"`
	Public          blockspec.Data `map:":"`
	Secret          []byte         `map:":secret"`
	ConnectorPreset string         `map:"preset"`
	CgroupPreset    string         `map:"cgroupPreset"`
}

type SerializedFilesystem struct {
	Public fsspec.Data `map:":"`
	Secret []byte      `map:":secret"`
}

var Registry = typeregistry.NewPrepopulated([]typeregistry.Sample{
	{Name: string(blockspec.Aoe), Sample: blockspec.AoeShare{}},
	{Name: string(blockspec.Iscsi), Sample: blockspec.IscsiShare{}},
	{Name: string(blockspec.Nbd), Sample: blockspec.NbdShare{}},
	{Name: string(blockspec.Nvme), Sample: blockspec.NvmeShare{}},
	{Name: string(fsspec.Nfs), Sample: fsspec.NfsShare{}},
	{Name: string(fsspec.Smb), Sample: fsspec.SmbShare{}},
	{Name: string(fsspec.Ext4), Sample: fsspec.Ext4Data{}},
	{Name: string(fsspec.Xfs), Sample: fsspec.XfsData{}},
})

func (pc *PublishContext) Serialize(out map[string]string, publicKey hpke.PublicKey) error {
	enc, sender, err := hpke.NewSender(publicKey, hpke.HKDFSHA256(), hpke.AES256GCM(), nil)
	if err != nil {
		return fmt.Errorf("hpke.NewSender: %w", err)
	}

	s := Serialized{
		Enc: enc,
	}

	for k, b := range pc.BlockDevices {
		public, secret, err := secrets.Split(b.Data)
		if err != nil {
			return fmt.Errorf("block '%s': split protocol info: %w", k, err)
		}

		plaintext, err := json.Marshal(secret)
		if err != nil {
			return err
		}
		tag := []byte("block")
		ciphertext, err := sender.Seal(tag, plaintext)
		if err != nil {
			return fmt.Errorf("block '%s': sender.Seal: %w", k, err)
		}

		s.Block = append(s.Block, SerializedBlock{
			ConnectorId:     b.ConnectorId,
			Name:            k,
			Public:          public,
			Secret:          ciphertext,
			ConnectorPreset: b.ConnectorPreset,
			CgroupPreset:    b.CgroupPreset,
		})
	}

	if pc.Filesystem != nil {
		public, secret, err := secrets.Split(pc.Filesystem)
		if err != nil {
			return fmt.Errorf("fs: split protocol info: %w", err)
		}

		plaintext, err := json.Marshal(secret)
		if err != nil {
			return fmt.Errorf("fs: marshal: %w", err)
		}
		tag := []byte("filesystem")
		ciphertext, err := sender.Seal(tag, plaintext)
		if err != nil {
			return fmt.Errorf("fs: sender.Seal: %w", err)
		}

		s.Fs = SerializedFilesystem{
			Public: public,
			Secret: ciphertext,
		}
	}

	_, err = serialmaps.SerializeInto(s, out, volumecontext.Prefix, serialmaps.WithRegistry(Registry))
	return err
}

func Parse(src map[string]string, privateKey hpke.PrivateKey) (PublishContext, error) {
	result := PublishContext{
		BlockDevices: map[string]Block{},
	}
	var s Serialized

	err := serialmaps.Parse(src, volumecontext.Prefix, &s, serialmaps.WithRegistry(Registry))
	if err != nil {
		return PublishContext{}, fmt.Errorf("serialmaps.Parse: %w", err)
	}

	receiver, err := hpke.NewRecipient(s.Enc, privateKey, hpke.HKDFSHA256(), hpke.AES256GCM(), nil)
	if err != nil {
		return PublishContext{}, fmt.Errorf("hpke.NewRecipient: %w", err)
	}

	for i, v := range s.Block {
		tag := []byte("block")
		plaintext, err := receiver.Open(tag, v.Secret)
		if err != nil {
			return PublishContext{}, fmt.Errorf("block i=%d: open secret: %w", i, err)
		}
		err = InterfaceUnmarshal(plaintext, &v.Public)
		if err != nil {
			return PublishContext{}, fmt.Errorf("block i=%d: unmarshal: %w", i, err)
		}
		result.BlockDevices[v.Name] = Block{
			ConnectorId:     v.ConnectorId,
			Data:            v.Public,
			ConnectorPreset: v.ConnectorPreset,
			CgroupPreset:    v.CgroupPreset,
		}
	}

	if s.Fs.Public != nil {
		tag := []byte("filesystem")
		plaintext, err := receiver.Open(tag, s.Fs.Secret)
		if err != nil {
			return PublishContext{}, fmt.Errorf("fs: open secret: %w", err)
		}
		err = InterfaceUnmarshal(plaintext, &s.Fs.Public)
		if err != nil {
			return PublishContext{}, fmt.Errorf("fs: unmarshal: %w", err)
		}
		result.Filesystem = s.Fs.Public
	}

	return result, nil
}

func InterfaceUnmarshal(source []byte, field any) error {
	val := reflect.ValueOf(field).Elem().Elem()
	if val.Kind() == reflect.Pointer {
		return json.Unmarshal(source, val.Interface())
	}

	ptr := reflect.New(val.Type())
	ptr.Elem().Set(val)
	err := json.Unmarshal(source, ptr.Interface())
	if err != nil {
		return fmt.Errorf("fs: unmarshal: %w", err)
	}
	reflect.ValueOf(field).Elem().Set(ptr.Elem())
	return nil
}
