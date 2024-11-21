package exporterstoretypes

import (
	"bytes"

	"github.com/benw10-1/brotato-exporter/errutil"
	"github.com/google/uuid"
	"github.com/tinylib/msgp/msgp"
)

// ExporterUser
type ExporterUser struct {
	UserID         uuid.UUID `json:"user_id"`
	MaxSubscribers int       `json:"max_subscribers"`
}

// UnmarshalMsg
func (eu *ExporterUser) UnmarshalMsg(bts []byte) error {
	r := bytes.NewReader(bts)

	msgpR := msgp.NewReader(r)

	userID, err := msgpR.ReadBytes(nil)
	if err != nil {
		return errutil.NewStackError(err)
	}
	eu.UserID, err = uuid.FromBytes(userID)
	if err != nil {
		return errutil.NewStackError(err)
	}

	eu.MaxSubscribers, err = msgpR.ReadInt()
	if err != nil {
		return errutil.NewStackError(err)
	}

	return nil
}

// MarshalMsg
func (eu *ExporterUser) MarshalMsg() ([]byte, error) {
	res := make([]byte, 0, 100)

	userIDBts, err := eu.UserID.MarshalBinary()
	if err != nil {
		return nil, errutil.NewStackError(err)
	}

	res = msgp.AppendBytes(res, userIDBts)
	res = msgp.AppendInt(res, eu.MaxSubscribers)

	return res, nil
}
