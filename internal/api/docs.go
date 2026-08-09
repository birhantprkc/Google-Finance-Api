package api

import (
	"fmt"
	"net/http"
	"strings"
)

// apiEndpoint describes one public API route. This catalog is the single source
// of truth: it registers the routes AND renders openapi.json, llms.txt and
// index.html.md. Adding an entry here updates routing and every generated
// document at once, so the documentation cannot drift away from the server.
type apiEndpoint struct {
	Path        string // ServeMux pattern, e.g. "/v1/quote/{ticker}"
	Group       string // Ticker, Market, Live, System; also the OpenAPI tag
	Name        string // short label used in generated link lists
	Title       string // OpenAPI operation summary
	Desc        string // one-line description reused by every generated document
	Example     string // concrete, fetchable URL used in the generated docs
	OperationID string
	Params      []obj // OpenAPI parameter objects or $refs
	Response    obj   // OpenAPI 200 response object
	Errors      []int // status codes this endpoint can return besides 200
	Handler     http.HandlerFunc
}

// groupOrder fixes the section order in every generated document.
var groupOrder = []string{"Ticker", "Market", "Live", "System"}

var groupDescriptions = map[string]string{
	"Ticker": "Per-instrument data: quotes, company profile, charts, news and financials",
	"Market": "Market-wide data: indices, movers, trending stocks, earnings and headlines",
	"Live":   "Real-time price feed backed by a shared upstream poller",
	"System": "Service health",
}

// docPage is a crawlable page listed in the sitemap. API endpoints are excluded:
// they are parameterized and return live data, not indexable documents.
type docPage struct {
	Path       string
	ChangeFreq string
	Priority   string
}

var docPages = []docPage{
	{Path: "/", ChangeFreq: "weekly", Priority: "1.0"},
	{Path: "/index.html.md", ChangeFreq: "weekly", Priority: "0.9"},
	{Path: "/llms.txt", ChangeFreq: "weekly", Priority: "0.8"},
	{Path: "/openapi.json", ChangeFreq: "monthly", Priority: "0.8"},
}

const projectSummary = "Unofficial REST API wrapping Google Finance's internal batchexecute RPC endpoint. No API key required. Zero external dependencies."

const repoURL = "https://github.com/KilimcininKorOglu/Google-Finance-Api"

// apiEndpoints returns the full catalog bound to the given handlers.
func apiEndpoints(h *handlers, version string) []apiEndpoint {
	return []apiEndpoint{
		{
			Path: "/v1/quote/{ticker}", Group: "Ticker", Name: "Quote",
			Title:       "Get quote",
			Desc:        "Real-time price, change, and market data for a ticker",
			Example:     "/v1/quote/GOOGL:NASDAQ",
			OperationID: "getQuote",
			Params:      []obj{tickerParam},
			Response:    jsonResponse("Quote data", schemaRef("Quote")),
			Errors:      tickerErrors,
			Handler:     h.getQuote,
		},
		{
			Path: "/v1/company/{ticker}", Group: "Ticker", Name: "Company",
			Title:       "Get company info",
			Desc:        "Company description, CEO, sector, employees, and key metrics",
			Example:     "/v1/company/AAPL:NASDAQ",
			OperationID: "getCompany",
			Params:      []obj{tickerParam},
			Response:    jsonResponse("Company info", schemaRef("CompanyInfo")),
			Errors:      tickerErrors,
			Handler:     h.getCompany,
		},
		{
			Path: "/v1/chart/{ticker}", Group: "Ticker", Name: "Chart",
			Title:       "Get price chart",
			Desc:        "Historical price and volume data points; range=1D|5D|1M|6M|YTD|1Y|5Y|MAX (default 1M)",
			Example:     "/v1/chart/MSFT:NASDAQ?range=1M",
			OperationID: "getChart",
			Params: []obj{
				tickerParam,
				queryParam("range", "Time range of the returned series",
					stringEnum("1M", "1D", "5D", "1M", "6M", "YTD", "1Y", "5Y", "MAX")),
			},
			Response: jsonResponse("Chart data", schemaRef("ChartData")),
			Errors:   tickerErrors,
			Handler:  h.getChart,
		},
		{
			Path: "/v1/news/{ticker}", Group: "Ticker", Name: "News",
			Title:       "Get news",
			Desc:        "Latest news articles related to the ticker",
			Example:     "/v1/news/TSLA:NASDAQ",
			OperationID: "getNews",
			Params:      []obj{tickerParam},
			Response:    jsonResponse("News items", arrayOf(schemaRef("NewsItem"))),
			Errors:      tickerErrors,
			Handler:     h.getNews,
		},
		{
			Path: "/v1/financials/{ticker}", Group: "Ticker", Name: "Financials",
			Title:       "Get financials",
			Desc:        "Revenue, net income, EPS, margins, and P/E; type=annual|quarterly, both when omitted",
			Example:     "/v1/financials/GOOGL:NASDAQ?type=annual",
			OperationID: "getFinancials",
			Params: []obj{
				tickerParam,
				queryParam("type", "Restrict the result to one reporting period type",
					stringEnum("all", "quarterly", "annual", "all")),
			},
			Response: jsonResponse("Financial periods", arrayOf(schemaRef("FinancialPeriod"))),
			Errors:   tickerErrors,
			Handler:  h.getFinancials,
		},
		{
			Path: "/v1/related/{ticker}", Group: "Ticker", Name: "Related",
			Title:       "Get related stocks",
			Desc:        "Stocks related to the given ticker",
			Example:     "/v1/related/AAPL:NASDAQ",
			OperationID: "getRelated",
			Params:      []obj{tickerParam},
			Response:    jsonResponse("Related stocks", arrayOf(schemaRef("RelatedStock"))),
			Errors:      tickerErrors,
			Handler:     h.getRelated,
		},
		{
			Path: "/v1/classification/{ticker}", Group: "Ticker", Name: "Classification",
			Title:       "Get ticker classification",
			Desc:        "Descriptive classification labels such as Most active or US listed security",
			Example:     "/v1/classification/AAPL:NASDAQ",
			OperationID: "getClassification",
			Params:      []obj{tickerParam},
			Response:    jsonResponse("Classification labels", arrayOf(schemaRef("ClassificationLabel"))),
			Errors:      tickerErrors,
			Handler:     h.getClassification,
		},
		{
			Path: "/v1/analyst/{ticker}", Group: "Ticker", Name: "Analyst",
			Title:       "Get analyst coverage",
			Desc:        "Curated news and analysis articles grouped by Google Finance",
			Example:     "/v1/analyst/AAPL:NASDAQ",
			OperationID: "getAnalyst",
			Params:      []obj{tickerParam},
			Response:    jsonResponse("Analyst articles", arrayOf(schemaRef("NewsItem"))),
			Errors:      tickerErrors,
			Handler:     h.getAnalyst,
		},
		{
			Path: "/v1/context/{ticker}", Group: "Ticker", Name: "Context",
			Title:       "Get cross-exchange listings",
			Desc:        "The same instrument on other exchanges, with per-listing price and currency",
			Example:     "/v1/context/AAPL:NASDAQ",
			OperationID: "getContext",
			Params:      []obj{tickerParam},
			Response:    jsonResponse("Cross-exchange listings", arrayOf(schemaRef("CrossListing"))),
			Errors:      tickerErrors,
			Handler:     h.getContext,
		},
		{
			Path: "/v1/full/{ticker}", Group: "Ticker", Name: "Full",
			Title:       "Get full data",
			Desc:        "Combined quote, company, chart, and news in a single batched request",
			Example:     "/v1/full/GOOGL:NASDAQ",
			OperationID: "getFull",
			Params: []obj{
				tickerParam,
				queryParam("range", "Chart range; unknown values silently fall back to 1M",
					obj{{"type", "string"}, {"default", "1M"}}),
			},
			Response: jsonResponse("Full quote data", schemaRef("FullQuote")),
			Errors:   tickerErrors,
			Handler:  h.getFull,
		},

		{
			Path: "/v1/market/indices", Group: "Market", Name: "Indices",
			Title:       "Market indices",
			Desc:        "Global market indices: Dow, S&P 500, NASDAQ, DAX, Nikkei, and more",
			Example:     "/v1/market/indices",
			OperationID: "getMarketIndices",
			Response:    jsonResponse("Index list", arrayOf(schemaRef("MarketIndex"))),
			Errors:      marketErrors,
			Handler:     h.getMarketIndices,
		},
		{
			Path: "/v1/market/movers", Group: "Market", Name: "Movers",
			Title:       "Market movers",
			Desc:        "Top gainers, losers, and most active stocks; count 1-100 (default 10)",
			Example:     "/v1/market/movers?category=gainers&count=10",
			OperationID: "getMarketMovers",
			Params: []obj{
				queryParam("category", "Mover category; unknown values fall back to most-active",
					stringEnum("most-active", "most-active", "gainers", "losers")),
				queryParam("count", "Number of results; values outside 1-100 fall back to 10",
					obj{{"type", "integer"}, {"minimum", 1}, {"maximum", 100}, {"default", 10}}),
				queryParam("offset", "Number of results to skip; negative values fall back to 0",
					obj{{"type", "integer"}, {"minimum", 0}, {"default", 0}}),
			},
			Response: jsonResponse("Mover list", arrayOf(schemaRef("MarketMover"))),
			Errors:   marketErrors,
			Handler:  h.getMarketMovers,
		},
		{
			Path: "/v1/market/trending", Group: "Market", Name: "Trending",
			Title:       "Trending stocks",
			Desc:        "Currently trending stocks on Google Finance",
			Example:     "/v1/market/trending",
			OperationID: "getTrending",
			Response:    jsonResponse("Trending list", arrayOf(schemaRef("MarketMover"))),
			Errors:      marketErrors,
			Handler:     h.getTrending,
		},
		{
			Path: "/v1/market/earnings", Group: "Market", Name: "Earnings",
			Title:       "Earnings calendar",
			Desc:        "Upcoming earnings announcements",
			Example:     "/v1/market/earnings",
			OperationID: "getEarnings",
			Response:    jsonResponse("Earnings events", arrayOf(schemaRef("EarningsEvent"))),
			Errors:      marketErrors,
			Handler:     h.getEarnings,
		},
		{
			Path: "/v1/market/headlines", Group: "Market", Name: "Headlines",
			Title:       "Top headline",
			Desc:        "Current top finance headline",
			Example:     "/v1/market/headlines",
			OperationID: "getHeadlines",
			Response:    jsonResponse("Headline", schemaRef("Headline")),
			Errors:      marketErrors,
			Handler:     h.getHeadlines,
		},

		{
			Path: "/v1/live", Group: "Live", Name: "Live stream",
			Title:       "Live price stream (SSE)",
			Desc:        "Server-Sent Events price stream for 8 tickers, refreshed every 15 seconds",
			Example:     "/v1/live",
			OperationID: "liveStream",
			Response: obj{
				{"description", "SSE event stream"},
				{"content", obj{{"text/event-stream", obj{{"schema", typed("string")}}}}},
			},
			Errors:  []int{503},
			Handler: h.liveStream,
		},
		{
			Path: "/v1/live/snapshot", Group: "Live", Name: "Live snapshot",
			Title:       "Live price snapshot (JSON)",
			Desc:        "One-shot JSON array of the cached live quotes; empty until the hub has data",
			Example:     "/v1/live/snapshot",
			OperationID: "liveSnapshot",
			Response:    jsonResponse("Array of live quotes", arrayOf(schemaRef("Quote"))),
			Handler:     h.sseQuotes,
		},

		{
			Path: "/healthz", Group: "System", Name: "Health",
			Title:       "Health check",
			Desc:        "Service health and running version",
			Example:     "/healthz",
			OperationID: "healthz",
			Response: jsonResponse("OK", schemaObject(
				prop("status", "string"),
				prop("version", "string"),
			)),
			Handler: healthHandler(version),
		},
	}
}

func healthHandler(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": version})
	}
}

func endpointsByGroup(eps []apiEndpoint, group string) []apiEndpoint {
	var out []apiEndpoint
	for _, e := range eps {
		if e.Group == group {
			out = append(out, e)
		}
	}
	return out
}

const tickerFormatTable = "| Type | Format | Example |\n" +
	"| --- | --- | --- |\n" +
	"| Stock | `SYMBOL:EXCHANGE` | `GOOGL:NASDAQ`, `THYAO:IST` |\n" +
	"| Index | `.SYMBOL:EXCHANGE` | `.DJI:INDEXDJX` |\n" +
	"| Crypto | `BASE-QUOTE` | `BTC-USD`, `ETH-USD` |\n" +
	"| Forex | `BASE-QUOTE` | `EUR-USD`, `GBP-USD` |\n" +
	"| ETF | `SYMBOL:EXCHANGE` | `SPY:NYSEARCA` |\n"

// escapePipes protects a Markdown table cell from descriptions that contain a
// pipe, such as the range=1D|5D|... enumerations.
func escapePipes(s string) string {
	return strings.ReplaceAll(s, "|", `\|`)
}

const bareSymbolNote = "A bare symbol such as `AAPL` is rejected with HTTP 400; the exchange or quote currency is always required."

// renderLLMsTxt builds llms.txt per the llmstxt.org spec: an H1, a blockquote
// summary, then H2 sections whose lists use the required [name](url) hyperlinks.
func renderLLMsTxt(baseURL string, eps []apiEndpoint) []byte {
	var b strings.Builder

	fmt.Fprintf(&b, "# Google Finance API\n\n> %s\n\n", projectSummary)
	b.WriteString("Covers stocks, ETFs, indices, crypto pairs, and forex on every exchange Google Finance supports (NASDAQ, NYSE, IST, KRX, TYO, LSE, and more). Every endpoint answers a plain HTTP GET with JSON and needs no authentication.\n\n")

	b.WriteString("## Docs\n\n")
	fmt.Fprintf(&b, "- [API reference](%s/index.html.md): Every endpoint, ticker format, and example in clean Markdown\n", baseURL)
	fmt.Fprintf(&b, "- [OpenAPI 3.1 specification](%s/openapi.json): Machine-readable schema for every endpoint and response\n\n", baseURL)

	for _, group := range groupOrder {
		groupEps := endpointsByGroup(eps, group)
		if len(groupEps) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s endpoints\n\n", group)
		for _, e := range groupEps {
			fmt.Fprintf(&b, "- [%s](%s%s): %s\n", e.Name, baseURL, e.Example, e.Desc)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Ticker formats\n\n")
	b.WriteString("- Stock: `SYMBOL:EXCHANGE`, for example `GOOGL:NASDAQ` or `THYAO:IST`\n")
	b.WriteString("- Index: `.SYMBOL:EXCHANGE`, for example `.DJI:INDEXDJX`\n")
	b.WriteString("- Crypto: `BASE-QUOTE`, for example `BTC-USD`\n")
	b.WriteString("- Forex: `BASE-QUOTE`, for example `EUR-USD`\n")
	b.WriteString("- ETF: `SYMBOL:EXCHANGE`, for example `SPY:NYSEARCA`\n\n")
	b.WriteString(bareSymbolNote + "\n\n")

	b.WriteString("## Optional\n\n")
	fmt.Fprintf(&b, "- [Source code](%s): Go implementation, MIT licensed\n", repoURL)

	return []byte(b.String())
}

// renderIndexMarkdown builds the Markdown twin of the landing page. The
// llmstxt.org spec asks for a clean Markdown version of each page at the same
// URL plus ".md"; a URL without a filename uses "index.html.md" instead.
func renderIndexMarkdown(baseURL string, eps []apiEndpoint) []byte {
	var b strings.Builder

	fmt.Fprintf(&b, "# Google Finance API\n\n> %s\n\n", projectSummary)
	fmt.Fprintf(&b, "Base URL: `%s`\n\n", baseURL)

	b.WriteString("## About\n\n")
	b.WriteString("Unofficial REST API that wraps Google Finance's internal batchexecute RPC endpoint. One HTTP GET returns real-time quotes, company profiles, financial statements, charts, and news. No API key and no external dependencies.\n\n")
	b.WriteString("Covers stocks, ETFs, indices, crypto pairs, and forex on every exchange Google Finance supports: NASDAQ, NYSE, IST, KRX, TYO, LSE, and more.\n\n")

	b.WriteString("## Endpoints\n\n")
	b.WriteString("| Method | Path | Group | Description |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	for _, group := range groupOrder {
		for _, e := range endpointsByGroup(eps, group) {
			fmt.Fprintf(&b, "| GET | `%s` | %s | %s |\n", e.Path, e.Group, escapePipes(e.Desc))
		}
	}
	b.WriteString("\n")

	b.WriteString("## Examples\n\n")
	for _, group := range groupOrder {
		for _, e := range endpointsByGroup(eps, group) {
			fmt.Fprintf(&b, "- [%s](%s%s): `curl \"%s%s\"`\n", e.Name, baseURL, e.Example, baseURL, e.Example)
		}
	}
	b.WriteString("\n")

	b.WriteString("## Ticker formats\n\n")
	b.WriteString(tickerFormatTable)
	b.WriteString("\n" + bareSymbolNote + "\n\n")

	b.WriteString("## Response notes\n\n")
	b.WriteString("- Every `/v1/*` response sets `Cache-Control: no-store`; the data is live and must not be cached.\n")
	b.WriteString("- Status codes: 400 invalid input, 404 no data for the ticker, 500 decode failure, 502 upstream failure, 503 live stream at capacity.\n")
	b.WriteString("- Error bodies are `{\"error\": \"...\", \"message\": \"...\"}`.\n\n")

	b.WriteString("## Links\n\n")
	fmt.Fprintf(&b, "- [OpenAPI 3.1 specification](%s/openapi.json)\n", baseURL)
	fmt.Fprintf(&b, "- [llms.txt](%s/llms.txt)\n", baseURL)
	fmt.Fprintf(&b, "- [Source code](%s)\n", repoURL)
	b.WriteString("- License: MIT\n")

	return []byte(b.String())
}

// renderSitemap builds sitemap.xml from the crawlable page list.
func renderSitemap(baseURL string) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, p := range docPages {
		b.WriteString("  <url>\n")
		fmt.Fprintf(&b, "    <loc>%s%s</loc>\n", baseURL, p.Path)
		fmt.Fprintf(&b, "    <changefreq>%s</changefreq>\n", p.ChangeFreq)
		fmt.Fprintf(&b, "    <priority>%s</priority>\n", p.Priority)
		b.WriteString("  </url>\n")
	}
	b.WriteString("</urlset>\n")
	return []byte(b.String())
}

// renderRobots builds robots.txt and advertises the sitemap.
func renderRobots(baseURL string) []byte {
	var b strings.Builder
	b.WriteString("User-agent: *\n")
	b.WriteString("Allow: /\n\n")
	fmt.Fprintf(&b, "Sitemap: %s/sitemap.xml\n", baseURL)
	return []byte(b.String())
}
