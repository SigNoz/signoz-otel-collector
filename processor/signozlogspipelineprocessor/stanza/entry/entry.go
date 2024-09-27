package signozstanzaentry

import (
	"encoding/json"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

type Entry struct {
	entry.Entry

	parsedJsonBody map[string]any
}

func (e *Entry) bodyJson() map[string]any {
	if e.parsedJsonBody == nil {
		parseBodyJson := func() map[string]any {
			bodyMap, isAlreadyMap := e.Body.(map[string]any)
			if isAlreadyMap {
				return bodyMap
			}

			bodyBytes, hasBytes := e.Body.([]byte)
			if !hasBytes {
				bodyStr, isStr := e.Body.(string)
				if isStr {
					bodyBytes = []byte(bodyStr)
					hasBytes = true
				}
			}

			if hasBytes {
				var parsedBody map[string]any
				err := json.Unmarshal(bodyBytes, &parsedBody)
				if err == nil {
					return parsedBody
				}
			}
			return map[string]any{}
		}

		e.parsedJsonBody = parseBodyJson()
	}

	return e.parsedJsonBody
}
