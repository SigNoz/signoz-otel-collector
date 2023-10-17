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

type rAttrs struct {
	priority string
	version  string
	hostname string
	appname  string
	procid   string
}
type log struct {
	timestamp string
	msgid     string
	body      string
}

func (l *Heroku) Parse(body []byte) (plog.Logs, int) {
	data := string(body)

	loglines := octectCountingSplitter(data)

	resdata := map[rAttrs][]log{}
	for _, line := range loglines {
		parsedLog := l.parser.FindStringSubmatch(line)

		if len(parsedLog) != len(l.names) {
			//TODO: do something here to conver that it wasn't parsed
			resdata[rAttrs{}] = append(resdata[rAttrs{}], log{
				body: line,
			})
		} else {
			d := rAttrs{
				priority: parsedLog[1],
				version:  parsedLog[2],
				hostname: parsedLog[4],
				appname:  parsedLog[5],
				procid:   parsedLog[6],
			}

			resdata[d] = append(resdata[d], log{
				// for timestamp as of now not parsing and leaving it to the user if they want to map it to actual timestamp
				// can change it later if required
				timestamp: parsedLog[3],
				msgid:     parsedLog[7],
				body:      parsedLog[8],
			})
		}
	}

	ld := plog.NewLogs()
	for resource, logbodies := range resdata {
		rl := ld.ResourceLogs().AppendEmpty()
		if resource != (rAttrs{}) {
			rl.Resource().Attributes().EnsureCapacity(5)
			rl.Resource().Attributes().PutStr("priority", resource.priority)
			rl.Resource().Attributes().PutStr("version", resource.version)
			rl.Resource().Attributes().PutStr("hostname", resource.hostname)
			rl.Resource().Attributes().PutStr("appname", resource.appname)
			rl.Resource().Attributes().PutStr("procid", resource.procid)
		}

		sl := rl.ScopeLogs().AppendEmpty()
		for _, log := range logbodies {
			rec := sl.LogRecords().AppendEmpty()
			rec.Body().SetStr(log.body)
			if log.timestamp != "" {
				rec.Attributes().EnsureCapacity(2)
				rec.Attributes().PutStr("timestamp", log.timestamp)
				rec.Attributes().PutStr("msgid", log.msgid)
			}
		}
	}
	return ld, len(loglines)

}

func octectCountingSplitter(data string) []string {
	strings := []string{}

	index := 0
	lx := len(data)
	for {
		lenghStr := ""

		// ignore tabs and spaces
		for {
			if index >= lx || (data[index] != ' ' && data[index] != '\t' && data[index] != '\n') {
				break
			}
			index++
		}

		if index >= lx {
			break
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
