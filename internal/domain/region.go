package domain

import "fmt"

// Region — страна магазина Steam (параметр cc), см. глоссарий.
type Region string

// Поддерживаемые в R1 регионы (REQ-FUNC-005).
const (
	RegionBY Region = "BY"
	RegionRU Region = "RU"
	RegionKZ Region = "KZ"
	RegionUS Region = "US"
	RegionPL Region = "PL"
)

// RegionInfo описывает регион для клиента: название, валюта, поддержка ITAD.
type RegionInfo struct {
	Code          Region `json:"code"`
	Name          string `json:"name"`
	Currency      string `json:"currency"`
	ITADSupported bool   `json:"itad_supported"`
}

// Regions — справочник регионов R1 в порядке показа пользователю.
// Steam продаёт игры в BY и KZ в долларах США (Приложение В SRS).
var Regions = []RegionInfo{
	{Code: RegionBY, Name: "Беларусь", Currency: "USD"},
	{Code: RegionRU, Name: "Россия", Currency: "RUB"},
	{Code: RegionKZ, Name: "Казахстан", Currency: "USD"},
	{Code: RegionUS, Name: "США", Currency: "USD", ITADSupported: true},
	{Code: RegionPL, Name: "Польша", Currency: "PLN", ITADSupported: true},
}

// ParseRegion проверяет код региона.
func ParseRegion(s string) (Region, error) {
	for _, r := range Regions {
		if string(r.Code) == s {
			return r.Code, nil
		}
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownRegion, s)
}

// ITADSupported сообщает, хранится ли для региона история из ITAD (SS-23).
func (r Region) ITADSupported() bool {
	for _, info := range Regions {
		if info.Code == r {
			return info.ITADSupported
		}
	}
	return false
}
