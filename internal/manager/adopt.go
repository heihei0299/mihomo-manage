package manager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"

	"gopkg.in/yaml.v3"
)

// AdoptReport describes the differences found between the current config.yaml
// and the rendered config (subscription + override file).
type AdoptReport struct {
	NoChanges bool
	Fields    []string // scalar/map fields to be adopted, including deletions
	ArrayDiff []string // array fields that differ (reported, never adopted)
	LargeDiff bool     // len(Fields) >= 5
}

// Adopt compares the current config.yaml with the rendered config and moves
// scalar/map differences into the override file, preserving its existing
// content. Array differences are reported but never adopted. A large diff
// (>= 5 fields) requires force. Idempotent: a second run reports no changes
// once the differences have been adopted.
func (p *configPipeline) AdoptConfig(ctx context.Context, force bool) (AdoptReport, error) {
	report := AdoptReport{}
	release, err := p.acquireConfigUpdate(ctx)
	if err != nil {
		return report, err
	}
	defer release()
	if err := p.prepareLocked(); err != nil {
		return report, err
	}

	cur, err := p.fs.ReadFile(configYAML)
	if err != nil {
		if os.IsNotExist(err) {
			return report, fmt.Errorf("config.yaml does not exist; nothing to adopt")
		}
		return report, err
	}

	rendered, err := p.previewConfig(ctx, nil)
	if err != nil {
		return report, err
	}

	baseMap, err := p.subscriptionBaseMap()
	if err != nil {
		return report, err
	}

	var curMap, rendMap map[string]any
	if err := yaml.Unmarshal(cur, &curMap); err != nil {
		return report, fmt.Errorf("parsing config.yaml: %w", err)
	}
	if err := yaml.Unmarshal([]byte(rendered), &rendMap); err != nil {
		return report, fmt.Errorf("parsing rendered config: %w", err)
	}

	for k, v := range curMap {
		rv, ok := rendMap[k]
		if ok && reflect.DeepEqual(v, rv) {
			continue
		}
		if _, isList := v.([]any); isList {
			report.ArrayDiff = append(report.ArrayDiff, k)
			continue
		}
		if _, isList := rv.([]any); isList {
			report.ArrayDiff = append(report.ArrayDiff, k)
			continue
		}
		report.Fields = append(report.Fields, k)
	}
	for k, rv := range rendMap {
		if _, exists := curMap[k]; exists {
			continue
		}
		if _, isList := rv.([]any); isList {
			report.ArrayDiff = append(report.ArrayDiff, k)
		} else {
			report.Fields = append(report.Fields, k)
		}
	}
	sort.Strings(report.Fields)
	sort.Strings(report.ArrayDiff)

	if len(report.Fields) == 0 && len(report.ArrayDiff) == 0 {
		report.NoChanges = true
		return report, nil
	}
	// Array-only differences are informational; nothing to write.
	if len(report.Fields) == 0 {
		return report, nil
	}

	report.LargeDiff = len(report.Fields) >= 5
	if report.LargeDiff && !force {
		return report, ErrAdoptNeedsConfirmation
	}

	if err := p.writeOverrideFields(report.Fields, curMap, baseMap); err != nil {
		return report, err
	}
	return report, nil
}

// subscriptionBaseMap reads the subscription data used as the merge base.
func (p *configPipeline) subscriptionBaseMap() (map[string]any, error) {
	data, err := p.fs.ReadFile(subscriptionDataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	var base any
	if err := yaml.Unmarshal(data, &base); err != nil {
		return nil, fmt.Errorf("parsing subscription data: %w", err)
	}
	if mapping, ok := base.(map[string]any); ok {
		return mapping, nil
	}
	return map[string]any{}, nil
}

// writeOverrideFields merges the given fields (values from curMap) into the
// override file, preserving any content already present. Deleted keys become
// pointed !delete directives when the subscription still provides them; keys
// that only existed in the override file are removed outright.
func (p *configPipeline) writeOverrideFields(fields []string, curMap, baseMap map[string]any) error {
	values := make(map[string]*yaml.Node, len(fields))
	removals := make(map[string]bool)
	for _, k := range fields {
		current, exists := curMap[k]
		if exists {
			node, err := adoptValueNode(current, baseMap[k], k)
			if err != nil {
				return err
			}
			values[k] = node
			continue
		}
		if _, inBase := baseMap[k]; inBase {
			values[k] = deleteDirectiveNode()
			continue
		}
		removals[k] = true
	}

	override := yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	if data, err := p.fs.ReadFile(OverrideFilePath); err == nil {
		if len(data) > 0 {
			var parsed yaml.Node
			if err := yaml.Unmarshal(data, &parsed); err != nil {
				return fmt.Errorf("parsing override file: %w", err)
			}
			if len(parsed.Content) > 0 {
				override = parsed
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if len(override.Content) != 1 || override.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("override file must be a YAML mapping")
	}
	mapping := override.Content[0]
	for _, k := range fields {
		if removals[k] {
			index := findMappingEntry(mapping, k)
			if index >= 0 {
				mapping.Content = append(mapping.Content[:index], mapping.Content[index+2:]...)
			}
			continue
		}
		value := values[k]
		index := findMappingEntry(mapping, k)
		if index >= 0 {
			previous := mapping.Content[index+1]
			value.HeadComment, value.LineComment, value.FootComment = previous.HeadComment, previous.LineComment, previous.FootComment
			value.Anchor = previous.Anchor
			mapping.Content[index+1] = value
			continue
		}
		mapping.Content = append(mapping.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}, value)
	}

	out, err := yaml.Marshal(&override)
	if err != nil {
		return fmt.Errorf("marshaling override file: %w", err)
	}
	if err := p.fs.MkdirAll(configDir, dirPermPrivate); err != nil {
		return err
	}
	tmpPath := OverrideFilePath + ".tmp"
	if err := p.fs.WriteFile(tmpPath, out, filePermPrivateRW); err != nil {
		return errors.Join(fmt.Errorf("staging override file: %w", err), p.fs.RemoveAll(tmpPath))
	}
	if err := p.fs.Rename(tmpPath, OverrideFilePath); err != nil {
		return errors.Join(fmt.Errorf("committing override file: %w", err), p.fs.RemoveAll(tmpPath))
	}
	return nil
}

// adoptValueNode renders current as an overlay value that reproduces it on top
// of base. Nested keys missing from current but present in base become pointed
// !delete directives, so the adopted block keeps following subscription
// updates for fields it does not mention.
func adoptValueNode(current, base any, path string) (*yaml.Node, error) {
	curMap, curIsMap := current.(map[string]any)
	if !curIsMap {
		var node yaml.Node
		if err := node.Encode(current); err != nil {
			return nil, fmt.Errorf("encoding adopted field %s: %w", path, err)
		}
		return &node, nil
	}

	baseMap, _ := base.(map[string]any)
	node := newMappingNode()
	keys := make([]string, 0, len(curMap))
	for key := range curMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		child, err := adoptValueNode(curMap[key], baseMap[key], path+"."+key)
		if err != nil {
			return nil, err
		}
		node.Content = append(node.Content, stringNode(key), child)
	}

	missing := make([]string, 0, len(baseMap))
	for key := range baseMap {
		if _, exists := curMap[key]; !exists {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	for _, key := range missing {
		node.Content = append(node.Content, stringNode(key), deleteDirectiveNode())
	}
	return node, nil
}

func deleteDirectiveNode() *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: deleteDirective, Value: "null"}
}

func stringNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}
