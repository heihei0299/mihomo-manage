package manager

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	replaceDirective = "!replace"
	deleteDirective  = "!delete"
)

func isDirectiveTag(tag string) bool {
	return tag == replaceDirective || tag == deleteDirective
}

// isTerminalRule reports whether rule is mihomo's fallback rule. mihomo
// trims the rule type, so any casing and stray spaces around the type field
// still count; a rule like MATCHING must not be mistaken for the fallback.
func isTerminalRule(rule string) bool {
	ruleType := rule
	if comma := strings.IndexByte(rule, ','); comma >= 0 {
		ruleType = rule[:comma]
	}
	return strings.EqualFold(strings.TrimSpace(ruleType), "MATCH")
}

// mergeConfig renders base YAML with overlay YAML. Overlay directives are
// honoured at every level and never reach the rendered output.
func mergeConfig(baseYAML, overlayYAML string) (string, error) {
	if strings.TrimSpace(baseYAML) == "" && strings.TrimSpace(overlayYAML) == "" {
		return "", nil
	}
	if strings.TrimSpace(overlayYAML) == "" {
		return baseYAML, nil
	}

	overlayRoot, err := parseRootNode(overlayYAML)
	if err != nil {
		// Legacy placeholder templates may not parse as YAML; keep the raw
		// text so the deprecation path can migrate them.
		if strings.TrimSpace(baseYAML) == "" && isLegacyOverlay(overlayYAML) {
			return overlayYAML, nil
		}
		return "", fmt.Errorf("parsing overlay YAML: %w", err)
	}

	if err := rejectDuplicateOverlayKeys(overlayRoot, nil); err != nil {
		return "", err
	}

	switch overlayRoot.Tag {
	case deleteDirective:
		return "", fmt.Errorf("document: !delete cannot be applied to the whole document")
	case replaceDirective:
		return "", fmt.Errorf("document: !replace cannot be applied to the whole document")
	}

	if strings.TrimSpace(baseYAML) == "" {
		if !containsDirective(overlayRoot) {
			return overlayYAML, nil
		}
		merged, err := mergeNode(nil, overlayRoot, nil)
		if err != nil {
			return "", err
		}
		return marshalMerged(merged)
	}

	baseRoot, err := parseRootNode(baseYAML)
	if err != nil {
		return "", fmt.Errorf("parsing base YAML: %w", err)
	}

	if baseRoot.Kind != yaml.MappingNode || overlayRoot.Kind != yaml.MappingNode {
		// A non-mapping overlay replaces the base wholesale. Without
		// directives the raw text is preserved as before.
		if !containsDirective(overlayRoot) {
			return overlayYAML, nil
		}
		var merged *yaml.Node
		if overlayRoot.Kind == yaml.MappingNode {
			// A non-mapping base has no keys to delete; mapping directives
			// are still validated against the empty base.
			merged, err = mergeNode(nil, overlayRoot, nil)
		} else {
			merged, err = mergeReplacement(overlayRoot, nil)
		}
		if err != nil {
			return "", err
		}
		return marshalMerged(merged)
	}

	merged, err := mergeMapping(baseRoot, overlayRoot, nil)
	if err != nil {
		return "", err
	}
	return marshalMerged(merged)
}

func isLegacyOverlay(overlayYAML string) bool {
	return strings.Contains(overlayYAML, "{{subscription}}") || strings.Contains(overlayYAML, "{{routing_rules}}")
}

func parseRootNode(text string) (*yaml.Node, error) {
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(text), &document); err != nil {
		return nil, err
	}
	if len(document.Content) == 0 {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
	}
	return document.Content[0], nil
}

func marshalMerged(node *yaml.Node) (string, error) {
	// Render through the generic value tree to keep the historical key
	// ordering of generated configs; key order and comments are explicitly
	// outside the merge contract.
	var value any
	if err := node.Decode(&value); err != nil {
		return "", fmt.Errorf("decoding merged YAML: %w", err)
	}
	out, err := yaml.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshaling merged YAML: %w", err)
	}
	return string(out), nil
}

func containsDirective(node *yaml.Node) bool {
	if node == nil {
		return false
	}
	if isDirectiveTag(node.Tag) {
		return true
	}
	for _, child := range node.Content {
		if containsDirective(child) {
			return true
		}
	}
	return false
}

// rejectDuplicateOverlayKeys fails on repeated mapping keys anywhere in the
// overlay. YAML node parsing keeps both entries; silently keeping the last one
// would contradict the identity-set rules that reject duplicates. A delete and
// an add for the same key report the more specific conflict.
func rejectDuplicateOverlayKeys(node *yaml.Node, path []string) error {
	if node == nil {
		return nil
	}
	switch node.Kind {
	case yaml.MappingNode:
		seen := make(map[string]*yaml.Node, len(node.Content)/2)
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]
			keyPath := joinPath(path, key.Value)
			if previous, exists := seen[key.Value]; exists {
				previousDeleted := previous.Tag == deleteDirective
				currentDeleted := value.Tag == deleteDirective
				if previousDeleted != currentDeleted {
					return fmt.Errorf("%s: both deleted and added", pathString(keyPath))
				}
				return fmt.Errorf("%s: duplicate entry in override", pathString(keyPath))
			}
			seen[key.Value] = value
			if err := rejectDuplicateOverlayKeys(value, keyPath); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for i, item := range node.Content {
			if err := rejectDuplicateOverlayKeys(item, itemPath(path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

var identityArrayKeys = map[string]bool{
	"proxies":         true,
	"proxy-groups":    true,
	"proxy-providers": true,
	"rule-providers":  true,
}

func mergeNode(base, overlay *yaml.Node, path []string) (*yaml.Node, error) {
	if overlay.Tag == replaceDirective {
		return mergeReplacement(overlay, path)
	}

	switch overlay.Kind {
	case yaml.MappingNode:
		if base == nil || base.Kind != yaml.MappingNode {
			base = newMappingNode()
		}
		if isProviderMap(path) {
			return mergeProviderMap(base, overlay, path)
		}
		return mergeMapping(base, overlay, path)
	case yaml.SequenceNode:
		if base == nil || base.Kind != yaml.SequenceNode {
			base = newSequenceNode()
		}
		switch {
		case isIdentityArrayPath(path):
			return mergeIdentityArray(base, overlay, path)
		case isRulesPath(path):
			return mergeRules(base, overlay, path)
		}
		return mergeReplacement(overlay, path)
	default:
		return mergeReplacement(overlay, path)
	}
}

// mergeReplacement strips every directive tag from a node that replaces its
// base wholesale. A nested !delete cannot be applied to a replaced subtree.
func mergeReplacement(node *yaml.Node, path []string) (*yaml.Node, error) {
	explicit := node.Tag == replaceDirective
	if err := stripDirectives(node, path, explicit); err != nil {
		return nil, err
	}
	return node, nil
}

func stripDirectives(node *yaml.Node, path []string, explicitReplace bool) error {
	switch node.Tag {
	case deleteDirective:
		if explicitReplace {
			return fmt.Errorf("%s: !delete is not allowed inside !replace", pathString(path))
		}
		return fmt.Errorf("%s: !delete is not allowed inside a replaced value", pathString(path))
	case replaceDirective:
		node.Tag = ""
	}

	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			if err := rejectDirectiveKey(key, path); err != nil {
				return err
			}
			if err := stripDirectives(node.Content[i+1], joinPath(path, key.Value), explicitReplace); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for i, item := range node.Content {
			if err := stripDirectives(item, itemPath(path, i), explicitReplace); err != nil {
				return err
			}
		}
	}
	return nil
}

func mergeMapping(base, overlay *yaml.Node, path []string) (*yaml.Node, error) {
	for i := 0; i+1 < len(overlay.Content); i += 2 {
		keyNode := overlay.Content[i]
		valueNode := overlay.Content[i+1]
		key := keyNode.Value
		fieldPath := joinPath(path, key)
		if err := rejectDirectiveKey(keyNode, path); err != nil {
			return nil, err
		}

		if valueNode.Tag == deleteDirective {
			if err := validateDeleteValue(valueNode, fieldPath); err != nil {
				return nil, err
			}
			index := findMappingEntry(base, key)
			if index < 0 {
				return nil, fmt.Errorf("%s: !delete target not found", pathString(fieldPath))
			}
			base.Content = append(base.Content[:index], base.Content[index+2:]...)
			continue
		}

		if valueNode.Tag == replaceDirective {
			replacement, err := mergeReplacement(valueNode, fieldPath)
			if err != nil {
				return nil, err
			}
			setMappingEntry(base, keyNode, replacement)
			continue
		}

		merged, err := mergeNode(mappingValue(base, key), valueNode, fieldPath)
		if err != nil {
			return nil, err
		}
		setMappingEntry(base, keyNode, merged)
	}
	return base, nil
}

// mergeProviderMap handles proxy-providers/rule-providers, whose identity is
// the mapping key rather than a name field. A same-named provider is replaced
// whole; new providers are appended.
func mergeProviderMap(base, overlay *yaml.Node, path []string) (*yaml.Node, error) {
	for i := 0; i+1 < len(overlay.Content); i += 2 {
		keyNode := overlay.Content[i]
		valueNode := overlay.Content[i+1]
		key := keyNode.Value
		fieldPath := joinPath(path, key)
		if err := rejectDirectiveKey(keyNode, path); err != nil {
			return nil, err
		}

		if valueNode.Tag == deleteDirective {
			if err := validateDeleteValue(valueNode, fieldPath); err != nil {
				return nil, err
			}
			index := findMappingEntry(base, key)
			if index < 0 {
				return nil, fmt.Errorf("%s: !delete target not found", pathString(fieldPath))
			}
			base.Content = append(base.Content[:index], base.Content[index+2:]...)
			continue
		}

		replacement, err := mergeReplacement(valueNode, fieldPath)
		if err != nil {
			return nil, err
		}
		setMappingEntry(base, keyNode, replacement)
	}
	return base, nil
}

func mergeIdentityArray(base, overlay *yaml.Node, path []string) (*yaml.Node, error) {
	deleted := map[string]bool{}
	added := map[string]bool{}

	for i, item := range overlay.Content {
		position := itemPath(path, i)

		if item.Tag == deleteDirective {
			if item.Kind != yaml.ScalarNode || item.Value == "" {
				return nil, fmt.Errorf("%s: !delete array item must name an entry", pathString(position))
			}
			name := item.Value
			itemNamePath := joinPath(path, name)
			if added[name] {
				return nil, fmt.Errorf("%s: both deleted and added", pathString(itemNamePath))
			}
			if !removeNamedItems(base, name) {
				return nil, fmt.Errorf("%s: !delete target not found", pathString(itemNamePath))
			}
			deleted[name] = true
			continue
		}

		name, ok := mappingName(item)
		if !ok {
			return nil, fmt.Errorf("%s: identity array item must have a name", pathString(position))
		}
		itemNamePath := joinPath(path, name)
		if deleted[name] {
			return nil, fmt.Errorf("%s: both deleted and added", pathString(itemNamePath))
		}
		if added[name] {
			return nil, fmt.Errorf("%s: duplicate entry in override", pathString(itemNamePath))
		}
		added[name] = true

		replacement, err := mergeReplacement(item, itemNamePath)
		if err != nil {
			return nil, err
		}
		replaceNamedItem(base, name, replacement)
	}
	return base, nil
}

func mergeRules(base, overlay *yaml.Node, path []string) (*yaml.Node, error) {
	deleted := map[string]bool{}
	added := map[string]bool{}
	additions := make([]*yaml.Node, 0, len(overlay.Content))

	for i, item := range overlay.Content {
		position := itemPath(path, i)

		if item.Tag == deleteDirective {
			if item.Kind != yaml.ScalarNode || item.Value == "" {
				return nil, fmt.Errorf("%s: !delete rule must be a rule text", pathString(position))
			}
			rule := item.Value
			rulePath := joinPath(path, rule)
			if added[rule] {
				return nil, fmt.Errorf("%s: both deleted and added", pathString(rulePath))
			}
			if !removeRuleItems(base, rule) {
				return nil, fmt.Errorf("%s: !delete target not found", pathString(rulePath))
			}
			deleted[rule] = true
			continue
		}

		if item.Kind == yaml.ScalarNode {
			rule := item.Value
			if deleted[rule] {
				return nil, fmt.Errorf("%s: both deleted and added", pathString(joinPath(path, rule)))
			}
			added[rule] = true
		}
		replacement, err := mergeReplacement(item, position)
		if err != nil {
			return nil, err
		}
		additions = append(additions, replacement)
	}

	insertAt := len(base.Content)
	for i, item := range base.Content {
		if item.Kind == yaml.ScalarNode && isTerminalRule(item.Value) {
			insertAt = i
			break
		}
	}
	merged := make([]*yaml.Node, 0, len(base.Content)+len(additions))
	merged = append(merged, base.Content[:insertAt]...)
	merged = append(merged, additions...)
	merged = append(merged, base.Content[insertAt:]...)
	base.Content = merged
	return base, nil
}

func rejectDirectiveKey(key *yaml.Node, path []string) error {
	if !isDirectiveTag(key.Tag) {
		return nil
	}
	example := key.Value + ": " + key.Tag + " null"
	if key.Tag == replaceDirective {
		example = key.Value + ": !replace <value>"
	}
	fieldPath := pathString(joinPath(path, key.Value))
	return fmt.Errorf("%s: %s must be attached to a value (e.g. %q), not to a mapping key", fieldPath, key.Tag, example)
}

func validateDeleteValue(node *yaml.Node, path []string) error {
	if node.Kind == yaml.ScalarNode && isNullScalar(node) {
		return nil
	}
	field := pathString(path)
	switch {
	case node.Kind == yaml.SequenceNode && isRulesPath(path):
		return fmt.Errorf("%s: !delete removes a single rule; write %q as a rules item", field, "- !delete <rule text>")
	case node.Kind == yaml.SequenceNode && isIdentityArrayPath(path):
		return fmt.Errorf("%s: !delete removes a single entry; write %q as an array item", field, "- !delete <name>")
	case node.Kind == yaml.MappingNode && isProviderMap(path):
		return fmt.Errorf("%s: !delete removes a single provider; write %q under %s", field, "<name>: !delete null", pathString(path))
	case node.Kind == yaml.MappingNode:
		return fmt.Errorf("%s: !delete requires a null value; to remove a nested key, write %q inside the mapping", field, "<key>: !delete null")
	default:
		return fmt.Errorf("%s: !delete requires a null value", field)
	}
}

func isNullScalar(node *yaml.Node) bool {
	if node.Kind != yaml.ScalarNode {
		return false
	}
	if node.Tag == "!!null" {
		return true
	}
	switch node.Value {
	case "", "null", "Null", "NULL", "~":
		return true
	}
	return false
}

func newMappingNode() *yaml.Node {
	return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
}

func newSequenceNode() *yaml.Node {
	return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
}

func findMappingEntry(mapping *yaml.Node, key string) int {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Kind == yaml.ScalarNode && mapping.Content[i].Value == key {
			return i
		}
	}
	return -1
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	index := findMappingEntry(mapping, key)
	if index < 0 {
		return nil
	}
	return mapping.Content[index+1]
}

func setMappingEntry(mapping, keyNode, valueNode *yaml.Node) {
	index := findMappingEntry(mapping, keyNode.Value)
	if index >= 0 {
		mapping.Content[index+1] = valueNode
		return
	}
	mapping.Content = append(mapping.Content, keyNode, valueNode)
}

func mappingName(item *yaml.Node) (string, bool) {
	if item.Kind != yaml.MappingNode {
		return "", false
	}
	for i := 0; i+1 < len(item.Content); i += 2 {
		if item.Content[i].Kind == yaml.ScalarNode && item.Content[i].Value == "name" && item.Content[i+1].Kind == yaml.ScalarNode {
			return item.Content[i+1].Value, true
		}
	}
	return "", false
}

func findNamedItem(list *yaml.Node, name string) int {
	for i, item := range list.Content {
		if itemName, ok := mappingName(item); ok && itemName == name {
			return i
		}
	}
	return -1
}

// replaceNamedItem keeps the first position of name and collapses duplicate
// subscription entries with the same identity.
func replaceNamedItem(list *yaml.Node, name string, replacement *yaml.Node) {
	first := findNamedItem(list, name)
	if first < 0 {
		list.Content = append(list.Content, replacement)
		return
	}
	list.Content[first] = replacement
	kept := list.Content[:first+1]
	for _, item := range list.Content[first+1:] {
		if itemName, ok := mappingName(item); ok && itemName == name {
			continue
		}
		kept = append(kept, item)
	}
	list.Content = kept
}

func removeNamedItems(list *yaml.Node, name string) bool {
	removed := false
	kept := list.Content[:0]
	for _, item := range list.Content {
		if itemName, ok := mappingName(item); ok && itemName == name {
			removed = true
			continue
		}
		kept = append(kept, item)
	}
	list.Content = kept
	return removed
}

func removeRuleItems(list *yaml.Node, rule string) bool {
	removed := false
	kept := list.Content[:0]
	for _, item := range list.Content {
		if item.Kind == yaml.ScalarNode && item.Value == rule {
			removed = true
			continue
		}
		kept = append(kept, item)
	}
	list.Content = kept
	return removed
}

// The identity whitelist and the rules placement rule apply to the documented
// top-level config fields only; a nested key that happens to share the name
// keeps the ordinary deep-merge/replace semantics.
func isIdentityArrayPath(path []string) bool {
	return len(path) == 1 && identityArrayKeys[path[0]]
}

func isProviderMap(path []string) bool {
	if len(path) != 1 {
		return false
	}
	switch path[0] {
	case "proxy-providers", "rule-providers":
		return true
	}
	return false
}

func isRulesPath(path []string) bool {
	return len(path) == 1 && path[0] == "rules"
}

func joinPath(path []string, key string) []string {
	out := make([]string, len(path), len(path)+1)
	copy(out, path)
	return append(out, key)
}

func itemPath(path []string, index int) []string {
	out := make([]string, len(path), len(path)+1)
	copy(out, path)
	return append(out, fmt.Sprintf("[%d]", index))
}

func pathString(path []string) string {
	if len(path) == 0 {
		return "document"
	}
	var b strings.Builder
	for i, part := range path {
		if i > 0 && !strings.HasPrefix(part, "[") {
			b.WriteByte('.')
		}
		b.WriteString(part)
	}
	return b.String()
}
