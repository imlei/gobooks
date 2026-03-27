package numbering

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

var rulesFieldRe = regexp.MustCompile(`^rules\[([^\]]+)\]\[(prefix|next_number|padding_length)\]$`)

// ParseRulesPost reads rules[module][field] form fields from a POST body.
func ParseRulesPost(c *fiber.Ctx) ([]DisplayRule, error) {
	defaults := DefaultDisplayRules()
	byKey := make(map[string]*DisplayRule, len(defaults))
	for i := range defaults {
		r := defaults[i]
		row := r
		byKey[r.ModuleKey] = &row
	}

	c.Context().PostArgs().VisitAll(func(k, v []byte) {
		key := string(k)
		val := strings.TrimSpace(string(v))
		m := rulesFieldRe.FindStringSubmatch(key)
		if len(m) != 3 {
			return
		}
		mod := m[1]
		field := m[2]
		row, ok := byKey[mod]
		if !ok {
			return
		}
		switch field {
		case "prefix":
			row.Prefix = val
		case "next_number":
			n, err := strconv.Atoi(val)
			if err != nil {
				n = 0
			}
			row.NextNumber = n
		case "padding_length":
			n, err := strconv.Atoi(val)
			if err != nil {
				n = 0
			}
			row.PaddingLength = n
		}
	})

	out := make([]DisplayRule, 0, len(defaults))
	for _, d := range defaults {
		row := byKey[d.ModuleKey]
		v := strings.TrimSpace(c.FormValue("rules[" + d.ModuleKey + "][enabled]"))
		row.Enabled = v == "true" || v == "1" || v == "on"
		row.ModuleName = d.ModuleName
		nr := NormalizeRule(*row)
		out = append(out, nr)
	}
	return out, nil
}
