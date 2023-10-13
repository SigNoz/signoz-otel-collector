package bodyparser

import (
	"regexp"
	"strconv"

	"go.opentelemetry.io/collector/pdata/plog"
)

type Heroku struct {
	parser *regexp.Regexp
	names  []string
}

func NewHeroku() *Heroku {
	regex, err := regexp.Compile(`^<(?P<priority>\d|\d{2}|1[1-8]\d|19[01])>(?P<version>\d{1,2})\s(?P<timestamp>-|[^\s]+)\s(?P<hostname>[\S]{1,255})\s(?P<appname>[\S]{1,48})\s(?P<procid>[\S]{1,128})\s(?P<msgid>[\S]{1,32})(?:\s(?P<msg>.+))?$`)
	if err != nil {
		panic(err)
	}

	names := regex.SubexpNames()

	return &Heroku{
		parser: regex,
		names:  names,
	}
}
func (l *Heroku) Parse(body []byte, records plog.LogRecordSlice) {
	data := string(body)
	result := []map[string]interface{}{}
	logStrings := octectCountingSplitter(data)
	for _, logString := range logStrings {
		res := l.parser.FindStringSubmatch(logString)
		tresult := map[string]interface{}{}
		for index, name := range l.names {
			tresult[name] = res[index]
		}
		result = append(result, tresult)
	}

	records.EnsureCapacity(len(logStrings))

	for _, event := range result {
		lr := records.AppendEmpty()

		attrs := lr.Attributes()
		attrs.EnsureCapacity(7)

		for k, val := range event {
			if k != "msg" && k != "" {
				attrs.PutStr(k, val.(string))
			}
		}
		lr.Body().SetStr(event["msg"].(string))
	}

}

func octectCountingSplitter(data string) []string {
	strings := []string{}

	index := 0
	lx := len(data)
	for {
		if index >= lx {
			break
		}
		lenghStr := ""

		// ignore tabs and spaces
		for {
			if index >= lx || (data[index] != ' ' && data[index] != '\t' && data[index] != '\n') {
				break
			}
			index++
		}

		for i := index; i < lx; i++ {
			if data[i] == ' ' {
				break
			}
			index++
			lenghStr += string(data[i])
		}

		length, _ := strconv.Atoi(lenghStr)
		end := index + length
		strings = append(strings, data[index+1:end])
		index = end
	}
	return strings
}
