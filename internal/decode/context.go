package decode

import (
	"encoding/json"

	"github.com/kilimcininkoroglu/google-finance-api/internal/models"
)

// Classification decodes the uwlMvd payload into descriptive labels for a
// ticker (e.g. "Most active", "Stock", "US listed security"). The structure is
// data[0][0] = [machineID, [labels]], where each label is [name, description, ...].
func Classification(raw json.RawMessage) ([]models.ClassificationLabel, error) {
	arr, err := unmarshalNested(raw)
	if err != nil {
		return nil, err
	}

	root := drillDown(arr, 0, 0)
	if root == nil {
		return nil, nil
	}

	labels := atSlice(root, 1)
	if labels == nil {
		return nil, nil
	}

	var out []models.ClassificationLabel
	for _, item := range labels {
		a, ok := item.([]any)
		if !ok || len(a) < 1 {
			continue
		}
		name := atString(a, 0)
		if name == "" {
			continue
		}
		out = append(out, models.ClassificationLabel{
			Label:       name,
			Description: atString(a, 1),
		})
	}

	return out, nil
}

// StockContext decodes the mKsvE payload into the cross-exchange listings of a
// symbol. Each item carries a standard quote root at index 3, matched by the
// recursive findQuotableItems traversal.
func StockContext(raw json.RawMessage) ([]models.CrossListing, error) {
	arr, err := unmarshalNested(raw)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var listings []models.CrossListing
	findQuotableItems(arr, func(a []any) {
		tickerArr := atSlice(a, 1)
		if len(tickerArr) < 2 {
			return
		}
		symbol := atString(tickerArr, 0)
		exchange := atString(tickerArr, 1)
		key := symbol + ":" + exchange
		if seen[key] {
			return
		}
		seen[key] = true

		cl := models.CrossListing{
			Ticker:   symbol,
			Exchange: exchange,
			Name:     atString(a, 2),
			Currency: atString(a, 4),
		}
		priceArr := atSlice(a, 5)
		if priceArr != nil {
			cl.Price = atFloat(priceArr, 0)
			cl.Change = atFloat(priceArr, 1)
			cl.ChangePercent = atFloat(priceArr, 2)
		}
		listings = append(listings, cl)
	})

	return listings, nil
}
