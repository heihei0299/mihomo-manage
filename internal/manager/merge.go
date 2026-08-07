package manager

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func mergeConfig(baseYAML, overlayYAML string) (string, error) {
	var base, overlay any

	if strings.TrimSpace(baseYAML) == "" && strings.TrimSpace(overlayYAML) == "" {
		return "", nil
	}
	if strings.TrimSpace(overlayYAML) == "" {
		return baseYAML, nil
	}

	// Empty base: strip !replace tags from the overlay so they never leak
	// into the output. Legacy placeholder templates may not parse — keep
	// the raw text then (the deprecation warning path handles migration).
	if strings.TrimSpace(baseYAML) == "" {
		var cleaned any
		replaceFields, err := parseOverlay(overlayYAML, &cleaned)
		if err != nil {
			return overlayYAML, nil
		}
		if len(replaceFields) == 0 {
			return overlayYAML, nil
		}
		out, err := yaml.Marshal(cleaned)
		if err != nil {
			return "", fmt.Errorf("marshaling overlay YAML: %w", err)
		}
		return string(out), nil
	}

	if err := yaml.Unmarshal([]byte(baseYAML), &base); err != nil {
		return "", fmt.Errorf("parsing base YAML: %w", err)
	}

	replaceFields, err := parseOverlay(overlayYAML, &overlay)
	if err != nil {
		return "", fmt.Errorf("parsing overlay YAML: %w", err)
	}

	baseMap, baseOK := base.(map[string]any)
	overlayMap, overlayOK := overlay.(map[string]any)

	if !baseOK || !overlayOK {
		return overlayYAML, nil
	}

	merged := deepMergeMap(baseMap, overlayMap, replaceFields)

	out, err := yaml.Marshal(merged)
	if err != nil {
		return "", fmt.Errorf("marshaling merged YAML: %w", err)
	}

	return string(out), nil
}

// parseOverlay parses the overlay YAML through a node tree, collecting
// top-level fields tagged with !replace (the tag is stripped so the
// value decodes as its natural type).
func parseOverlay(overlayYAML string, out *any) (map[string]bool, error) {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(overlayYAML), &root); err != nil {
		return nil, err
	}
	replaceFields := map[string]bool{}
	if len(root.Content) > 0 && root.Content[0].Kind == yaml.MappingNode {
		m := root.Content[0]
		for i := 0; i+1 < len(m.Content); i += 2 {
			val := m.Content[i+1]
			if val.Tag == "!replace" {
				replaceFields[m.Content[i].Value] = true
				val.Tag = "" // strip the tag so Decode infers the natural type
			}
		}
	}
	return replaceFields, root.Decode(out)
}


var appendFields = map[string]bool{
	"proxies":         true,
	"proxy-groups":    true,
	"rules":           true,
	"proxy-providers": true,
	"rule-providers":  true,
}

func deepMergeMap(base, overlay map[string]any, replaceFields map[string]bool) map[string]any {
	result := make(map[string]any, len(base))

	for k, v := range base {
		result[k] = v
	}

	for k, v := range overlay {
		baseVal, exists := base[k]
		if !exists {
			result[k] = v
			continue
		}

		if replaceFields[k] {
			result[k] = v
			continue
		}

		baseMap, baseIsMap := baseVal.(map[string]any)
		overlayMap, overlayIsMap := v.(map[string]any)

		if baseIsMap && overlayIsMap {
			result[k] = deepMergeMap(baseMap, overlayMap, replaceFields)
			continue
		}

		if appendFields[k] {
			baseList, baseIsList := baseVal.([]any)
			overlayList, overlayIsList := v.([]any)
			if baseIsList && overlayIsList {
				result[k] = append(baseList, overlayList...)
				continue
			}
		}

		// Everything else (scalars, non-append arrays) is overridden by the overlay
		result[k] = v
	}

	return result
}
