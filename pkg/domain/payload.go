package domain

import (
	"encoding/json"
)

type Payload []byte

func (p Payload) ToItems() ([]Item, error) {
	items := []Item{}

	if len(p) == 0 {
		return items, nil
	}

	err := json.Unmarshal(p, &items)
	if err != nil {
		return nil, err
	}

	return items, nil
}
