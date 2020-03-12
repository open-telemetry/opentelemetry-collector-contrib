package redisreceiver

import "strings"

type apiParser struct {
	client    client
	delimiter string
}

func newApiParser(client client) *apiParser {
	return &apiParser{client: client, delimiter: client.delimiter()}
}

type clientFcn func(g client) (string, error)

func (p apiParser) info() (map[string]string, error) {
	return p.parseRedisString(func(c client) (string, error) {
		return c.retrieveInfo()
	})
}

func (p apiParser) parseRedisString(fcn clientFcn) (map[string]string, error) {
	str, err := fcn(p.client)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(str, p.delimiter)
	attrs := make(map[string]string)
	for _, line := range lines {
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		pair := strings.Split(line, ":")
		if len(pair) == 2 { // defensive, should always == 2
			attrs[pair[0]] = pair[1]
		}
	}
	return attrs, nil
}
