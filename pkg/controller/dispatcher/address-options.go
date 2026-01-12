package dispatcher

import "meoe.io/poly-csi/pkg/node/addropt"

func InjectAddressOptions(src []AddressOption, dst map[string]string) error {
	var opts []addropt.AddressOption
	for _, v := range src {
		opts = append(opts, addropt.AddressOption{
			Name:    v.Name,
			Address: v.Address,
		})
	}
	return addropt.Inject(opts, dst)
}
