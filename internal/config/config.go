package config

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

// Profile holds a parsed AmneziaWG / wg-quick configuration.
type Profile struct {
	Name    string
	Raw     string
	Address []string
	DNS     []string
	Endpoint string
}

// ParseWgQuick parses standard wg-quick text, including AmneziaWG obfuscation
// fields (Jc, Jmin, Jmax, S1-S4, H1-H4, I1-I5).
func ParseWgQuick(name, text string) (*Profile, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("config is empty")
	}

	p := &Profile{
		Name: name,
		Raw:  text + "\n",
	}

	scanner := bufio.NewScanner(strings.NewReader(text))
	section := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.Trim(line, "[]"))
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		switch section {
		case "interface":
			switch key {
			case "Address":
				p.Address = splitCSV(value)
			case "DNS":
				p.DNS = splitCSV(value)
			}
		case "peer":
			if key == "Endpoint" {
				p.Endpoint = value
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(p.Address) == 0 {
		return nil, fmt.Errorf("config missing [Interface] Address")
	}
	if p.Endpoint == "" {
		return nil, fmt.Errorf("config missing [Peer] Endpoint")
	}
	if _, _, err := net.SplitHostPort(p.Endpoint); err != nil {
		return nil, fmt.Errorf("invalid endpoint %q: %w", p.Endpoint, err)
	}

	return p, nil
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
