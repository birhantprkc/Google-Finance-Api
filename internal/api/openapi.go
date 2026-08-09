package api

import (
	"bytes"
	"encoding/json"
	"log"
	"strconv"
)

// kv is one ordered key/value pair of a JSON object.
type kv struct {
	k string
	v any
}

// obj is a JSON object that marshals its keys in declaration order. A plain
// map[string]any would sort keys alphabetically, which scrambles both the
// document layout and the property order of every schema.
type obj []kv

func (o obj) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, e := range o {
		if i > 0 {
			b.WriteByte(',')
		}
		key, err := encodeValue(e.k)
		if err != nil {
			return nil, err
		}
		b.Write(key)
		b.WriteByte(':')
		val, err := encodeValue(e.v)
		if err != nil {
			return nil, err
		}
		b.Write(val)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// encodeValue marshals v with HTML escaping disabled. Plain json.Marshal always
// escapes, which would turn readable text such as "S&P 500" into "S&P 500".
func encodeValue(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// marshalJSON renders v as indented JSON without HTML escaping.
func marshalJSON(v any) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		log.Printf("openapi: encode error: %v", err)
		return nil
	}
	return buf.Bytes()
}

// Schema construction helpers.

func typed(t string) obj { return obj{{"type", t}} }

func schemaRef(name string) obj { return obj{{"$ref", "#/components/schemas/" + name}} }

func arrayOf(items obj) obj { return obj{{"type", "array"}, {"items", items}} }

func schemaObject(props ...kv) obj {
	return obj{{"type", "object"}, {"properties", obj(props)}}
}

func prop(name, t string) kv { return kv{name, typed(t)} }

func propEnum(name string, values ...string) kv {
	return kv{name, obj{{"type", "string"}, {"enum", values}}}
}

func propRef(name, schema string) kv { return kv{name, schemaRef(schema)} }

func propArray(name, schema string) kv { return kv{name, arrayOf(schemaRef(schema))} }

func jsonResponse(description string, schema obj) obj {
	return obj{
		{"description", description},
		{"content", obj{{"application/json", obj{{"schema", schema}}}}},
	}
}

func queryParam(name, description string, schema obj) obj {
	return obj{{"name", name}, {"in", "query"}, {"description", description}, {"schema", schema}}
}

func stringEnum(def string, values ...string) obj {
	return obj{{"type", "string"}, {"enum", values}, {"default", def}}
}

// tickerParam references the shared path parameter defined in components.
var tickerParam = obj{{"$ref", "#/components/parameters/ticker"}}

// errorResponseNames maps a status code to its components.responses key. Every
// entry mirrors the status code convention implemented by the handlers.
var errorResponseNames = map[int]string{
	400: "BadRequest",
	404: "NotFound",
	500: "InternalError",
	502: "BadGateway",
	503: "ServiceUnavailable",
}

// Standard error sets shared by groups of endpoints.
var (
	tickerErrors = []int{400, 404, 500, 502}
	marketErrors = []int{404, 500, 502}
)

func openAPIParameters() obj {
	return obj{
		{"ticker", obj{
			{"name", "ticker"},
			{"in", "path"},
			{"required", true},
			{"description", "Ticker in format SYMBOL:EXCHANGE (stocks, ETFs, indices) or BASE-QUOTE (crypto, forex). Examples: GOOGL:NASDAQ, THYAO:IST, BTC-USD"},
			{"schema", typed("string")},
		}},
	}
}

func openAPIResponses() obj {
	errorSchema := schemaRef("Error")
	return obj{
		{"BadRequest", jsonResponse("Invalid ticker format or query parameter", errorSchema)},
		{"NotFound", jsonResponse("No data available for the requested ticker", errorSchema)},
		{"InternalError", jsonResponse("Failed to decode the upstream response", errorSchema)},
		{"BadGateway", jsonResponse("Upstream service unavailable", errorSchema)},
		{"ServiceUnavailable", jsonResponse("Live stream is at capacity", errorSchema)},
	}
}

func openAPISchemas() obj {
	return obj{
		{"Quote", schemaObject(
			prop("ticker", "string"),
			prop("exchange", "string"),
			prop("name", "string"),
			propEnum("type", "stock", "index", "crypto", "etf", "unknown"),
			prop("currency", "string"),
			prop("timezone", "string"),
			prop("price", "number"),
			prop("change", "number"),
			prop("changePercent", "number"),
			prop("previousClose", "number"),
			propRef("afterHours", "AfterHours"),
		)},
		{"AfterHours", schemaObject(
			prop("price", "number"),
			prop("change", "number"),
			prop("changePercent", "number"),
		)},
		{"CompanyInfo", schemaObject(
			prop("description", "string"),
			prop("ceo", "string"),
			prop("employees", "integer"),
			prop("marketCap", "number"),
			prop("open", "number"),
			prop("high", "number"),
			prop("low", "number"),
			prop("fiftyTwoWeekHigh", "number"),
			prop("fiftyTwoWeekLow", "number"),
			prop("peRatio", "number"),
			prop("volume", "integer"),
			prop("sector", "string"),
		)},
		{"ChartData", schemaObject(
			prop("previousClose", "number"),
			propArray("points", "ChartPoint"),
		)},
		{"ChartPoint", schemaObject(
			prop("date", "string"),
			prop("price", "number"),
			prop("volume", "integer"),
		)},
		{"NewsItem", schemaObject(
			prop("title", "string"),
			prop("source", "string"),
			prop("url", "string"),
			prop("timestamp", "integer"),
		)},
		{"FinancialPeriod", schemaObject(
			prop("fiscalEnd", "string"),
			prop("isAnnual", "boolean"),
			prop("currency", "string"),
			prop("revenue", "number"),
			prop("netIncome", "number"),
			prop("eps", "number"),
			prop("epsDiluted", "number"),
			prop("operatingMargin", "number"),
			prop("peRatio", "number"),
		)},
		{"RelatedStock", schemaObject(
			prop("ticker", "string"),
			prop("name", "string"),
			prop("price", "number"),
			prop("change", "number"),
			prop("changePercent", "number"),
		)},
		{"ClassificationLabel", schemaObject(
			prop("label", "string"),
			prop("description", "string"),
		)},
		{"CrossListing", schemaObject(
			prop("ticker", "string"),
			prop("exchange", "string"),
			prop("name", "string"),
			prop("price", "number"),
			prop("change", "number"),
			prop("changePercent", "number"),
			prop("currency", "string"),
		)},
		{"MarketIndex", schemaObject(
			prop("ticker", "string"),
			prop("name", "string"),
			prop("price", "number"),
			prop("change", "number"),
			prop("changePercent", "number"),
		)},
		{"MarketMover", schemaObject(
			prop("ticker", "string"),
			prop("name", "string"),
			prop("price", "number"),
			prop("change", "number"),
			prop("changePercent", "number"),
		)},
		{"EarningsEvent", schemaObject(
			prop("ticker", "string"),
			prop("name", "string"),
			prop("date", "string"),
			prop("exchange", "string"),
		)},
		{"Headline", schemaObject(
			prop("title", "string"),
			prop("url", "string"),
			prop("source", "string"),
		)},
		{"FullQuote", schemaObject(
			propRef("quote", "Quote"),
			propRef("company", "CompanyInfo"),
			propRef("chart", "ChartData"),
			propArray("news", "NewsItem"),
		)},
		{"Error", schemaObject(
			prop("error", "string"),
			prop("message", "string"),
		)},
	}
}

// renderOpenAPI builds the OpenAPI 3.1 document from the endpoint catalog, so
// the spec can never list an endpoint the server does not serve, or miss one it
// does. The version comes from the embedded VERSION file and the server URL from
// the resolved request base URL.
func renderOpenAPI(baseURL, version string, eps []apiEndpoint) []byte {
	paths := make(obj, 0, len(eps))
	for _, e := range eps {
		op := obj{
			{"tags", []string{e.Group}},
			{"summary", e.Title},
			{"description", e.Desc},
			{"operationId", e.OperationID},
		}
		if len(e.Params) > 0 {
			op = append(op, kv{"parameters", e.Params})
		}

		responses := obj{{"200", e.Response}}
		for _, code := range e.Errors {
			name, ok := errorResponseNames[code]
			if !ok {
				continue
			}
			responses = append(responses, kv{
				strconv.Itoa(code),
				obj{{"$ref", "#/components/responses/" + name}},
			})
		}
		op = append(op, kv{"responses", responses})

		paths = append(paths, kv{e.Path, obj{{"get", op}}})
	}

	tags := make([]obj, 0, len(groupOrder))
	for _, g := range groupOrder {
		if len(endpointsByGroup(eps, g)) == 0 {
			continue
		}
		tags = append(tags, obj{{"name", g}, {"description", groupDescriptions[g]}})
	}

	doc := obj{
		{"openapi", "3.1.0"},
		{"info", obj{
			{"title", "Google Finance API"},
			{"description", projectSummary},
			{"version", version},
			{"license", obj{
				{"name", "MIT"},
				{"url", repoURL + "/blob/main/LICENSE"},
			}},
		}},
		{"servers", []obj{{{"url", baseURL}, {"description", "Current host"}}}},
		{"tags", tags},
		{"paths", paths},
		{"components", obj{
			{"parameters", openAPIParameters()},
			{"responses", openAPIResponses()},
			{"schemas", openAPISchemas()},
		}},
	}

	return marshalJSON(doc)
}
