package decode

import (
	"encoding/json"

	"github.com/kilimcininkoroglu/google-finance-api/internal/models"
)

// Analyst decodes the o6pODe payload. The structure is data[0] = a list of
// groups, each group = [tag, [articles]], where an article shares the news
// item layout [url, title, source, image, timestamp, ...]. Groups are
// flattened into a single de-duplicated news list.
func Analyst(raw json.RawMessage) ([]models.NewsItem, error) {
	arr, err := unmarshalNested(raw)
	if err != nil {
		return nil, err
	}

	groups := atSlice(arr, 0)
	if groups == nil {
		return nil, nil
	}

	seen := make(map[string]bool)
	var news []models.NewsItem
	for _, group := range groups {
		g, ok := group.([]any)
		if !ok {
			continue
		}
		articles := atSlice(g, 1)
		for _, item := range articles {
			a, ok := item.([]any)
			if !ok || len(a) < 3 {
				continue
			}
			title := atString(a, 1)
			url := atString(a, 0)
			if title == "" || seen[url] {
				continue
			}
			seen[url] = true

			n := models.NewsItem{
				URL:    url,
				Title:  title,
				Source: atString(a, 2),
			}
			if len(a) > 4 {
				n.Timestamp = atInt64(a, 4)
			}
			news = append(news, n)
		}
	}

	return news, nil
}
