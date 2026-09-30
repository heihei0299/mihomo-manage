package manager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

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
// top-level scalar/map differences into the override file, preserving its
// existing content. Array differences are reported but never adopted. A large
// diff (>= 5 fields) requires force. Idempotent: a second run reports no
// changes once the differences have been adopted.
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

	var curMap, rendMap map[string]any
	if err := yaml.Unmarshal(cur, &curMap); err != nil {
		return report, fmt.Errorf("parsing config.yaml: %w", err)
	}
	if err := yaml.Unmarshal([]byte(rendered), &rendMap); err != nil {
		return report, fmt.Errorf("parsing rendered config: %w", err)
	}

	replaceFields := map[string]bool{}
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
		if hasMapDeletion(v, rv) {
			replaceFields[k] = true
		}
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

	if err := p.writeOverrideFields(report.Fields, curMap, replaceFields); err != nil {
		return report, err
	}
	return report, nil
}

// A partial map cannot express deletions through deep merge. Adopt the whole
// top-level map with !replace if any nested mapping key was removed.
func hasMapDeletion(current, rendered any) bool {
	curMap, curOK := current.(map[string]any)
	rendMap, rendOK := rendered.(map[string]any)
	if !curOK || !rendOK {
		return false
	}
	for k, rv := range rendMap {
		cv, exists := curMap[k]
		if !exists || hasMapDeletion(cv, rv) {
			return true
		}
	}
	return false
}

// writeOverrideFields merges the given fields (values from curMap) into the
// override file, preserving any content already present.
func (p *configPipeline) writeOverrideFields(fields []string, curMap map[string]any, replaceFields map[string]bool) error {
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
		var value yaml.Node
		if err := value.Encode(curMap[k]); err != nil {
			return fmt.Errorf("encoding adopted field %s: %w", k, err)
		}
		if _, exists := curMap[k]; !exists {
			value.Tag = "!delete"
		} else if replaceFields[k] {
			value.Tag = "!replace"
		}
		found := false
		for i := 0; i+1 < len(mapping.Content); i += 2 {
			if mapping.Content[i].Value != k {
				continue
			}
			previous := mapping.Content[i+1]
			if strings.HasPrefix(value.Tag, "!!") && strings.HasPrefix(previous.Tag, "!") && !strings.HasPrefix(previous.Tag, "!!") && previous.Tag != "!delete" {
				value.Tag = previous.Tag
			}
			value.HeadComment, value.LineComment, value.FootComment = previous.HeadComment, previous.LineComment, previous.FootComment
			value.Anchor = previous.Anchor
			*previous = value
			found = true
			break
		}
		if !found {
			mapping.Content = append(mapping.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}, &value)
		}
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
