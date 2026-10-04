package usage

import "strings"

// listPrices are Anthropic's API list prices per million tokens, by model id
// prefix, the longest prefix winning; a transcript's id may carry a date
// (claude-sonnet-4-5-20250929). A cache write is priced at the 5-minute
// rate: the transcript does not say which duration was written. A
// subscription pays none of this; it is what the same work costs on the API.
var listPrices = []struct {
	prefix string
	price  Price
}{
	{"claude-fable-5-1", Price{Input: 10, CacheWrite: 12.5, CacheRead: 0.25, Output: 50}},
	{"claude-mythos-5-1", Price{Input: 10, CacheWrite: 12.5, CacheRead: 0.25, Output: 50}},
	{"claude-fable-5", Price{Input: 10, CacheWrite: 12.5, CacheRead: 1, Output: 50}},
	{"claude-mythos-5", Price{Input: 10, CacheWrite: 12.5, CacheRead: 1, Output: 50}},
	{"claude-opus-5-5", Price{Input: 4, CacheWrite: 5, CacheRead: 0.2, Output: 20}},
	{"claude-opus-5", Price{Input: 5, CacheWrite: 6.25, CacheRead: 0.5, Output: 25}},
	{"claude-opus-4-8", Price{Input: 5, CacheWrite: 6.25, CacheRead: 0.5, Output: 25}},
	{"claude-opus-4-7", Price{Input: 5, CacheWrite: 6.25, CacheRead: 0.5, Output: 25}},
	{"claude-opus-4-6", Price{Input: 5, CacheWrite: 6.25, CacheRead: 0.5, Output: 25}},
	{"claude-opus-4-5", Price{Input: 5, CacheWrite: 6.25, CacheRead: 0.5, Output: 25}},
	{"claude-opus-4-1", Price{Input: 15, CacheWrite: 18.75, CacheRead: 1.5, Output: 75}},
	{"claude-opus-4", Price{Input: 15, CacheWrite: 18.75, CacheRead: 1.5, Output: 75}},
	{"claude-sonnet-5-5", Price{Input: 2, CacheWrite: 2.5, CacheRead: 0.2, Output: 10}},
	{"claude-sonnet-5", Price{Input: 2, CacheWrite: 2.5, CacheRead: 0.2, Output: 10}},
	{"claude-sonnet-4", Price{Input: 3, CacheWrite: 3.75, CacheRead: 0.3, Output: 15}},
	{"claude-haiku-4-5", Price{Input: 1, CacheWrite: 1.25, CacheRead: 0.1, Output: 5}},
	{"claude-3-5-haiku", Price{Input: 0.8, CacheWrite: 1, CacheRead: 0.08, Output: 4}},
}

// priceOf is a model's price: the user's own for that id first, else the
// list price of its family.
func (p Prices) priceOf(model string) (Price, bool) {
	if pr, ok := p[model]; ok {
		return pr, true
	}
	best := -1
	var got Price
	for _, lp := range listPrices {
		if strings.HasPrefix(model, lp.prefix) && len(lp.prefix) > best {
			best, got = len(lp.prefix), lp.price
		}
	}
	return got, best >= 0
}
