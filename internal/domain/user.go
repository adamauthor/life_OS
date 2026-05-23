package domain

import (
	"fmt"

	"github.com/google/uuid"
)

type UUID = uuid.UUID

func UserIDFromTelegram(telegramUserID int64) UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("telegram:%d", telegramUserID)))
}
