package cntriscsiadm

import (
	"fmt"
)

const (
	ChapNone              = "none"
	ChapFromPublishSecret = "publish-secret-mutual"
)

type IscsiPublishContext struct {
	Iqn       string
	Port      string
	Transport string
	Chap      string
}

func InjectPublishContext(dst map[string]string, src IscsiPublishContext) error {
	dst["iscsi.nqn"] = src.Iqn
	dst["iscsi.port"] = src.Port
	dst["iscsi.transport"] = src.Transport
	switch src.Chap {
	case ChapNone, ChapFromPublishSecret:
		dst["iscsi.chap"] = src.Chap
	default:
		return fmt.Errorf("chap: unknown value: %s", src.Chap)
	}
	return nil
}

func ParsePublishContext(src map[string]string) (IscsiPublishContext, error) {
	nqn := src["iscsi.nqn"]
	if nqn == "" {
		return IscsiPublishContext{}, fmt.Errorf("missing 'iscsi.nqn'")
	}
	port := src["iscsi.port"]
	if port == "" {
		return IscsiPublishContext{}, fmt.Errorf("missing 'iscsi.port'")
	}
	transport := src["iscsi.transport"]
	if transport == "" {
		return IscsiPublishContext{}, fmt.Errorf("missing 'iscsi.transport'")
	}
	chap := src["iscsi.chap"]
	switch chap {
	case "":
		return IscsiPublishContext{}, fmt.Errorf("missing 'iscsi.chap'")
	case ChapNone, ChapFromPublishSecret:
		// nothing to do here
	default:
		return IscsiPublishContext{}, fmt.Errorf("iscsi.chap: unknown value: '%s'", chap)
	}

	return IscsiPublishContext{
		Iqn:       nqn,
		Port:      port,
		Transport: transport,
		Chap:      chap,
	}, nil
}

type Chap struct {
	User string
	Pass string
}

func ParseChap(src map[string]string, prefix string) (Chap, error) {
	userKey := fmt.Sprintf("%s.user", prefix)
	nodeUser := src[userKey]
	if nodeUser == "" {
		return Chap{}, fmt.Errorf("field is missing: %s", userKey)
	}
	passKey := fmt.Sprintf("%s.pass", prefix)
	nodePass := src[passKey]
	if nodePass == "" {
		return Chap{}, fmt.Errorf("field is missing: %s", passKey)
	}
	return Chap{
		User: nodeUser,
		Pass: nodePass,
	}, nil
}

func (chap Chap) Inject(src map[string]string, prefix string) {
	src[fmt.Sprintf("%s.user", prefix)] = chap.User
	src[fmt.Sprintf("%s.pass", prefix)] = chap.Pass
}
