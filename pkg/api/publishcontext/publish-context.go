package publishcontext

import (
	"crypto/hpke"
	"encoding/json"
	"fmt"

	"meoe.io/poly-csi/pkg/api/blockapi"
	"meoe.io/poly-csi/pkg/api/fsapi"
	"meoe.io/poly-csi/pkg/api/volumecontext"
	"meoe.io/poly-csi/pkg/utils/secrets"
	"meoe.io/poly-csi/pkg/utils/serialmaps"
	"meoe.io/poly-csi/pkg/utils/typeregistry"
)

type PublishContext struct {
	BlockDevices map[string]Block `map:"block"`
	Filesystem   fsapi.Data       `map:"fs"`
}

type Block struct {
	ConnectorId string        `map:"connectorId"`
	Data        blockapi.Data `map:"data"`
}

// type AddressOption struct {
// 	Name    string `map:"n" json:"n" validate:"required"`
// 	Address string `map:"a" json:"a" validate:"required"`
// }

type Serialized struct {
	Enc   []byte               `map:"enc"`
	Block []SerializedBlock    `map:"block"`
	Fs    SerializedFilesystem `map:"fs"`
}

type SerializedBlock struct {
	ConnectorId string        `map:"connectorId"`
	Name        string        `map:"name"`
	Public      blockapi.Data `map:"public"`
	Secret      []byte        `map:"secret"`
}

type SerializedFilesystem struct {
	Public fsapi.Data `map:"public"`
	Secret []byte     `map:"secret"`
}

var Registry = typeregistry.NewPrepopulated([]typeregistry.Sample{
	{Name: string(blockapi.Aoe), Sample: blockapi.AoeShare{}},
	{Name: string(blockapi.Iscsi), Sample: blockapi.IscsiShare{}},
	{Name: string(blockapi.Nbd), Sample: blockapi.NbdShare{}},
	{Name: string(blockapi.Nvme), Sample: blockapi.NvmeShare{}},
	{Name: string(fsapi.Nfs), Sample: fsapi.NfsShare{}},
	{Name: string(fsapi.Smb), Sample: fsapi.SmbShare{}},
	{Name: string(fsapi.Ext4), Sample: fsapi.Ext4Data{}},
	{Name: string(fsapi.Xfs), Sample: fsapi.XfsData{}},
})

func (pc *PublishContext) Serialize(out map[string]string, publicKey hpke.PublicKey) error {
	enc, sender, err := hpke.NewSender(publicKey, hpke.HKDFSHA256(), hpke.AES256GCM(), nil)
	if err != nil {
		return err
	}

	s := Serialized{
		Enc: enc,
	}

	for k, b := range pc.BlockDevices {
		public, secret, err := secrets.Split(b.Data)
		if err != nil {
			return fmt.Errorf("split protocol info: %w", err)
		}

		plaintext, err := json.Marshal(secret)
		if err != nil {
			return err
		}
		tag := []byte("block")
		ciphertext, err := sender.Seal(tag, plaintext)
		if err != nil {
			return err
		}

		s.Block = append(s.Block, SerializedBlock{
			ConnectorId: b.ConnectorId,
			Name:        k,
			Public:      public,
			Secret:      ciphertext,
		})
	}

	if pc.Filesystem != nil {
		public, secret, err := secrets.Split(pc.Filesystem)
		if err != nil {
			return fmt.Errorf("split protocol info: %w", err)
		}

		plaintext, err := json.Marshal(secret)
		if err != nil {
			return err
		}
		tag := []byte("filesystem")
		ciphertext, err := sender.Seal(tag, plaintext)
		if err != nil {
			return err
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
	result := PublishContext{}
	var s Serialized

	err := serialmaps.Parse(src, volumecontext.Prefix, &s, serialmaps.WithRegistry(Registry))
	if err != nil {
		return PublishContext{}, err
	}

	receiver, err := hpke.NewRecipient(s.Enc, privateKey, hpke.HKDFSHA256(), hpke.AES256GCM(), nil)
	if err != nil {
		return PublishContext{}, err
	}

	for _, v := range s.Block {
		tag := []byte("block")
		plaintext, err := receiver.Open(tag, v.Secret)
		if err != nil {
			return PublishContext{}, err
		}
		err = json.Unmarshal(plaintext, &v.Public)
		if err != nil {
			return PublishContext{}, err
		}
		result.BlockDevices[v.Name] = Block{
			ConnectorId: v.ConnectorId,
			Data:        v.Public,
		}
	}

	if s.Fs.Public != nil {
		tag := []byte("filesystem")
		plaintext, err := receiver.Open(tag, s.Fs.Secret)
		if err != nil {
			return PublishContext{}, err
		}
		err = json.Unmarshal(plaintext, &s.Fs.Public)
		if err != nil {
			return PublishContext{}, err
		}
		result.Filesystem = s.Fs.Public
	}

	return result, nil
}
